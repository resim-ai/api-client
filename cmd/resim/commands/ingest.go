package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var (
	ingestLogCmd = &cobra.Command{
		Use:   "ingest",
		Short: "ingest - Ingests a new log, creating a build to track the software version, and generates metrics analysis",
		Long:  ``,
		Run:   ingestLog,
	}
)

const (
	ingestProjectKey            = "project"
	ingestSystemKey             = "system"
	ingestBranchKey             = "branch"
	ingestVersionKey            = "version"
	ingestLogNameKey            = "log-name"
	ingestExperienceLocationKey = "log-location"
	ingestExperienceTagsKey     = "tags"
	ingestGithubKey             = "github"
	ingestMetricsBuildKey       = "metrics-build-id"
	ingestBuildKey              = "build-id"
	ingestLogKey                = "log"
	ingestConfigFileKey         = "log-config"
	ingestBatchNameKey          = "ingestion-name"

	LogIngestURI = "public.ecr.aws/resim/open-builds/log-ingest:latest"
)

func init() {
	ingestLogCmd.Flags().Bool(ingestGithubKey, false, "Whether to output format in github action friendly format")
	// Project
	ingestLogCmd.Flags().String(ingestProjectKey, "", "The name or ID of the project to associate with the log")
	ingestLogCmd.MarkFlagRequired(ingestProjectKey)
	// Build ID
	ingestLogCmd.Flags().String(ingestBuildKey, "", "The ID of the build to use to pre-process the log. If not provided, the default ReSim log ingest build will be used to simply copy the log to the correct location.")
	// System
	ingestLogCmd.Flags().String(ingestSystemKey, "", "The name or ID of the system that generated the log")
	ingestLogCmd.MarkFlagsMutuallyExclusive(ingestBuildKey, ingestSystemKey)
	// Branch
	ingestLogCmd.Flags().String(ingestBranchKey, "log-ingest-branch", "The name or ID of the branch of the software that generated the log; if not provided, a default branch `log-ingest-branch` will be used")
	// Build version
	ingestLogCmd.Flags().String(ingestVersionKey, "latest", "The version (often commit SHA) of the software that generated the log; if not provided, a default version `latest` will be used")
	ingestLogCmd.MarkFlagsMutuallyExclusive(ingestBuildKey, ingestVersionKey)
	ingestLogCmd.MarkFlagsMutuallyExclusive(ingestBuildKey, ingestBranchKey)
	// Metrics Build
	ingestLogCmd.Flags().String(ingestMetricsBuildKey, "", "The ID of the metrics build to use in processing this log.")
	ingestLogCmd.MarkFlagRequired(ingestMetricsBuildKey)
	// Log Name
	ingestLogCmd.Flags().String(ingestLogNameKey, "", "A project-unique name to use in processing this log, often a run id.")
	// Log Location
	ingestLogCmd.Flags().String(ingestExperienceLocationKey, "", "An S3 prefix, which ReSim has access to, where the log is stored.")
	ingestLogCmd.Flags().StringArray(ingestLogKey, []string{}, "Log name and location pairs in the format 'name=s3://location'. Can be specified multiple times.")
	ingestLogCmd.Flags().String(ingestConfigFileKey, "", "Path to YAML file containing log configurations")
	// Support the old way, a config file, and the --log flag mutually exclusively:
	ingestLogCmd.MarkFlagsRequiredTogether(ingestLogNameKey, ingestExperienceLocationKey)
	ingestLogCmd.MarkFlagsOneRequired(ingestLogNameKey, ingestConfigFileKey, ingestLogKey)
	ingestLogCmd.MarkFlagsMutuallyExclusive(ingestLogNameKey, ingestConfigFileKey)
	ingestLogCmd.MarkFlagsMutuallyExclusive(ingestLogNameKey, ingestLogKey)
	ingestLogCmd.MarkFlagsMutuallyExclusive(ingestConfigFileKey, ingestLogKey)
	// Tags
	ingestLogCmd.Flags().StringSlice(ingestExperienceTagsKey, []string{}, "Comma-separated list of tags to apply. ReSim will automatically add the `ingested-via-resim` tag.")
	// Batch Name
	ingestLogCmd.Flags().String(ingestBatchNameKey, "", "A memorable name for this batch of logs to ingest. If not provided, a default name will be generated.")
	rootCmd.AddCommand(ingestLogCmd)
}

