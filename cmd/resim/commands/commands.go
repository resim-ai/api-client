package commands

import (
	"fmt"
	"io/fs"
	"log"

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
	viper.SetConfigName("resim")
	viper.AddConfigPath("$HOME/.resim")
	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError, *fs.PathError:
		default:
			log.Fatal("error reading config file: ", err, fmt.Sprintf("%T", err))
		}
	}
	return rootCmd.Execute()
}

func RegisterViperFlags(cmd *cobra.Command, args []string) {
	viper.BindPFlags(cmd.Flags())
}
