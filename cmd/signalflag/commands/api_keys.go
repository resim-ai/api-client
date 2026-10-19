package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/auth"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	apiKeysCmd = &cobra.Command{
		Use:     "api-keys",
		Short:   "api-keys contains commands for managing your organization's API keys",
		Long:    `api-keys contains commands for managing your organization's API keys. Managing keys needs an org admin signed in with their own account (browser login, or --username and --password); it is refused when the CLI is using an API key or client credentials.`,
		Aliases: []string{"api-key"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			RegisterViperFlags(cmd, args)
			if err := requirePersonSignIn(viper.GetViper()); err != nil {
				return err
			}
			SetClient(cmd, args)
			return nil
		},
	}
	createApiKeyCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates an API key and prints it once",
		Long:  ``,
		Run:   createApiKey,
	}
	listApiKeysCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists your organization's API keys",
		Long:  ``,
		Run:   listApiKeys,
	}
	revokeApiKeyCmd = &cobra.Command{
		Use:   "revoke",
		Short: "revoke - Revokes an API key by name or ID",
		Long:  ``,
		Run:   revokeApiKey,
	}
)

const (
	apiKeyNameKey          = "name"
	apiKeyIDKey            = "id"
	apiKeyAccessKey        = "access"
	apiKeyExpiresInDaysKey = "expires-in-days"

	apiKeysNeedPersonMessage = "Managing API keys needs a person signed in as an org admin."
	apiKeyListHeader         = "ID\tNAME\tPREFIX\tACCESS\tCREATED BY\tEXPIRES\tSTATUS\n"
)

var apiKeyAccessValues = map[string]bff.ApiKeyAccess{
	"read-only":  bff.ApiKeyAccessReadOnly,
	"read-write": bff.ApiKeyAccessReadWrite,
}

var apiKeyExpiryValues = map[int]bool{0: true, 30: true, 90: true, 365: true}

func init() {
	createApiKeyCmd.Flags().String(apiKeyNameKey, "", "The name of the API key. Must be unique among your organization's active keys.")
	createApiKeyCmd.MarkFlagRequired(apiKeyNameKey)
	createApiKeyCmd.Flags().String(apiKeyAccessKey, "read-only", "The key's access: read-only or read-write.")
	createApiKeyCmd.Flags().Int(apiKeyExpiresInDaysKey, 90, "Days until the key expires: 30, 90 or 365, or 0 for no expiry.")
	apiKeysCmd.AddCommand(createApiKeyCmd)

	apiKeysCmd.AddCommand(listApiKeysCmd)

	revokeApiKeyCmd.Flags().String(apiKeyNameKey, "", "The name of the active API key to revoke.")
	revokeApiKeyCmd.Flags().String(apiKeyIDKey, "", "The ID of the API key to revoke.")
	revokeApiKeyCmd.MarkFlagsOneRequired(apiKeyNameKey, apiKeyIDKey)
	revokeApiKeyCmd.MarkFlagsMutuallyExclusive(apiKeyNameKey, apiKeyIDKey)
	apiKeysCmd.AddCommand(revokeApiKeyCmd)

	rotate := &cobra.Command{Use: "rotate", Short: "Create a replacement without changing the old key", RunE: func(cmd *cobra.Command, _ []string) error {
		return actualRotateApiKey(cmd.Context(), BffClient, cmd.OutOrStdout(), viper.GetString(apiKeyNameKey), viper.GetString(apiKeyIDKey), viper.GetString("new-name"), viper.GetString(apiKeyAccessKey), viper.GetInt(apiKeyExpiresInDaysKey))
	}}
	rotate.Flags().String(apiKeyNameKey, "", "The active key to replace")
	rotate.Flags().String(apiKeyIDKey, "", "The ID of the active key to replace")
	rotate.MarkFlagsOneRequired(apiKeyNameKey, apiKeyIDKey)
	rotate.MarkFlagsMutuallyExclusive(apiKeyNameKey, apiKeyIDKey)
	rotate.Flags().String("new-name", "", "A different, unique replacement name")
	rotate.MarkFlagRequired("new-name")
	rotate.Flags().String(apiKeyAccessKey, "", "Access: read-only or read-write (defaults to the old key)")
	rotate.Flags().Int(apiKeyExpiresInDaysKey, 90, "Expiry in days: 30, 90, 365, or 0 for no expiry")
	apiKeysCmd.AddCommand(rotate)
	rootCmd.AddCommand(apiKeysCmd)
}

