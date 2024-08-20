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
		Short: "ingest - Ingests a new log and generates metrics analysis",
		Long:  ``,
		Run:   ingestLog,
	}
)

const (
	ingestProjectKey            = "project"
	ingestSystemKey             = "system"
	ingestBranchKey             = "branch"
	ingestVersionKey            = "version"
	ingestExperienceNameKey     = "experience-name"
	ingestExperienceLocationKey = "remote-log-location"
	ingestExperienceTagsKey     = "tags"
	ingestGithubKey             = "github"
	ingestMetricsBuildKey       = "metrics-build-id"
)

func init() {
	ingestLogCmd.Flags().Bool(ingestGithubKey, false, "Whether to output format in github action friendly format")
	// Project
	ingestLogCmd.Flags().String(ingestProjectKey, "", "The name or ID of the project to associate with the log")
	ingestLogCmd.MarkFlagRequired(ingestProjectKey)
	// System
	ingestLogCmd.Flags().String(ingestSystemKey, "", "The name or ID of the system that generated the log")
	ingestLogCmd.MarkFlagRequired(ingestSystemKey)
	// Branch
	ingestLogCmd.Flags().String(ingestBranchKey, "", "The name or ID of the branch of the software that generated the log")
	ingestLogCmd.MarkFlagRequired(ingestBranchKey) // TODO: make optional
	// Build
	ingestLogCmd.Flags().String(ingestVersionKey, "", "The version (often commit sha) of the software that generated the log")
	ingestLogCmd.MarkFlagRequired(ingestVersionKey) // TODO: make optional
	// Metrics Build
	ingestLogCmd.Flags().String(ingestMetricsBuildKey, "", "The ID of the metrics build to use in processing this log.")
	ingestLogCmd.MarkFlagRequired(ingestMetricsBuildKey)
	// Log Name
	ingestLogCmd.Flags().String(ingestExperienceNameKey, "", "A project-unique name to use in processing this log, often a run id.")
	ingestLogCmd.MarkFlagRequired(ingestExperienceNameKey)
	// Log Location
	ingestLogCmd.Flags().String(ingestExperienceLocationKey, "", "An S3 prefix, which ReSim has access to, where the log is stored.")
	ingestLogCmd.MarkFlagRequired(ingestExperienceLocationKey)
	// Tags
	ingestLogCmd.Flags().String(ingestExperienceTagsKey, "", "Comma-separated list of tags to apply.") // TODO: implement
	rootCmd.AddCommand(ingestLogCmd)
}

func ingestLog(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	logGithub := viper.GetBool(ingestGithubKey)
	if !logGithub {
		fmt.Println("Ingesting a log...")
	}

	// Check the system exists:
	systemID := getSystemID(Client, projectID, viper.GetString(ingestSystemKey), true)
	// Check the branch exists:
	branchID := getBranchID(Client, projectID, viper.GetString(ingestBranchKey), true)
	// Create a build using the ReSim standard log ingest build:
	logIngestURI := "hello world"

	body := api.CreateBuildForBranchInput{
		Description: Ptr("A ReSim Log Ingest Build"),
		ImageUri:    logIngestURI,
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
	if build.BuildID == nil {
		log.Fatal("no build ID")
	}

	// Create the experience
	experienceBody := api.CreateExperienceInput{
		Name:        viper.GetString(ingestExperienceNameKey),
		Location:    viper.GetString(ingestExperienceLocationKey),
		Description: "A ReSim Log Ingest Experience",
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
	batchBody := api.BatchInput{
		ExperienceIDs:     Ptr([]uuid.UUID{experienceID}),
		BuildID:           build.BuildID,
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

	// Report the results back to the user
	if logGithub {
		fmt.Printf("batch_id=%s\n", batch.BatchID.String())
	} else {
		fmt.Println("Ingested log successfully!")
		fmt.Printf("Batch ID: %s\n", batch.BatchID.String())
	}
}
