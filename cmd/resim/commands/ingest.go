package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	ingestLogCmd.MarkFlagRequired(ingestLogNameKey)
	// Log Location
	ingestLogCmd.Flags().String(ingestExperienceLocationKey, "", "An S3 prefix, which ReSim has access to, where the log is stored.")
	ingestLogCmd.MarkFlagRequired(ingestExperienceLocationKey)
	// Tags
	ingestLogCmd.Flags().StringSlice(ingestExperienceTagsKey, []string{}, "Comma-separated list of tags to apply. ReSim will automatically add the `ingested-via-resim` tag.")
	rootCmd.AddCommand(ingestLogCmd)
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

	// Create the experience
	experienceBody := api.CreateExperienceInput{
		Name:        viper.GetString(ingestLogNameKey),
		Location:    viper.GetString(ingestExperienceLocationKey),
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

	// Create or get any associated experience tags:
	experienceTags := viper.GetStringSlice(ingestExperienceTagsKey)
	experienceTags = append(experienceTags, "ingested-via-resim")
	for _, tag := range experienceTags {
		getOrCreateExperienceTagID(Client, projectID, tag)
		tagExperienceHelper(Client, projectID, experienceID, tag)
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

	// Finally, create a batch to process the log
	ingestionBatchName := fmt.Sprintf("Ingested Log: %s", experience.Name)
	batchBody := api.BatchInput{
		BatchName:         Ptr(ingestionBatchName),
		ExperienceIDs:     Ptr([]uuid.UUID{experienceID}),
		BuildID:           Ptr(buildID),
		AssociatedAccount: &associatedAccount,
		TriggeredVia:      DetermineTriggerMethod(),
		MetricsBuildID:    Ptr(metricsBuildID),
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

	// Get the jobs for this batch:
	jobsResponse, err := Client.ListJobsWithResponse(context.Background(), projectID, *batch.BatchID, &api.ListJobsParams{
		PageSize:  Ptr(10),
		PageToken: nil,
	})
	if err != nil {
		log.Fatal("unable to get jobs for batch:", err)
	}
	ValidateResponse(http.StatusOK, "unable to get jobs for batch", jobsResponse.HTTPResponse, jobsResponse.Body)
	if jobsResponse.JSON200 == nil {
		log.Fatal("empty response")
	}
	jobs := *jobsResponse.JSON200.Jobs
	if len(jobs) != 1 {
		log.Fatal("expected 1 job, got", len(jobs))
	}
	theJob := jobs[0]
	jobID := theJob.JobID

	// Report the results back to the user
	if logIngestGithub {
		fmt.Printf("batch_id=%s\n", batch.BatchID.String())
	} else {
		fmt.Println("Ingested log successfully!")
		fmt.Printf("Batch ID: %s\n", batch.BatchID.String())
		if resimURL := maybeGenerateResimURL(projectID, *batch.BatchID, *jobID); resimURL != "" {
			fmt.Printf("View the results at %s\n", resimURL)
		}
	}
}

func maybeGenerateResimURL(projectID uuid.UUID, batchID uuid.UUID, jobID uuid.UUID) string {
	// Generate resim url for the test:
	apiURL := viper.GetString(urlKey)
	baseURL := ""
	if apiURL == stagingAPIURL {
		baseURL = "https://app.resim.io/"
	} else if apiURL == prodAPIURL {
		baseURL = "https://app.resim.ai/"
	}
	if baseURL != "" {
		return fmt.Sprintf("%s/projects/%s/batches/%s/jobs/%s", baseURL, projectID.String(), batchID.String(), jobID.String())
	}
	return ""
}