func requirePersonSignIn(v *viper.Viper) error {
	mode, err := auth.DetermineAuthMode(auth.ConfigFromViper(v, ""))
	if err != nil {
		return err
	}
	if mode != auth.ModeDeviceCode && mode != auth.ModePassword {
		return errors.New(apiKeysNeedPersonMessage)
	}
	return nil
}

func createApiKey(cmd *cobra.Command, args []string) {
	err := actualCreateApiKey(context.Background(), BffClient, os.Stdout,
		viper.GetString(apiKeyNameKey), viper.GetString(apiKeyAccessKey), viper.GetInt(apiKeyExpiresInDaysKey))
	if err != nil {
		log.Fatal(err)
	}
}

const apiKeyCreateRecovery = "Do not automatically retry creation. Run api-keys list; if the new key exists but its secret was not received, explicitly revoke that key before creating a replacement."

func actualCreateApiKey(ctx context.Context, client graphql.Client, w io.Writer, name, access string, expiresInDays int) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("--name must not be empty")
	}
	accessValue, ok := apiKeyAccessValues[access]
	if !ok {
		return fmt.Errorf("--access must be read-only or read-write, got %q", access)
	}
	if !apiKeyExpiryValues[expiresInDays] {
		return fmt.Errorf("--expires-in-days must be 30, 90, 365, or 0 for no expiry, got %d", expiresInDays)
	}

	resp, err := bff.CreateApiKey(ctx, client, name, accessValue, expiresInDays)
	if err != nil {
		return fmt.Errorf("%w. %s", bffError(err), apiKeyCreateRecovery)
	}
	created := resp.CreateApiKey
	if created.Key == "" {
		return fmt.Errorf("the created key was not received. %s", apiKeyCreateRecovery)
	}
	_, err = fmt.Fprintf(w, "Created API key %q (%s, %s).\n\n%s\n\nCopy this key now. It won't be shown again.\n", created.ApiKey.Name, formatApiKeyAccess(created.ApiKey.Access), describeApiKeyExpiry(created.ApiKey.ExpiresAt), created.Key)
	if err != nil {
		return fmt.Errorf("could not write the created key. %s", apiKeyCreateRecovery)
	}
	return nil
}

