package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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
	apiKeyListHeader         = "NAME\tPREFIX\tACCESS\tCREATED BY\tEXPIRES\tSTATUS\n"
)

var apiKeyAccessValues = map[string]bff.ApiKeyAccess{
	"read-only":  bff.ApiKeyAccessReadOnly,
	"read-write": bff.ApiKeyAccessReadWrite,
}

var apiKeyExpiryValues = map[int]bool{0: true, 30: true, 90: true, 365: true}

func init() {
	createApiKeyCmd.Flags().String(apiKeyNameKey, "", "The name of the API key. Must be unique among your organization's active keys.")
	createApiKeyCmd.MarkFlagRequired(apiKeyNameKey)
	createApiKeyCmd.Flags().String(apiKeyAccessKey, "", "The key's access: read-only or read-write.")
	createApiKeyCmd.MarkFlagRequired(apiKeyAccessKey)
	createApiKeyCmd.Flags().Int(apiKeyExpiresInDaysKey, 90, "Days until the key expires: 30, 90 or 365, or 0 for no expiry.")
	apiKeysCmd.AddCommand(createApiKeyCmd)

	apiKeysCmd.AddCommand(listApiKeysCmd)

	revokeApiKeyCmd.Flags().String(apiKeyNameKey, "", "The name of the active API key to revoke.")
	revokeApiKeyCmd.Flags().String(apiKeyIDKey, "", "The ID of the API key to revoke.")
	revokeApiKeyCmd.MarkFlagsOneRequired(apiKeyNameKey, apiKeyIDKey)
	revokeApiKeyCmd.MarkFlagsMutuallyExclusive(apiKeyNameKey, apiKeyIDKey)
	apiKeysCmd.AddCommand(revokeApiKeyCmd)

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

func actualCreateApiKey(ctx context.Context, client graphql.Client, w io.Writer, name, access string, expiresInDays int) error {
	accessValue, ok := apiKeyAccessValues[access]
	if !ok {
		return fmt.Errorf("--access must be read-only or read-write, got %q", access)
	}
	if !apiKeyExpiryValues[expiresInDays] {
		return fmt.Errorf("--expires-in-days must be 30, 90, 365, or 0 for no expiry, got %d", expiresInDays)
	}

	resp, err := bff.CreateApiKey(ctx, client, name, accessValue, expiresInDays)
	if err != nil {
		return bffError(err)
	}

	created := resp.CreateApiKey
	fmt.Fprintf(w, "Created API key %q (%s, %s).\n\n", created.ApiKey.Name, formatApiKeyAccess(created.ApiKey.Access), describeApiKeyExpiry(created.ApiKey.ExpiresAt))
	fmt.Fprintf(w, "%s\n\n", created.Key)
	fmt.Fprintln(w, "Copy this key now. It won't be shown again.")
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
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			k.Name, k.Prefix, formatApiKeyAccess(k.Access), k.CreatedByEmail, formatApiKeyDate(k.ExpiresAt), formatApiKeyStatus(k.ApiKeyFields))
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
		found, err := findRevocableApiKey(ctx, client, name)
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
	if key.Status == bff.ApiKeyStatusRevoking {
		fmt.Fprintf(w, "API key %q is still being revoked. Run `%s` again to finish.\n", key.Name, revokeAgainCommand(key.Id))
		return nil
	}
	fmt.Fprintf(w, "Revoked API key %q (%s).\n", key.Name, key.Prefix)
	return nil
}

// A key left in REVOKING is still the one a name refers to until revoking it again finishes the change.
func findRevocableApiKey(ctx context.Context, client graphql.Client, name string) (string, error) {
	resp, err := bff.ListApiKeys(ctx, client)
	if err != nil {
		return "", bffError(err)
	}
	var revoking string
	for _, k := range resp.ApiKeys {
		if k.Name != name {
			continue
		}
		switch k.Status {
		case bff.ApiKeyStatusActive:
			return k.Id, nil
		case bff.ApiKeyStatusRevoking:
			revoking = k.Id
		}
	}
	if revoking != "" {
		return revoking, nil
	}
	return "", fmt.Errorf("no active API key named %q", name)
}

// bffError returns the BFF's own error messages without the GraphQL path prefix.
func bffError(err error) error {
	var list gqlerror.List
	var httpErr *graphql.HTTPError
	switch {
	case errors.As(err, &httpErr) && len(httpErr.Response.Errors) > 0:
		list = httpErr.Response.Errors
	case errors.As(err, &list):
	default:
		return err
	}
	messages := make([]string, 0, len(list))
	for _, e := range list {
		messages = append(messages, e.Message)
	}
	return errors.New(strings.Join(messages, "\n"))
}

func formatApiKeyAccess(access bff.ApiKeyAccess) string {
	for flagValue, a := range apiKeyAccessValues {
		if a == access {
			return flagValue
		}
	}
	return strings.ToLower(string(access))
}

func formatApiKeyStatus(k bff.ApiKeyFields) string {
	status := strings.ToLower(string(k.Status))
	if k.Status == bff.ApiKeyStatusRevoking {
		return fmt.Sprintf("%s (run `%s` again to finish)", status, revokeAgainCommand(k.Id))
	}
	return status
}

func revokeAgainCommand(id string) string {
	return fmt.Sprintf("%s api-keys revoke --id %s", rootCmd.Name(), id)
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
