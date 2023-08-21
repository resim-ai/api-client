package commands

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/resim-ai/api-client/api"
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

var Client api.ClientWithResponsesInterface

const ConfigPath = "$HOME/.resim"

func Execute() error {
	viper.SetConfigName("resim")
	viper.AddConfigPath(ConfigPath)
	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError, *fs.PathError:
		default:
			return errors.New(fmt.Sprintf("error reading config file: %v %T", err, err))
		}
	}

	var err error
	var credentialCache *CredentialCache
	Client, credentialCache, err = GetClient(context.Background())
	if err != nil {
		return err
	}

	defer credentialCache.SaveCredentialCache()

	return rootCmd.Execute()
}

func RegisterViperFlags(cmd *cobra.Command, args []string) {
	viper.BindPFlags(cmd.Flags())
}
