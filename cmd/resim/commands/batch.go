package commands

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/manifoldco/promptui"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	batchCmd = &cobra.Command{
		Use:     "batches",
		Short:   "batches contains commands for creating and managing batches",
		Long:    ``,
		Aliases: []string{"batch"},
	}
	createBatchCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new batch",
		Long:  ``,
		Run:   createBatch,
	}
	getBatchCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves a batch",
		Long:  ``,
		Run:   getBatch,
	}

	cancelBatchCmd = &cobra.Command{
		Use:   "cancel",
		Short: "cancel - Cancel a batch",
		Long:  ``,
		Run:   cancelBatch,
	}

	jobsBatchCmd = &cobra.Command{
		Use:   "jobs",
		Short: "jobs - Lists the jobs in a batch",
		Long:  ``,
		Run:   jobsBatch,
	}

	waitBatchCmd = &cobra.Command{
		Use:   "wait",
		Short: "wait - Wait for batch completion",
		Long:  `Awaits batch completion and returns an exit code corresponding to the batch status. 1 = internal error, 0 = SUCCEEDED, 2=ERROR, 5=CANCELLED, 6=timed out)`,
		Run:   waitBatch,
	}

	logsBatchCmd = &cobra.Command{
		Use:   "logs",
		Short: "logs - Lists the logs associated with a batch",
		Long:  ``,
		Run:   listBatchLogs,
	}
)

const (
	batchProjectKey            = "project"
	batchBuildIDKey            = "build-id"
	batchExperienceIDsKey      = "experience-ids"
	batchExperiencesKey        = "experiences"
	batchExperienceTagIDsKey   = "experience-tag-ids"
	batchExperienceTagNamesKey = "experience-tag-names"
	batchExperienceTagsKey     = "experience-tags"
	batchParameterKey          = "parameter"
	batchIDKey                 = "batch-id"
	batchNameKey               = "batch-name"
	batchGithubKey             = "github"
	batchMetricsBuildKey       = "metrics-build-id"
	batchExitStatusKey         = "exit-status"
	batchWaitTimeoutKey        = "wait-timeout"
	batchWaitPollKey           = "poll-every"
)

