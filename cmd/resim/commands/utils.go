package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
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

func ParseBuildSpec(buildSpecLocations []string) ([]byte, error) {
	// We assume that the build spec is a valid YAML file
	ctx := context.Background()

	options, err := cli.NewProjectOptions(
		buildSpecLocations,
		cli.WithOsEnv,
		cli.WithDotEnv,
		cli.WithName(viper.GetString(buildSpecNameKey)),
	)
	if err != nil {
		log.Fatal(err)
	}

	project, err := options.LoadProject(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Use the MarshalYAML method to get YAML representation
	projectYAML, err := project.MarshalYAML()
	if err != nil {
		log.Fatal(err)
	}

	return projectYAML, nil
}
