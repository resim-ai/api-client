package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	compose_types "github.com/compose-spec/compose-go/v2/types"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/bff"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type MetricsGlobal struct {
	SkipIfNoData *bool `yaml:"skip-if-no-data"`
}

type MetricsConfig struct {
	Version     int                    `yaml:"version"`
	Global      *MetricsGlobal         `yaml:"global,omitempty"`
	Topics      map[string]interface{} `yaml:"topics"`
	Metrics     map[string]interface{} `yaml:"metrics"`
	MetricsSets map[string]interface{} `yaml:"metrics sets"`
}

func applyGlobalSettings(config *MetricsConfig) {
	if config.Global == nil {
		return
	}
	if config.Global.SkipIfNoData != nil {
		for name, v := range config.Metrics {
			metricMap, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			// Only set if the metric doesn't already have it explicitly
			if _, exists := metricMap["skip-if-no-data"]; !exists {
				metricMap["skip-if-no-data"] = *config.Global.SkipIfNoData
			}
			config.Metrics[name] = metricMap
		}
	}
	config.Global = nil
}

func mergeConfigs(configs []MetricsConfig) (MetricsConfig, error) {
	result := MetricsConfig{
		Topics:      make(map[string]interface{}),
		Metrics:     make(map[string]interface{}),
		MetricsSets: make(map[string]interface{}),
	}

	for i, cfg := range configs {
		if i == 0 {
			result.Version = cfg.Version
		} else if cfg.Version != result.Version {
			return MetricsConfig{}, fmt.Errorf("conflicting versions across config files: %d and %d", result.Version, cfg.Version)
		}

		for k, v := range cfg.Topics {
			if _, exists := result.Topics[k]; exists {
				return MetricsConfig{}, fmt.Errorf("duplicate topic name %q found across config files", k)
			}
			result.Topics[k] = v
		}

		for k, v := range cfg.Metrics {
			if _, exists := result.Metrics[k]; exists {
				return MetricsConfig{}, fmt.Errorf("duplicate metric name %q found across config files", k)
			}
			result.Metrics[k] = v
		}

		for k, v := range cfg.MetricsSets {
			if _, exists := result.MetricsSets[k]; exists {
				return MetricsConfig{}, fmt.Errorf("duplicate metrics set name %q found across config files", k)
			}
			result.MetricsSets[k] = v
		}
	}

	return result, nil
}

// containsGlobChars returns true if the path contains glob metacharacters.
func containsGlobChars(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func resolveConfigPath(configPath string) ([]string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	filePath := path.Join(workDir, configPath)
	if filepath.IsAbs(configPath) {
		filePath = configPath
	}

	// If the path contains glob characters, expand with filepath.Glob
	if containsGlobChars(filePath) {
		matches, err := filepath.Glob(filePath)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", configPath, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("glob pattern %q matched no files", configPath)
		}
		sort.Strings(matches)
		return matches, nil
	}

	// Literal path: check if file exists
	if _, err := os.Stat(filePath); err == nil {
		return []string{filePath}, nil
	}

	// Try alternate extension
	if strings.HasSuffix(filePath, ".yml") {
		altPath := strings.TrimSuffix(filePath, ".yml") + ".yaml"
		if _, err := os.Stat(altPath); err == nil {
			return []string{altPath}, nil
		}
	} else if strings.HasSuffix(filePath, ".yaml") {
		altPath := strings.TrimSuffix(filePath, ".yaml") + ".yml"
		if _, err := os.Stat(altPath); err == nil {
			return []string{altPath}, nil
		}
	}

	return nil, fmt.Errorf("failed to find ReSim metrics config at %s (or .yaml variant). Are you in the right folder?", filePath)
}

