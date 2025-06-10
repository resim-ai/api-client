package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
)

// ParseParameterString parses a string in the format "key=value" or "key:value"
// into a key-value pair. It first tries to split on "=" and falls back to ":" if that fails.
// This is especially useful for cases where parameter names contain colons, which
// is often the case in ros-based systems (e.g., "namespace::param").
func ParseParameterString(parameterString string) (string, string, error) {
	// First try to split on equals sign (preferred delimiter)
	equalParts := strings.SplitN(parameterString, "=", 2)
	if len(equalParts) == 2 {
		return equalParts[0], equalParts[1], nil
	}

	// Fall back to using colon as delimiter
	colonParts := strings.SplitN(parameterString, ":", 2)
	if len(colonParts) == 2 {
		return colonParts[0], colonParts[1], nil
	}

	return "", "", fmt.Errorf("failed to parse parameter: %s - must be in the format <parameter-name>=<parameter-value> or <parameter-name>:<parameter-value>", parameterString)
}

func ParseBuildSpec(buildSpecLocation string, withOsEnv bool, withEnvFiles []string) ([]byte, error) {
	// We assume that the build spec is a valid YAML file
	ctx := context.Background()

	options, err := cli.NewProjectOptions(
		[]string{buildSpecLocation},
		cli.WithLoadOptions(func(options *loader.Options) {
			options.SkipDefaultValues = true
		}),
		cli.WithNormalization(false),
	)
	if err != nil {
		log.Fatal(err)
	}
	if withOsEnv {
		err = cli.WithOsEnv(options)
		if err != nil {
			log.Fatal(err)
		}
	}
	if len(withEnvFiles) > 0 {
		fmt.Println("withEnvFiles", withEnvFiles)
		err = cli.WithEnvFiles(withEnvFiles...)(options)
		if err != nil {
			log.Fatal(err)
		}
		err = cli.WithDotEnv(options)
		if err != nil {
			log.Fatal(err)
		}
	}
	project, err := options.LoadProject(ctx)
	if err != nil {
		log.Fatal(err)
	}

	projectJSON, err := project.MarshalJSON()
	if err != nil {
		log.Fatal(err)
	}

	return projectJSON, nil
}

func getAndValidatePoolLabels(poolLabelsKey string) []api.PoolLabel {
	poolLabels := []api.PoolLabel{}
	if viper.IsSet(poolLabelsKey) {
		poolLabels = viper.GetStringSlice(poolLabelsKey)
	}
	for i := range poolLabels {
		poolLabels[i] = strings.TrimSpace(poolLabels[i])
		if poolLabels[i] == "resim" {
			log.Fatal("failed to run command: resim is a reserved pool label")
		}
	}
	return poolLabels
}
