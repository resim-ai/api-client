package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Client api.ClientWithResponsesInterface
var BffClient graphql.Client

// ConfigPath is the directory holding the CLI's config file and credential cache.
const ConfigPath = "$HOME/.signalflag"

// LegacyConfigPath is the directory the CLI used before it was renamed. It is
// still used, with a deprecation warning, when ConfigPath does not exist.
const LegacyConfigPath = "$HOME/.resim"

// ConfigFileName is the base name (without extension) of the YAML config file
// inside ConfigPath. LegacyConfigFileName is the name used inside LegacyConfigPath.
const ConfigFileName = "signalflag"
const LegacyConfigFileName = "resim"

var (
	rootCmd = &cobra.Command{
		Use:              "signalflag",
		Short:            "Signalflag - Command Line Interface",
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
}

func Execute() error {
	ApplyStyle(rootCmd)
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
	viper.SetConfigName(ConfigFileNameFor(configDir))
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
	ctx := context.Background()

	configDir, _ := GetConfigDir()
	cfg := auth.ConfigFromViper(viper.GetViper(), configDir)
	result, err := auth.Authenticate(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	Client, err = auth.NewAPIClient(ctx, *result.Cache, result.APIURL)
	if err != nil {
		log.Fatal(err)
	}

	BffClient, err = auth.NewBFFClient(ctx, *result.Cache, result.APIURL)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := result.Cache.Save(cfg.CacheDir); err != nil {
			log.Println("error saving credential cache:", err)
		}
	}()
}

var legacyConfigDirWarned bool

// GetConfigDir returns the directory holding the config file and credential
// cache, creating it if needed. If the current location does not exist but the
// legacy one does, the legacy directory is used and a deprecation warning is
// printed once so that existing installs keep working.
func GetConfigDir() (string, error) {
	expectedDir := os.ExpandEnv(ConfigPath)
	if _, err := os.Stat(expectedDir); err == nil {
		return expectedDir, nil
	}
	legacyDir := os.ExpandEnv(LegacyConfigPath)
	if _, err := os.Stat(legacyDir); err == nil {
		if !legacyConfigDirWarned {
			legacyConfigDirWarned = true
			fmt.Fprintf(os.Stderr, "WARNING: the config directory %s is deprecated; move it to %s (and rename %s.yaml to %s.yaml)\n",
				legacyDir, expectedDir, LegacyConfigFileName, ConfigFileName)
		}
		return legacyDir, nil
	}
	if err := os.Mkdir(expectedDir, 0700); err != nil {
		log.Println("error creating directory:", err)
		return "", err
	}
	return expectedDir, nil
}

// ConfigFileNameFor returns the config file base name to use inside configDir:
// the legacy name when configDir is the legacy directory, the current name otherwise.
func ConfigFileNameFor(configDir string) string {
	if configDir == os.ExpandEnv(LegacyConfigPath) {
		return LegacyConfigFileName
	}
	return ConfigFileName
}

// ConfigFilePath returns the full path of the YAML config file.
func ConfigFilePath() string {
	configDir, _ := GetConfigDir()
	return filepath.Join(configDir, ConfigFileNameFor(configDir)+".yaml")
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
