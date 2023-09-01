package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/jmespath/go-jmespath"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Client api.ClientWithResponsesInterface

const ConfigPath = "$HOME/.resim"

var (
	rootCmd = &cobra.Command{
		Use:           "resim",
		Short:         "resim - Command Line Interface for ReSim",
		Long:          ``,
		SilenceErrors: true,
		SilenceUsage:  true,
		Run:           rootCommand,
	}
)

func rootCommand(cmd *cobra.Command, args []string) {
	viper.SetConfigName("resim")
	viper.AddConfigPath(ConfigPath)
	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError, *fs.PathError:
		default:
			log.Fatal(fmt.Errorf("error reading config file: %v %T", err, err))
		}
	}

	var err error
	var credentialCache *CredentialCache
	Client, credentialCache, err = GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	defer credentialCache.SaveCredentialCache()
}

func Execute() error {
	ApplyReSimStyle(rootCmd)
	return rootCmd.Execute()
}

func OutputJson(data interface{}, query string) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if query != "" {
		query_result, err := jmespath.Search(query, data)
		if err != nil {
			log.Fatal("invalid jmespath query:", err)
		}
		enc.Encode(query_result)
	} else {
		enc.Encode(data)
	}
}

func RegisterViperFlagsAndSetClient(cmd *cobra.Command, args []string) {
	viper.BindPFlags(cmd.Flags())
	viper.SetConfigName("resim")
	viper.AddConfigPath(ConfigPath)
	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError, *fs.PathError:
		default:
			log.Fatal(fmt.Errorf("error reading config file: %v %T", err, err))
		}
	}

	var err error
	var credentialCache *CredentialCache
	Client, credentialCache, err = GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	defer credentialCache.SaveCredentialCache()
}

func AliasNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "project-id":
		name = "project"
		break
	case "project-name":
		name = "project"
		break
	}
	return pflag.NormalizedName(name)
}
