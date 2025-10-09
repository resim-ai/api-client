package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	compose_types "github.com/compose-spec/compose-go/v2/types"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/bff"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
)

const METRICS_2_POOL_LABEL = "resim:metrics2"

// Add Metrics 2.0 Pool labels to the list of pool labels:
func AddMetrics2PoolLabels(poolLabels *[]api.PoolLabel) {
	*poolLabels = append(*poolLabels, METRICS_2_POOL_LABEL)
}

// ProcessMetricsSet handles the common logic for processing metrics sets
// and automatically adding the special metrics 2.0 pool label when needed.
// Returns the metrics set name if set, nil otherwise.
func ProcessMetricsSet(metricsSetKey string, poolLabels *[]api.PoolLabel) *string {
	if !viper.IsSet(metricsSetKey) {
		return nil
	}

	metricsSet := Ptr(viper.GetString(metricsSetKey))

	// Metrics 2.0 steps will only be run if we use the special pool
	// label, so let's enable it automatically if the user requested a
	// metrics set
	AddMetrics2PoolLabels(poolLabels)

	return metricsSet
}

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

func ParseBuildSpec(buildSpecLocation string, withOsEnv bool, withEnvFiles []string, profiles []string, quiet bool) (*compose_types.Project, error) {
	// We assume that the build spec is a valid YAML file
	ctx := context.Background()

	options, err := cli.NewProjectOptions(
		[]string{buildSpecLocation},
		cli.WithLoadOptions(func(options *loader.Options) {
			options.SkipDefaultValues = true
		}),
		cli.WithNormalization(false),
		cli.WithProfiles(profiles),
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
		if !quiet {
			fmt.Println("withEnvFiles", withEnvFiles)
		}
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

	return project, nil
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

func SyncMetricsConfig(projectID uuid.UUID, branchName string, verbose bool) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	configFilePath := path.Join(workDir, ".resim/metrics/config.yml")
	if verbose {
		fmt.Printf("Looking for metrics config at %s\n", configFilePath)
	}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return fmt.Errorf("failed to find ReSim metrics config at %s", configFilePath)
	}
	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	configB64 := base64.StdEncoding.EncodeToString(configData)

	templateDir := path.Join(workDir, ".resim/metrics/templates")
	if verbose {
		fmt.Printf("Looking for templates in %s\n", templateDir)
	}
	files, err := os.ReadDir(templateDir)
	if err != nil {
		return fmt.Errorf("failed to read templates dir: %w", err)
	}
	templates := []bff.MetricsTemplate{}
	for _, f := range files {
		if f.IsDir() {
			if verbose {
				fmt.Printf("Skipping directory %s\n", f.Name())
			}
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Name()), ".liquid") {
			if verbose {
				fmt.Printf("Skipping non .liquid file %s\n", f.Name())
			}
			continue
		}
		if verbose {
			fmt.Printf("Found template %s\n", f.Name())
		}
		fullPath := path.Join(templateDir, f.Name())
		contents, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", fullPath, err)
		}
		templates = append(templates, bff.MetricsTemplate{
			Name:     f.Name(),
			Contents: base64.StdEncoding.EncodeToString(contents),
		})
	}

	_, err = bff.UpdateMetricsConfig(context.Background(), BffClient, projectID.String(), configB64, templates, branchName)
	if err != nil {
		return fmt.Errorf("failed to sync metrics config: %w", err)
	}

	if verbose {
		fmt.Println("Successfully synced metrics config with templates:")
		for _, t := range templates {
			fmt.Printf("\t%s\n", t.Name)
		}
	}
	return nil
}
