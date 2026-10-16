package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/auth"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func init() {
	rootCmd.AddCommand(newAPIKeysCommand())
}

func newAPIKeysCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "api-keys",
		Short: "Manage organization API keys using a human sign-in",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			RegisterViperFlags(cmd, args)
			mode, err := auth.ConfigFromViper(viper.GetViper(), "").Mode()
			if err != nil {
				return err
			}
			if mode != auth.ModeDeviceCode && mode != auth.ModePassword {
				return fmt.Errorf("API-key management requires a human sign-in; remove API-key and client-credential settings")
			}
			return SetClient(cmd, args)
		},
	}
	list := &cobra.Command{
		Use: "list", Short: "List key metadata and status", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			response, err := bff.ListApiKeys(cmd.Context(), BffClient)
			if err != nil {
				return safeAPIKeyError(err)
			}
			return writeAPIKeyJSON(cmd, response.ApiKeys)
		},
	}
	var createName, createAccess string
	var createExpiry int
	create := &cobra.Command{
		Use: "create", Short: "Create a key and print its secret once", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			access, expiry, err := apiKeyCreateOptions(createAccess, createExpiry)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(createName)
			if name == "" {
				return fmt.Errorf("--name must not be empty")
			}
			return createAndPrintAPIKey(cmd, name, access, expiry)
		},
	}
	create.Flags().StringVar(&createName, "name", "", "Unique key name")
	create.Flags().StringVar(&createAccess, "access", "READ_ONLY", "Access: READ_ONLY or READ_WRITE")
	create.Flags().IntVar(&createExpiry, "expires-in-days", 90, "Expiry: 30, 90, 365, or 0 for no expiry")
	create.MarkFlagRequired("name")

	var revokeID, revokeName string
	revoke := &cobra.Command{
		Use: "revoke", Short: "Revoke a key or retry a pending revocation", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, name, err := apiKeySelector(revokeID, revokeName)
			if err != nil {
				return err
			}
			if id == "" {
				key, err := findAPIKey(cmd, id, name)
				if err != nil {
					return err
				}
				id = key.Id
			}
			response, err := bff.RevokeApiKey(cmd.Context(), BffClient, id)
			if err != nil {
				return safeAPIKeyError(err)
			}
			if err := writeAPIKeyJSON(cmd, &response.RevokeApiKey); err != nil {
				return err
			}
			if response.RevokeApiKey.Status == bff.ApiKeyStatusRevoking {
				fmt.Fprintf(cmd.ErrOrStderr(), "Revocation is pending synchronization. Retry api-keys revoke --id %s to finish.\n", id)
			}
			return nil
		},
	}
	revoke.Flags().StringVar(&revokeID, "id", "", "Key ID (also accepts revoked keys for safe retries)")
	revoke.Flags().StringVar(&revokeName, "name", "", "Unique non-revoked key name")

	var rotateID, rotateName, newName, rotateAccess string
	var rotateExpiry int
	rotate := &cobra.Command{
		Use: "rotate", Short: "Create a replacement without changing the old key", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, name, err := apiKeySelector(rotateID, rotateName)
			if err != nil {
				return err
			}
			replacementName := strings.TrimSpace(newName)
			if replacementName == "" {
				return fmt.Errorf("--new-name must not be empty")
			}
			old, err := findAPIKey(cmd, id, name)
			if err != nil {
				return err
			}
			if replacementName == old.Name {
				return fmt.Errorf("--new-name must differ from the old key name")
			}
			accessName := rotateAccess
			if accessName == "" {
				accessName = string(old.Access)
			}
			access, expiry, err := apiKeyCreateOptions(accessName, rotateExpiry)
			if err != nil {
				return err
			}
			err = createAndPrintAPIKey(cmd, replacementName, access, expiry)
			if err != nil {
				return fmt.Errorf("%w; the old key was not changed", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "The old key was not changed. Install and test the replacement, then use the same endpoint settings to run: signalflag api-keys revoke --id %s\n", old.Id)
			return nil
		},
	}
	rotate.Flags().StringVar(&rotateID, "id", "", "Existing key ID")
	rotate.Flags().StringVar(&rotateName, "name", "", "Existing unique non-revoked key name")
	rotate.Flags().StringVar(&newName, "new-name", "", "Unique replacement key name")
	rotate.Flags().StringVar(&rotateAccess, "access", "", "Access: READ_ONLY or READ_WRITE (defaults to the old key)")
	rotate.Flags().IntVar(&rotateExpiry, "expires-in-days", 90, "Expiry: 30, 90, 365, or 0 for no expiry")
	rotate.MarkFlagRequired("new-name")
	command.AddCommand(list, create, revoke, rotate)
	return command
}

