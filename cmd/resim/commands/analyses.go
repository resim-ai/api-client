package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	analysisCmd = &cobra.Command{
		Use:     "analyses",
		Short:   "analyses contains commands for creating and running analyses",
		Long:    ``,
		Aliases: []string{"analysis"},
	}
	createAnalysisCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates and runs an Analysis over the selected recordings",
		Long: `Creates and runs an Analysis: a named, revisioned test suite over the selected
Recordings, executed as a normal batch (one job per Recording) that runs the
agentic build on the metrics-2 event path.`,
		Run: createAnalysis,
	}
)

const (
	analysisProjectKey     = "project"
	analysisNameKey        = "name"
	analysisRecordingsKey  = "recording-ids"
	analysisBuildIDKey     = "build-id"
	analysisCustomImageKey = "custom-image-build-id"
	analysisSkillIDKey     = "skill-id"
	analysisDescriptionKey = "description"
	analysisPoolLabelsKey  = "pool-labels"
	analysisBatchNameKey   = "batch-name"
	analysisGithubKey      = "github"
)

func init() {
	createAnalysisCmd.Flags().String(analysisProjectKey, "", "The name or ID of the project")
	createAnalysisCmd.MarkFlagRequired(analysisProjectKey)
	createAnalysisCmd.Flags().String(analysisNameKey, "", "A name for the Analysis")
	createAnalysisCmd.MarkFlagRequired(analysisNameKey)
	createAnalysisCmd.Flags().StringSlice(analysisRecordingsKey, []string{}, "Recording experience names or IDs to analyze, comma separated")
	createAnalysisCmd.MarkFlagRequired(analysisRecordingsKey)
	createAnalysisCmd.Flags().String(analysisBuildIDKey, "", "The ID of the agentic build to run")
	createAnalysisCmd.MarkFlagRequired(analysisBuildIDKey)
	createAnalysisCmd.Flags().String(analysisCustomImageKey, "", "Optional custom build image ID to run instead of the default agentic build (skill input tier c)")
	createAnalysisCmd.Flags().String(analysisSkillIDKey, "", "Optional catalog skill id to detect against (skill input tier a)")
	createAnalysisCmd.Flags().String(analysisDescriptionKey, "", "Optional plain-language detection prompt, also used as the Analysis description (skill input tier b)")
	createAnalysisCmd.Flags().StringSlice(analysisPoolLabelsKey, []string{}, "Optional pool labels, comma separated")
	createAnalysisCmd.Flags().String(analysisBatchNameKey, "", "Optional name for the batch that runs the Analysis")
	createAnalysisCmd.Flags().Bool(analysisGithubKey, false, "Whether to output format in github action friendly format")
	analysisCmd.AddCommand(createAnalysisCmd)

	rootCmd.AddCommand(analysisCmd)
}

func createAnalysis(ccmd *cobra.Command, args []string) {
	analysisGithub := viper.GetBool(analysisGithubKey)
	if !analysisGithub {
		fmt.Println("Creating an Analysis...")
	}

	projectID := getProjectID(Client, viper.GetString(analysisProjectKey))

	name := viper.GetString(analysisNameKey)
	if name == "" {
		log.Fatal("empty analysis name")
	}

	recordingIdentifiers := viper.GetStringSlice(analysisRecordingsKey)
	if len(recordingIdentifiers) == 0 {
		log.Fatal("empty recording-ids")
	}
	recordingIDs := make([]api.ExperienceID, 0, len(recordingIdentifiers))
	for _, identifier := range recordingIdentifiers {
		recordingIDs = append(recordingIDs, getExperienceID(Client, projectID, identifier, true, false))
	}

	buildID := getBuildID(Client, projectID, viper.GetString(analysisBuildIDKey))

	body := api.CreateAnalysisInput{
		Name:         name,
		BuildID:      buildID,
		RecordingIDs: recordingIDs,
	}

	if viper.IsSet(analysisCustomImageKey) {
		customImageBuildID := getBuildID(Client, projectID, viper.GetString(analysisCustomImageKey))
		body.CustomImageBuildID = &customImageBuildID
	}
	if viper.IsSet(analysisSkillIDKey) {
		body.SkillID = Ptr(viper.GetString(analysisSkillIDKey))
	}
	if viper.IsSet(analysisDescriptionKey) {
		body.Description = Ptr(viper.GetString(analysisDescriptionKey))
	}
	if viper.IsSet(analysisPoolLabelsKey) {
		poolLabels := api.PoolLabels(viper.GetStringSlice(analysisPoolLabelsKey))
		body.PoolLabels = &poolLabels
	}
	if viper.IsSet(analysisBatchNameKey) {
		batchName := api.Name(viper.GetString(analysisBatchNameKey))
		body.BatchName = &batchName
	}

	response, err := Client.CreateAnalysisWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create analysis: ", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create analysis", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	run := *response.JSON201

	var batchID string
	if run.Batch.BatchID != nil {
		batchID = run.Batch.BatchID.String()
	}

	if analysisGithub {
		if batchID != "" {
			fmt.Printf("batch_id=%s\n", batchID)
		}
	} else {
		fmt.Println("Created and started Analysis successfully!")
		if batchID != "" {
			fmt.Printf("Batch ID: %s\n", batchID)
		}
		OutputJson(run)
	}
}
