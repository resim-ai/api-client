package commands

import (
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

// Existing installations keep using the legacy directory until migrated.
const LegacyConfigPath = "$HOME/.resim"

const ConfigFileName = "signalflag"
const LegacyConfigFileName = "resim"

var (
	rootCmd = &cobra.Command{
		Use:               "signalflag",
		Short:             "Signalflag - Command Line Interface",
		Long:              ``,
		SilenceErrors:     true,
		SilenceUsage:      true,
		Run:               rootCommand,
		PersistentPreRunE: RegisterViperFlagsAndSetClient,
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

func RegisterViperFlagsAndSetClient(cmd *cobra.Command, args []string) error {
	RegisterViperFlags(cmd, args)
	return SetClient(cmd, args)
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
			cmd.Flags().SetAnnotation(f.Name, cobra.BashCompOneRequiredFlag, []string{"false"})
		}
	})
}

func SetClient(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	configDir, _ := GetConfigDir()
	cfg := auth.ConfigFromViper(viper.GetViper(), configDir)
	cfg.DeviceCodeOutput = cmd.ErrOrStderr()
	result, err := auth.Authenticate(ctx, cfg)
	if err != nil {
		return err
	}

	Client, err = auth.NewAPIClient(ctx, *result.Cache, result.APIURL)
	if err != nil {
		return err
	}

	BffClient, err = auth.NewBFFClient(ctx, *result.Cache, result.APIURL)
	if err != nil {
		return err
	}

	defer func() {
		if err := result.Cache.Save(cfg.CacheDir); err != nil {
			log.Println("error saving credential cache:", err)
		}
	}()
	return nil
}

var legacyConfigDirWarned bool

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