func apiKeyCreateOptions(access string, days int) (bff.ApiKeyAccess, *int, error) {
	preset := bff.ApiKeyAccess(strings.ToUpper(access))
	if preset != bff.ApiKeyAccessReadOnly && preset != bff.ApiKeyAccessReadWrite {
		return "", nil, fmt.Errorf("--access must be READ_ONLY or READ_WRITE")
	}
	switch days {
	case 0, 30, 90, 365:
		return preset, &days, nil
	default:
		return "", nil, fmt.Errorf("--expires-in-days must be 30, 90, 365, or 0 for no expiry")
	}
}

func apiKeySelector(id, name string) (string, string, error) {
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if (id == "") == (name == "") {
		return "", "", fmt.Errorf("provide exactly one of --id or --name")
	}
	return id, name, nil
}

func findAPIKey(cmd *cobra.Command, id, name string) (bff.ApiKeyMetadata, error) {
	response, err := bff.ListApiKeys(cmd.Context(), BffClient)
	if err != nil {
		return bff.ApiKeyMetadata{}, safeAPIKeyError(err)
	}
	var matches []bff.ApiKeyMetadata
	for _, key := range response.ApiKeys {
		if (id != "" && key.Id == id) || (name != "" && key.Name == name && key.Status != bff.ApiKeyStatusRevoked) {
			matches = append(matches, key.ApiKeyMetadata)
		}
	}
	if len(matches) == 0 {
		return bff.ApiKeyMetadata{}, fmt.Errorf("no matching key; use api-keys list to inspect metadata")
	}
	if len(matches) > 1 {
		return bff.ApiKeyMetadata{}, fmt.Errorf("key name is ambiguous; select it with --id")
	}
	return matches[0], nil
}

const apiKeyCreateRecovery = "Do not automatically retry creation. Run api-keys list; if the new key exists but its secret was not received, explicitly revoke that key before creating a replacement."

func createAndPrintAPIKey(cmd *cobra.Command, name string, access bff.ApiKeyAccess, expiry *int) error {
	response, err := bff.CreateApiKey(cmd.Context(), BffClient, name, access, expiry)
	if err != nil {
		return fmt.Errorf("%w. %s", safeAPIKeyError(err), apiKeyCreateRecovery)
	}
	if err := writeAPIKeyJSON(cmd, &response.CreateApiKey); err != nil {
		return fmt.Errorf("could not write the created key. %s", apiKeyCreateRecovery)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Save this key securely; its secret cannot be shown again.")
	return nil
}

func writeAPIKeyJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

var apiKeySecretPattern = regexp.MustCompile(`sfk_[A-Za-z0-9_-]+`)

func safeAPIKeyError(err error) error {
	var httpError *graphql.HTTPError
	if errors.As(err, &httpError) {
		return fmt.Errorf("API-key request failed (HTTP %d)", httpError.StatusCode)
	}
	var graphQLErrors gqlerror.List
	if errors.As(err, &graphQLErrors) {
		messages := make([]string, len(graphQLErrors))
		for i, item := range graphQLErrors {
			messages[i] = apiKeySecretPattern.ReplaceAllString(item.Message, "[redacted]")
		}
		return errors.New(strings.Join(messages, "; "))
	}
	return fmt.Errorf("API-key request failed; check the endpoint and connection")
}
