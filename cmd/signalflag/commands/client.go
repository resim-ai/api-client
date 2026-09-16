package commands

import (
	"github.com/resim-ai/api-client/auth"
	"github.com/spf13/viper"
)

const verboseKey = "verbose"

func init() {
	rootCmd.PersistentFlags().String(auth.KeyURL, "", "The URL of the API.")
	viper.SetDefault(auth.KeyURL, auth.ProdAPIURL)
	rootCmd.PersistentFlags().String(auth.KeyAuthURL, "", "The URL of the authentication endpoint.")
	viper.SetDefault(auth.KeyAuthURL, auth.ProdAuthURL)
	rootCmd.PersistentFlags().String(auth.KeyClientID, "", "Authentication credentials client ID")
	rootCmd.PersistentFlags().String(auth.KeyClientSecret, "", "Authentication credentials client secret")
	rootCmd.PersistentFlags().String(auth.KeyDevInteractiveClient, "", "Client ID for dev interactive login")
	viper.SetDefault(auth.KeyDevInteractiveClient, auth.DefaultDevInteractiveClientID)
	rootCmd.PersistentFlags().String(auth.KeyDevNonInteractiveClient, "", "Client ID for dev non-interactive login")
	viper.SetDefault(auth.KeyDevNonInteractiveClient, auth.DefaultDevNonInteractiveClientID)
	rootCmd.PersistentFlags().String(auth.KeyProdInteractiveClient, "", "Client ID for prod interactive login")
	viper.SetDefault(auth.KeyProdInteractiveClient, auth.DefaultProdInteractiveClientID)
	rootCmd.PersistentFlags().String(auth.KeyProdNonInteractiveClient, "", "Client ID for prod non-interactive login")
	viper.SetDefault(auth.KeyProdNonInteractiveClient, auth.DefaultProdNonInteractiveClientID)
	rootCmd.PersistentFlags().String(auth.KeyUsername, "", "username for non-interactive login")
	rootCmd.PersistentFlags().String(auth.KeyPassword, "", "password for non-interactive login")
	rootCmd.PersistentFlags().Bool(verboseKey, false, "Verbose mode")
}
