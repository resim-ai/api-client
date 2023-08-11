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

func Execute() error {
	viper.SetDefault("url", "https://api.resim.ai/v1")
	return rootCmd.Execute()
}