func mergeConfigFiles(paths []string, verbose bool) ([]byte, error) {
	// First pass: expand all paths (including globs) into a flat list of resolved files
	var resolvedFiles []string
	for _, p := range paths {
		resolved, err := resolveConfigPath(p)
		if err != nil {
			return nil, err
		}
		for _, r := range resolved {
			if verbose {
				fmt.Printf("Looking for metrics config at %s (found: %s)\n", p, r)
			}
		}
		resolvedFiles = append(resolvedFiles, resolved...)
	}

	// Second pass: read, parse, and collect configs
	configs := make([]MetricsConfig, 0, len(resolvedFiles))
	for _, filePath := range resolvedFiles {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
		}

		var cfg MetricsConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", filePath, err)
		}

		applyGlobalSettings(&cfg)
		configs = append(configs, cfg)
	}

	merged, err := mergeConfigs(configs)
	if err != nil {
		return nil, err
	}

	out, err := yaml.Marshal(&merged)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize merged config: %w", err)
	}

	return out, nil
}

// HasMetricsSetName returns true when the metrics set pointer references a
// non-empty metrics set name.
func HasMetricsSetName(metricsSet *string) bool {
	return metricsSet != nil && *metricsSet != ""
}

// NormalizeMetricsSetName returns nil when a metrics set name is missing or
// explicitly empty so runtime callers can treat both cases as "unset".
func NormalizeMetricsSetName(metricsSet *string) *string {
	if !HasMetricsSetName(metricsSet) {
		return nil
	}
	return metricsSet
}

// ProcessMetricsSet handles the common logic for processing metrics sets
// and automatically adding the special metrics 2.0 pool label when needed.
// Returns the metrics set name if set, nil otherwise.
func ProcessMetricsSet(metricsSetKey string, poolLabels *[]api.PoolLabel) *string {
	if !viper.IsSet(metricsSetKey) {
		return nil
	}

	metricsSet := Ptr(viper.GetString(metricsSetKey))
	if !HasMetricsSetName(metricsSet) {
		return nil
	}

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

// prepareMetricsConfig merges config files and returns a base64-encoded string.
func prepareMetricsConfig(configPaths []string, verbose bool) (string, error) {
	configData, err := mergeConfigFiles(configPaths, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to process metrics config files: %w", err)
	}
	return base64.StdEncoding.EncodeToString(configData), nil
}

// readTemplates reads .liquid template files from the given directory path
// and returns them as base64-encoded MetricsTemplate values.
func readTemplates(templatesPath string, verbose bool) ([]bff.MetricsTemplate, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	templateDir := path.Join(workDir, templatesPath)
	if filepath.IsAbs(templatesPath) {
		templateDir = templatesPath
	}

	templates := []bff.MetricsTemplate{}

	// Check if templates directory exists - it's optional
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		if verbose {
			fmt.Printf("Templates directory not found at %s, skipping templates\n", templatesPath)
		}
		return templates, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat templates dir: %w", err)
	}

	// Directory exists, read templates
	if verbose {
		fmt.Println("Looking for templates in", templatesPath)
	}
	files, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates dir: %w", err)
	}

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
			return nil, fmt.Errorf("failed to read template %s: %w", fullPath, err)
		}
		templates = append(templates, bff.MetricsTemplate{
			Name:     f.Name(),
			Contents: base64.StdEncoding.EncodeToString(contents),
		})
	}

	return templates, nil
}

func SyncMetricsConfig(projectID uuid.UUID, branchID uuid.UUID, configPaths []string, templatesPath string, verbose bool) error {
	branch, err := Client.GetBranchForProjectWithResponse(context.Background(), projectID, branchID)
	if err != nil {
		log.Fatal("unable to retrieve branch associated with the build being run:", err)
	}
	branchName := branch.JSON200.Name
	if branchName == "" {
		log.Fatal("branch has no name associated with it")
	}

	configB64, err := prepareMetricsConfig(configPaths, verbose)
	if err != nil {
		return err
	}

	templates, err := readTemplates(templatesPath, verbose)
	if err != nil {
		return err
	}

	_, err = bff.UpdateMetricsConfig(
		context.Background(),
		BffClient,
		projectID.String(),
		configB64,
		templates,
		branchName, //TODO: We should use branch ids instead of names
	)
	if err != nil {
		return fmt.Errorf("failed to sync metrics config: %w", err)
	}

	if verbose {
		fmt.Print("Successfully synced metrics config")
		if len(templates) > 0 {
			fmt.Println(", and the following templates:")
		} else {
			fmt.Println(".")
		}
		for _, t := range templates {
			fmt.Printf("\t%s\n", t.Name)
		}
	}
	return nil
}
