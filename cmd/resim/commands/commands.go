package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:           "resim",
		Short:         "resim - Command Line Interface for ReSim",
		Long:          ``,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
)

const (
	URLKey          = "url"
	ClientIDKey     = "client_id"
	ClientSecretKey = "client_secret"
)

func AddRootCmdFlags() {
	rootCmd.PersistentFlags().String(URLKey, "", "The url of the API.")
	rootCmd.PersistentFlags().String(ClientIDKey, "", "Authentication credentials client ID")
	rootCmd.PersistentFlags().String(ClientSecretKey, "", "Authentication credentials client secret")

	viper.BindPFlags(rootCmd.PersistentFlags())
}

func Execute() error {
	AddRootCmdFlags()
	viper.SetDefault("url", "https://api.resim.ai/v1")
	return rootCmd.Execute()
}