func init() {
	createBatchCmd.Flags().Bool(batchGithubKey, false, "Whether to output format in github action friendly format")
	createBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project to associate with the batch")
	createBatchCmd.MarkFlagRequired(batchProjectKey)
	createBatchCmd.Flags().String(batchBuildIDKey, "", "The ID of the build.")
	createBatchCmd.MarkFlagRequired(batchBuildIDKey)
	createBatchCmd.Flags().String(batchMetricsBuildKey, "", "The ID of the metrics build to use in this batch.")
	// the separate ID and name flags for experiences and experience tags are kept for backwards compatibility
	createBatchCmd.Flags().String(batchExperienceIDsKey, "", "Comma-separated list of experience IDs to run.")
	createBatchCmd.Flags().String(batchExperiencesKey, "", "List of experience names or list of experience IDs to run, comma-separated")
	createBatchCmd.Flags().String(batchExperienceTagIDsKey, "", "Comma-separated list of experience tag IDs to run.")
	createBatchCmd.Flags().String(batchExperienceTagNamesKey, "", "Comma-separated list of experience tag names to run.")
	createBatchCmd.Flags().String(batchExperienceTagsKey, "", "List of experience tag names or list of experience tag IDs to run, comma-separated.")
	createBatchCmd.Flags().StringSlice(batchParameterKey, []string{}, "(Optional) Parameter overrides to pass to the build. Format: <parameter-name>:<parameter-value>. Accepts repeated parameters or comma-separated parameters.")
	createBatchCmd.MarkFlagsOneRequired(batchExperienceIDsKey, batchExperiencesKey, batchExperienceTagIDsKey, batchExperienceTagNamesKey, batchExperienceTagsKey)
	batchCmd.AddCommand(createBatchCmd)

	getBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	getBatchCmd.MarkFlagRequired(batchProjectKey)
	getBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve.")
	getBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	getBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	getBatchCmd.Flags().Bool(batchExitStatusKey, false, "If set, exit code corresponds to batch status (1 = internal error, 0 = SUCCEEDED, 2=ERROR, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	batchCmd.AddCommand(getBatchCmd)

	cancelBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	cancelBatchCmd.MarkFlagRequired(batchProjectKey)
	cancelBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to cancel.")
	cancelBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to cancel (e.g. rejoicing-aquamarine-starfish).")
	cancelBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	batchCmd.AddCommand(cancelBatchCmd)

	jobsBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	jobsBatchCmd.MarkFlagRequired(batchProjectKey)
	jobsBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve jobs for.")
	jobsBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	jobsBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	batchCmd.AddCommand(jobsBatchCmd)

	waitBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	waitBatchCmd.MarkFlagRequired(batchProjectKey)
	waitBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to await completion.")
	waitBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to await completion (e.g. rejoicing-aquamarine-starfish).")
	waitBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	waitBatchCmd.Flags().String(batchWaitTimeoutKey, "1h", "Amount of time to wait for a batch to finish, expressed in Golang duration string.")
	waitBatchCmd.Flags().String(batchWaitPollKey, "30s", "Interval between checking batch status, expressed in Golang duration string.")
	batchCmd.AddCommand(waitBatchCmd)

	logsBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	logsBatchCmd.MarkFlagRequired(batchProjectKey)
	logsBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to list logs for.")
	logsBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to list logs for (e.g. rejoicing-aquamarine-starfish).")
	logsBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	logsBatchCmd.MarkFlagsOneRequired(batchIDKey, batchNameKey)
	batchCmd.AddCommand(logsBatchCmd)

	rootCmd.AddCommand(batchCmd)
}

func createBatch(ccmd *cobra.Command, args []string) {
	metricsBuildCompatible := true
	incompatibleExperienceNames := []string{}

	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batchGithub := viper.GetBool(batchGithubKey)
	if !batchGithub {
		fmt.Println("Creating a batch...")
	}

	// Parse the build ID
	buildID, err := uuid.Parse(viper.GetString(batchBuildIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal("failed to parse build ID: ", err)
	}

	// Obtain the system:
	build := actualGetBuild(projectID, buildID)
	if build.SystemID == nil {
		log.Fatal("empty system ID")
	}
	system := actualGetSystem(projectID, *build.SystemID)
	systemID := *system.SystemID

	var allExperienceIDs []uuid.UUID
	var allExperienceNames []string

	// Parse --experience-ids
	if viper.IsSet(batchExperienceIDsKey) {
		experienceIDs := parseUUIDs(viper.GetString(batchExperienceIDsKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
	}

	// Parse --experiences into either IDs or names
	if viper.IsSet(batchExperiencesKey) {
		experienceIDs, experienceNames := parseUUIDsAndNames(viper.GetString(batchExperiencesKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
		allExperienceNames = append(allExperienceNames, experienceNames...)
	}

	metricsBuildID := uuid.Nil
	if viper.IsSet(batchMetricsBuildKey) {
		metricsBuildID, err = uuid.Parse(viper.GetString(batchMetricsBuildKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
		fmt.Println("Checking the compatiblity")
		metricsBuildCompatible = checkSystemMetricsBuildCompatibility(projectID, systemID, metricsBuildID)
	}

	if viper.IsSet(batchExperienceTagIDsKey) && viper.IsSet(batchExperienceTagNamesKey) {
		log.Fatalf("failed to create batch: %v and %v are mutually exclusive parameters", batchExperienceTagNamesKey, batchExperienceTagIDsKey)
	}

	var allExperienceTagIDs []uuid.UUID
	var allExperienceTagNames []string

	// Parse --experience-tag-ids
	if viper.IsSet(batchExperienceTagIDsKey) {
		experienceTagIDs := parseUUIDs(viper.GetString(batchExperienceTagIDsKey))
		allExperienceTagIDs = append(allExperienceTagIDs, experienceTagIDs...)
	}

	// Parse --experience-tag-names:
	if viper.IsSet(batchExperienceTagNamesKey) {
		experienceTagNames := strings.Split(viper.GetString(batchExperienceTagNamesKey), ",")
		for i := range experienceTagNames {
			experienceTagNames[i] = strings.TrimSpace(experienceTagNames[i])
		}
		allExperienceTagNames = append(allExperienceTagNames, experienceTagNames...)
	}

	// Parse --experience-tags
	if viper.IsSet(batchExperienceTagsKey) {
		experienceTagIDs, experienceTagNames := parseUUIDsAndNames(viper.GetString(batchExperienceTagsKey))
		allExperienceTagIDs = append(allExperienceTagIDs, experienceTagIDs...)
		allExperienceTagNames = append(allExperienceTagNames, experienceTagNames...)
	}

	// Parse --parameter (if any provided)
	parameters := api.BatchParameters{}
	if viper.IsSet(batchParameterKey) {
		parameterStrings := viper.GetStringSlice(batchParameterKey)
		for _, parameterString := range parameterStrings {
			parameter := strings.Split(parameterString, ":")
			if len(parameter) != 2 {
				log.Fatal("failed to parse parameter: ", parameterString, " - must be in the format <parameter-name>:<parameter-value>")
			}
			parameters[parameter[0]] = parameter[1]
		}
	}

	compatibleExperiences := getCompatibleExperiences(projectID, systemID)
	// Validate the experience ID list
	for _, experienceID := range allExperienceIDs {
		found := false
		for _, compatibleExperience := range compatibleExperiences {
			if *compatibleExperience.ExperienceID == experienceID {
				found = true
				break
			}
		}
		if !found {
			missingExperience := actualGetExperience(projectID, experienceID)
			incompatibleExperienceNames = append(incompatibleExperienceNames, *missingExperience.Name)
		}
	}
	// Validate the experience name list:
	for _, experienceName := range allExperienceNames {
		found := false
		for _, compatibleExperience := range compatibleExperiences {
			if *compatibleExperience.Name == experienceName {
				found = true
				break
			}
		}
		if !found {
			incompatibleExperienceNames = append(incompatibleExperienceNames, experienceName)
		}
	}

	// Build the request body
	body := api.BatchInput{
		BuildID:    &buildID,
		Parameters: &parameters,
	}

	if allExperienceIDs != nil {
		body.ExperienceIDs = &allExperienceIDs
	}

	if allExperienceNames != nil {
		body.ExperienceNames = &allExperienceNames
	}

	if allExperienceTagIDs != nil {
		body.ExperienceTagIDs = &allExperienceTagIDs
	}

	if allExperienceTagNames != nil {
		body.ExperienceTagNames = &allExperienceTagNames
	}

	if metricsBuildID != uuid.Nil {
		body.MetricsBuildID = &metricsBuildID
	}

	if !batchGithub {
		// If the metrics build is incompatible or there are incompatible experiences, prompt the user:
		if !metricsBuildCompatible {
			wordPromptContent := promptContent{
				"Please choose either Y/n.",
				"The metrics build you have chosen is not registered as compatible with the build. Are you sure you want to continue? [Y/n]",
			}
			word := promptGetInput(wordPromptContent)
			if word == "n" {
				log.Fatal("Batch not created, due to incompatible metrics build")
			}
		}
		if len(incompatibleExperienceNames) > 0 {
			wordPromptContent := promptContent{
				"Please choose either Y/n.",
				fmt.Sprintf("The following experience(s) are not compatible with the system:\n %v. Are you sure you want to continue? [Y/n]", incompatibleExperienceNames),
			}
			word := promptGetInput(wordPromptContent)
			if word == "n" {
				log.Fatal("Batch not created, due to incompatible experiences")
			}
		}
	}
	// Make the request
	response, err := Client.CreateBatchWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create batch:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create batch", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	batch := *response.JSON201

	if !batchGithub {
		// Report the results back to the user
		fmt.Println("Created batch successfully!")
	}
	if batch.BatchID == nil {
		log.Fatal("empty ID")
	}
	if !batchGithub {
		fmt.Println("Batch ID:", batch.BatchID.String())
	} else {
		fmt.Printf("batch_id=%s\n", batch.BatchID.String())
	}
	if batch.FriendlyName == nil {
		log.Fatal("empty name")
	}
	if !batchGithub {
		fmt.Println("Batch name:", *batch.FriendlyName)
	}
	if batch.Status == nil {
		log.Fatal("empty status")
	}
	if !batchGithub {
		fmt.Println("Status:", *batch.Status)
	}
}

func actualGetBatch(projectID uuid.UUID, batchIDRaw string, batchName string) *api.Batch {
	var batch *api.Batch
	if batchIDRaw != "" {
		batchID, err := uuid.Parse(batchIDRaw)
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
		response, err := Client.GetBatchWithResponse(context.Background(), projectID, batchID)
		if err != nil {
			log.Fatal("unable to retrieve batch:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve batch", response.HTTPResponse, response.Body)
		batch = response.JSON200
		return batch
	} else if batchName != "" {
		var pageToken *string = nil
		for {
			response, err := Client.ListBatchesWithResponse(context.Background(), projectID, &api.ListBatchesParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list batches:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list batches", response.HTTPResponse, response.Body)
			if response.JSON200.Batches == nil {
				log.Fatal("unable to find batch: ", batchName)
			}
			batches := *response.JSON200.Batches

			for _, b := range batches {
				if b.FriendlyName != nil && *b.FriendlyName == batchName {
					batch = &b
					return batch
				}
			}

			if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
				pageToken = response.JSON200.NextPageToken
			} else {
				log.Fatal("unable to find batch: ", batchName)
			}
		}
	} else {
		log.Fatal("must specify either the batch ID or the batch name")
	}
	return batch
}

func getBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batch := actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))

	if viper.GetBool(batchExitStatusKey) {
		if batch.Status == nil {
			log.Fatal("no status returned")
		}
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			os.Exit(0)
		case api.BatchStatusERROR:
			os.Exit(2)
		case api.BatchStatusSUBMITTED:
			os.Exit(3)
		case api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
			os.Exit(4)
		case api.BatchStatusCANCELLED:
			os.Exit(5)
		default:
			log.Fatal("unknown batch status: ", batch.Status)
		}
	}

	OutputJson(batch)
}

func waitBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	var batch *api.Batch
	timeout, _ := time.ParseDuration(viper.GetString(batchWaitTimeoutKey))
	pollWait, _ := time.ParseDuration(viper.GetString(batchWaitPollKey))
	startTime := time.Now()
	for {
		batch = actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))
		if batch.Status == nil {
			log.Fatal("no status returned")
		}
		viper.Set(batchIDKey, batch.BatchID.String())
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			os.Exit(0)
		case api.BatchStatusERROR:
			os.Exit(2)
		case api.BatchStatusSUBMITTED, api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
		case api.BatchStatusCANCELLED:
			os.Exit(5)
		default:
			log.Fatal("unknown batch status: ", *batch.Status)
		}

		if time.Now().After(startTime.Add(timeout)) {
			log.Fatalf("Failed to reach a final state after %v, last state %s", timeout, *batch.Status)
			os.Exit(6)
		}
		time.Sleep(pollWait)
	}
}

func jobsBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	var batchID uuid.UUID
	var err error
	if viper.IsSet(batchIDKey) {
		batchID, err = uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
	} else if viper.IsSet(batchNameKey) {
		batch := actualGetBatch(projectID, "", viper.GetString(batchNameKey))
		batchID = *batch.BatchID
	} else {
		log.Fatal("must specify either the batch ID or the batch name")
	}

	// Now list the jobs
	jobs := []api.Job{}
	var pageToken *string = nil
	for {
		response, err := Client.ListJobsWithResponse(context.Background(), projectID, batchID, &api.ListJobsParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("unable to list jobs:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list jobs", response.HTTPResponse, response.Body)
		if response.JSON200.Jobs == nil {
			log.Fatal("unable to list jobs")
		}
		responseJobs := *response.JSON200.Jobs
		jobs = append(jobs, responseJobs...)

		if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
			pageToken = response.JSON200.NextPageToken
		} else {
			break
		}
	}
	OutputJson(jobs)
}

func listBatchLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	var batchID uuid.UUID
	var err error
	if viper.IsSet(batchIDKey) {
		batchID, err = uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
	} else {
		batch := actualGetBatch(projectID, "", viper.GetString(batchNameKey))
		batchID = *batch.BatchID
	}
	logs := []api.BatchLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListBatchLogsForBatchWithResponse(context.Background(), projectID, batchID, &api.ListBatchLogsForBatchParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("unable to list logs:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list logs", response.HTTPResponse, response.Body)
		if response.JSON200.Logs == nil {
			log.Fatal("unable to list logs")
		}
		responseLogs := *response.JSON200.Logs
		logs = append(logs, responseLogs...)

		if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
			pageToken = response.JSON200.NextPageToken
		} else {
			break
		}
	}
	OutputJson(logs)
}

func cancelBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batch := actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))

	response, err := Client.CancelBatchWithResponse(context.Background(), projectID, *batch.BatchID)
	if err != nil {
		log.Fatal("failed to cancel batch:", err)
	}
	ValidateResponse(http.StatusOK, "failed to cancel batch", response.HTTPResponse, response.Body)
	fmt.Println("Batch cancelled successfully!")
}

// Helpers
func checkSystemMetricsBuildCompatibility(projectID uuid.UUID, systemID uuid.UUID, metricsBuildID uuid.UUID) bool {
	found := false
	var pageToken *string = nil
pageLoop:
	for {
		// Check if the metrics build is compatible with the system
		response, err := Client.GetSystemsForMetricsBuildWithResponse(context.Background(), projectID, metricsBuildID, &api.GetSystemsForMetricsBuildParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("failed to list systems for metrics build:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list systems for metrics build", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response when listing systems for metrics build")
		}
		pageToken = response.JSON200.NextPageToken
		systems := *response.JSON200.Systems
		for _, s := range systems {
			if *s.SystemID == systemID {
				found = true
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return found
}

func getCompatibleExperiences(projectID uuid.UUID, systemID uuid.UUID) []api.Experience {
	var pageToken *string = nil
	var compatibleExperiences []api.Experience
	for {
		// Page through the applicable experiecnes
		response, err := Client.ListExperiencesForSystemWithResponse(context.Background(), projectID, systemID, &api.ListExperiencesForSystemParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("failed to list experiences for system:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list experiences for system", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response when listing experiences for system")
		}
		pageToken = response.JSON200.NextPageToken
		compatibleExperiences = append(compatibleExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return compatibleExperiences
}

type promptContent struct {
	errorMsg string
	label    string
}

func promptGetInput(pc promptContent) string {
	validate := func(input string) error {
		if input != "Y" && input != "n" {
			return errors.New(pc.errorMsg)
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     pc.label,
		Templates: templates,
		Validate:  validate,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Input: %s\n", result)

	return result
}
