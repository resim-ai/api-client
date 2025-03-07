package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Client api.ClientWithResponsesInterface

const ConfigPath = "$HOME/.resim"

var (
	rootCmd = &cobra.Command{
		Use:              "resim",
		Short:            "resim - Command Line Interface for ReSim",
		Long:             ``,
		SilenceErrors:    true,
		SilenceUsage:     true,
		Run:              rootCommand,
		PersistentPreRun: RegisterViperFlagsAndSetClient,
	}
)

func rootCommand(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		cmd.Help()
		os.Exit(0)
	}

	viper.SetConfigName("resim")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(os.ExpandEnv(ConfigPath))
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

func OutputJson(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func RegisterViperFlagsAndSetClient(cmd *cobra.Command, args []string) {
	RegisterViperFlags(cmd, args)
	SetClient(cmd, args)
}

func RegisterViperFlags(cmd *cobra.Command, args []string) {
	configDir, _ := GetConfigDir()
	viper.BindPFlags(cmd.Flags())
	viper.SetConfigName("resim")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError, *fs.PathError:
		default:
			log.Fatal(fmt.Errorf("error reading config file: %v %T", err, err))
		}
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) {
			// For any flag that we receive via external methods (config file, environment variable)
			// we can consider it as "not required" for further processing
			cmd.Flags().SetAnnotation(f.Name, cobra.BashCompOneRequiredFlag, []string{"false"})
		}
	})
}

func SetClient(cmd *cobra.Command, args []string) {
	var err error
	var credentialCache *CredentialCache
	Client, credentialCache, err = GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	defer credentialCache.SaveCredentialCache()
}

func GetConfigDir() (string, error) {
	expectedDir := os.ExpandEnv(ConfigPath)
	// Check first if the directory exists, and if it does not, create it:
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		err := os.Mkdir(expectedDir, 0700)
		if err != nil {
			log.Println("error creating directory:", err)
			return "", err
		}
	}
	return expectedDir, nil
}

func AliasNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "project-id":
		name = "project"
	case "project-name":
		name = "project"
	case "branch-name":
		name = "branch"
	case "job-id":
		name = "test-id"
	}
	return pflag.NormalizedName(name)
}