type logPair struct {
	name     string
	location string
}
type LogConfig struct {
	Name     string `yaml:"name"`
	Location string `yaml:"location"`
}

type LogsFile struct {
	Logs []LogConfig `yaml:"logs"`
}

func ingestLog(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	logIngestGithub := viper.GetBool(ingestGithubKey)
	if !logIngestGithub {
		fmt.Println("Ingesting a log...")
	}

	var buildID uuid.UUID
	var err error
	if viper.IsSet(ingestBuildKey) {
		buildID, err = uuid.Parse(viper.GetString(ingestBuildKey))
		if err != nil {
			log.Fatal("invalid build ID")
		}
	} else {
		// Create a build using the ReSim standard log ingest build:
		systemID := getSystemID(Client, projectID, viper.GetString(ingestSystemKey), true)
		// Check the branch exists:
		branchID := getOrCreateBranchID(Client, projectID, viper.GetString(ingestBranchKey), logIngestGithub)
		buildID = getOrCreateBuild(Client, projectID, branchID, systemID, LogIngestURI, viper.GetString(ingestVersionKey))
	}

	var logsToProcess []LogConfig

	// Get logs from config file if provided
	if viper.IsSet(ingestConfigFileKey) {
		configLogs, err := readLogsFromConfig(viper.GetString(ingestConfigFileKey))
		if err != nil {
			log.Fatal("Error reading config file:", err)
		}
		logsToProcess = configLogs
	}

	// Add logs from command line flags if provided
	if viper.IsSet(ingestLogKey) {
		logPairs, err := parseLogPairs(viper.GetStringSlice(ingestLogKey))
		if err != nil {
			log.Fatal(err)
		}
		for _, pair := range logPairs {
			logsToProcess = append(logsToProcess, LogConfig{
				Name:     pair.name,
				Location: pair.location,
			})
		}
	}

	// Use the old way of `--log-name` and `--log-location`
	if viper.IsSet(ingestLogNameKey) && viper.IsSet(ingestExperienceLocationKey) {
		logsToProcess = append(logsToProcess, LogConfig{
			Name:     viper.GetString(ingestLogNameKey),
			Location: viper.GetString(ingestExperienceLocationKey),
		})
	}

	// Validate we have at least one log to process
	if len(logsToProcess) == 0 {
		log.Fatal("No logs specified. Use --log flags or --config-file to specify logs to ingest")
	}

	// Check for duplicate names
	nameSet := make(map[string]bool)
	for _, l := range logsToProcess {
		if nameSet[l.Name] {
			log.Fatal("Duplicate log name found:", l.Name)
		}
		nameSet[l.Name] = true
	}

	// Process each log
	experienceIDs := []uuid.UUID{}
	for _, logConfig := range logsToProcess {
		if !logIngestGithub {
			fmt.Printf("Processing log: %s\n", logConfig.Name)
		}

		// Create the experience
		experienceBody := api.CreateExperienceInput{
			Name:        logConfig.Name,
			Location:    logConfig.Location,
			Description: "Ingested into ReSim via the CLI",
		}
		experienceResponse, err := Client.CreateExperienceWithResponse(context.Background(), projectID, experienceBody)
		if err != nil {
			log.Fatal("unable to create experience:", err)
		}
		ValidateResponse(http.StatusCreated, "unable to create experience", experienceResponse.HTTPResponse, experienceResponse.Body)
		if experienceResponse.JSON201 == nil {
			log.Fatal("empty response")
		}
		experience := *experienceResponse.JSON201
		experienceID := experience.ExperienceID
		experienceIDs = append(experienceIDs, experienceID)
		// Merge global tags with log-specific tags
		allTags := append([]string{}, viper.GetStringSlice(ingestExperienceTagsKey)...)
		allTags = append(allTags, "ingested-via-resim")

		// Create or get any associated experience tags:
		for _, tag := range allTags {
			getOrCreateExperienceTagID(Client, projectID, tag)
			tagExperienceHelper(Client, projectID, experienceID, tag)
		}
	}

	// Process the associated account: by default, we try to get from CI/CD environment variables
	// Otherwise, we use the account flag. The default is "".
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(batchAccountKey) {
		associatedAccount = viper.GetString(batchAccountKey)
	}

	// Validate the metrics build exists:
	metricsBuildID, err := uuid.Parse(viper.GetString(ingestMetricsBuildKey))
	if err != nil || metricsBuildID == uuid.Nil {
		log.Fatal("Metrics build ID is required")
	}

	// Finally, create a batch to process the log(s)
	batchBody := api.BatchInput{
		ExperienceIDs:     Ptr(experienceIDs),
		BuildID:           Ptr(buildID),
		AssociatedAccount: &associatedAccount,
		TriggeredVia:      DetermineTriggerMethod(),
		MetricsBuildID:    Ptr(metricsBuildID),
	}
	if viper.IsSet(ingestBatchNameKey) {
		batchBody.BatchName = Ptr(viper.GetString(ingestBatchNameKey))
	}

	batchResponse, err := Client.CreateBatchWithResponse(context.Background(), projectID, batchBody)
	if err != nil {
		log.Fatal("unable to create batch:", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create batch", batchResponse.HTTPResponse, batchResponse.Body)
	if batchResponse.JSON201 == nil {
		log.Fatal("empty response")
	}
	batch := *batchResponse.JSON201
	if batch.BatchID == nil {
		log.Fatal("no batch ID")
	}
	// Report the results back to the user
	if logIngestGithub {
		fmt.Printf("batch_id=%s\n", batch.BatchID.String())
	} else {
		fmt.Println("Ingested logs successfully!")
		fmt.Printf("Batch ID: %s\n", batch.BatchID.String())
		if resimURL := maybeGenerateResimURL(projectID, *batch.BatchID); resimURL != "" {
			fmt.Printf("View the results at %s\n", resimURL)
		}
	}
}

// Index over the imageURI and the version to determine whether or not we want to create a new build:
func getOrCreateBuild(client api.ClientWithResponsesInterface, projectID uuid.UUID, branchID uuid.UUID, systemID uuid.UUID, imageURI string, version string) uuid.UUID {
	buildID := getBuildIDFromImageURIAndVersion(client, projectID, systemID, branchID, imageURI, version, false)
	if buildID != uuid.Nil {
		return buildID
	}
	body := api.CreateBuildForBranchInput{
		Description: Ptr("A ReSim Log Ingest Build"),
		ImageUri:    LogIngestURI,
		Version:     viper.GetString(ingestVersionKey),
		SystemID:    systemID,
	}
	response, err := Client.CreateBuildForBranchWithResponse(context.Background(), projectID, branchID, body)
	if err != nil {
		log.Fatal("unable to create build:", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create build", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	build := *response.JSON201
	return build.BuildID
}

func parseLogPairs(pairs []string) ([]logPair, error) {
	var results []logPair
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid log pair format: %s (expected 'name=location')", pair)
		}

		name := strings.TrimSpace(parts[0])
		location := strings.TrimSpace(parts[1])

		if name == "" || location == "" {
			return nil, fmt.Errorf("both name and location must be non-empty in pair: %s", pair)
		}

		results = append(results, logPair{
			name:     name,
			location: location,
		})
	}
	return results, nil
}

func readLogsFromConfig(configPath string) ([]LogConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config LogsFile
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(config.Logs) == 0 {
		return nil, fmt.Errorf("no logs defined in config file")
	}

	// Validate the config
	for i, log := range config.Logs {
		if log.Name == "" {
			return nil, fmt.Errorf("log at index %d missing name", i)
		}
		if log.Location == "" {
			return nil, fmt.Errorf("log at index %d missing location", i)
		}
	}

	return config.Logs, nil
}

func maybeGenerateResimURL(projectID uuid.UUID, batchID uuid.UUID) string {
	// Generate resim url for the test:
	apiURL := viper.GetString(urlKey)
	baseURL := ""
	if apiURL == stagingAPIURL {
		baseURL = "https://app.resim.io/"
	} else if apiURL == prodAPIURL {
		baseURL = "https://app.resim.ai/"
	}
	if baseURL != "" {
		return fmt.Sprintf("%s/projects/%s/batches/%s", baseURL, projectID.String(), batchID.String())
	}
	return ""
}
