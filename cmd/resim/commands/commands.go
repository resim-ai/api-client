package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Client api.ClientWithResponsesInterface
var BffClient graphql.Client

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

	ctx := context.Background()
	credentialCache, err := Authenticate(ctx)
	if err != nil {
		log.Fatal(err)
	}
	Client, err = GetClient(ctx, *credentialCache)
	if err != nil {
		log.Fatal(err)
	}

	BffClient = GetBffClient(ctx, *credentialCache)

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
	// Ensure all commands use a common alias normalization for flags
	cmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)
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
	ctx := context.Background()
	credentialCache, err := Authenticate(ctx)
	if err != nil {
		log.Fatal(err)
	}
	Client, err = GetClient(ctx, *credentialCache)
	if err != nil {
		log.Fatal(err)
	}

	BffClient = GetBffClient(ctx, *credentialCache)

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
	// Basic normalization
	name = strings.TrimSpace(name)

	// Capture existing flag names without triggering normalization recursion
	existingBase := map[string]bool{}
	existingIDs := map[string]bool{}
	f.VisitAll(func(fl *pflag.Flag) {
		existingBase[fl.Name] = true
		if strings.HasSuffix(fl.Name, "-id") {
			existingIDs[fl.Name] = true
		}
	})

	// Backwards-compat aliases
	switch name {
	case "project-id":
		name = "project"
	case "project-name":
		name = "project"
	case "branch-name":
		name = "branch"
	case "job-id":
		name = "test-id"
	case "locations":
		name = "location"
	}

	// Support --<component> and --id as aliases for --<component>-id for known components
	knownComponents := []string{"batch", "report", "sweep", "build", "test", "branch", "system"}

	// Map component name and id variants consistently
	for _, component := range knownComponents {
		idFlag := component + "-id"
		// --<component> → --<component>-id if id flag exists
		if name == component {
			if existingIDs[idFlag] {
				name = idFlag
				return pflag.NormalizedName(name)
			}
		}
		// --<component>-id → --<component> if id flag not present but base exists (e.g., systems)
		if name == idFlag {
			if !existingIDs[idFlag] && existingBase[component] {
				name = component
				return pflag.NormalizedName(name)
			}
		}
	}

	// Map --id -> the unique --<component>-id present in this flag set
	if name == "id" {
		idCandidates := []string{}
		for _, component := range knownComponents {
			candidate := component + "-id"
			if existingIDs[candidate] {
				idCandidates = append(idCandidates, candidate)
			}
		}
		if len(idCandidates) == 1 {
			name = idCandidates[0]
			return pflag.NormalizedName(name)
		}
		// If there are no *-id flags, allow mapping to a single base component flag (e.g., systems)
		if len(idCandidates) == 0 {
			baseCandidates := []string{}
			for _, component := range knownComponents {
				if existingBase[component] {
					baseCandidates = append(baseCandidates, component)
				}
			}
			if len(baseCandidates) == 1 {
				name = baseCandidates[0]
				return pflag.NormalizedName(name)
			}
		}
	}

	return pflag.NormalizedName(name)
}