func listApiKeys(cmd *cobra.Command, args []string) {
	if err := actualListApiKeys(context.Background(), BffClient, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func actualListApiKeys(ctx context.Context, client graphql.Client, w io.Writer) error {
	resp, err := bff.ListApiKeys(ctx, client)
	if err != nil {
		return bffError(err)
	}
	if len(resp.ApiKeys) == 0 {
		fmt.Fprintln(w, "no API keys")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)
	fmt.Fprint(tw, apiKeyListHeader)
	for _, k := range resp.ApiKeys {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			k.Id, k.Name, k.Prefix, formatApiKeyAccess(k.Access), k.CreatedByEmail, formatApiKeyDate(k.ExpiresAt), formatApiKeyStatus(k.ApiKeyFields))
	}
	return tw.Flush()
}

func revokeApiKey(cmd *cobra.Command, args []string) {
	err := actualRevokeApiKey(context.Background(), BffClient, os.Stdout, viper.GetString(apiKeyNameKey), viper.GetString(apiKeyIDKey))
	if err != nil {
		log.Fatal(err)
	}
}

func actualRevokeApiKey(ctx context.Context, client graphql.Client, w io.Writer, name, id string) error {
	if id == "" {
		found, err := findActiveApiKey(ctx, client, name)
		if err != nil {
			return err
		}
		id = found
	}

	resp, err := bff.RevokeApiKey(ctx, client, id)
	if err != nil {
		return bffError(err)
	}

	key := resp.RevokeApiKey.ApiKeyFields
	if key.RevocationPending {
		_, err = fmt.Fprintf(w, "Revocation is pending synchronization for API key %q (%s). Retry api-keys revoke --id %s to finish.\n", key.Name, key.Prefix, key.Id)
		return err
	}
	_, err = fmt.Fprintf(w, "Revoked API key %q (%s).\n", key.Name, key.Prefix)
	return err
}

func findActiveApiKey(ctx context.Context, client graphql.Client, name string) (string, error) {
	resp, err := bff.ListApiKeys(ctx, client)
	if err != nil {
		return "", bffError(err)
	}
	var id string
	for _, k := range resp.ApiKeys {
		if k.Name == name && (k.Status == bff.ApiKeyStatusActive || k.RevocationPending) {
			if id != "" {
				return "", errors.New("key name is ambiguous; select it with --id")
			}
			id = k.Id
		}
	}
	if id != "" {
		return id, nil
	}
	return "", fmt.Errorf("no active API key named %q", name)
}

func actualRotateApiKey(ctx context.Context, client graphql.Client, w io.Writer, name, id, newName, access string, expiresInDays int) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("--new-name must not be empty")
	}
	resp, err := bff.ListApiKeys(ctx, client)
	if err != nil {
		return bffError(err)
	}
	for _, key := range resp.ApiKeys {
		if (id != "" && key.Id == id) || (id == "" && key.Name == name && key.Status == bff.ApiKeyStatusActive) {
			if key.Status != bff.ApiKeyStatusActive || key.RevocationPending {
				return errors.New("only an active key can be replaced")
			}
			if newName == key.Name {
				return errors.New("--new-name must differ from the old key name")
			}
			if access == "" {
				access = formatApiKeyAccess(key.Access)
			}
			if err := actualCreateApiKey(ctx, client, w, newName, access, expiresInDays); err != nil {
				return fmt.Errorf("%w; the old key was not changed", err)
			}
			_, err := fmt.Fprintf(w, "The old key was not changed. Install and test the replacement, then use the same endpoint settings to run: signalflag api-keys revoke --id %s\n", key.Id)
			return err
		}
	}
	return errors.New("no matching active key; use api-keys list to inspect metadata")
}

var apiKeySecretPattern = regexp.MustCompile(`(?:sfk_|sf_live_)[A-Za-z0-9_-]+`)

func bffError(err error) error {
	var list gqlerror.List
	var httpErr *graphql.HTTPError
	if errors.As(err, &httpErr) {
		return fmt.Errorf("API-key request failed (HTTP %d)", httpErr.StatusCode)
	}
	if !errors.As(err, &list) {
		return errors.New("API-key request failed; check the endpoint and connection")
	}
	messages := make([]string, 0, len(list))
	for _, e := range list {
		messages = append(messages, apiKeySecretPattern.ReplaceAllString(e.Message, "[redacted]"))
	}
	return errors.New(strings.Join(messages, "\n"))
}

func formatApiKeyStatus(key bff.ApiKeyFields) string {
	if key.RevocationPending {
		return "revoking"
	}
	return strings.ToLower(string(key.Status))
}

func formatApiKeyAccess(access bff.ApiKeyAccess) string {
	for flagValue, a := range apiKeyAccessValues {
		if a == access {
			return flagValue
		}
	}
	return strings.ToLower(string(access))
}

func formatApiKeyDate(value string) string {
	if value == "" {
		return "never"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.Format("2006-01-02")
}

func describeApiKeyExpiry(expiresAt string) string {
	if expiresAt == "" {
		return "never expires"
	}
	return "expires " + formatApiKeyDate(expiresAt)
}
