package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
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
)

const (
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
	// TODO(simon) We want at least one of the above flags. The function we want
	// is: .MarkFlagsOneRequired this was merged into Cobra recently:
	// https://github.com/spf13/cobra/pull/1952 - but we need to wait for a stable
	// release and upgrade before implementing here.
	batchCmd.AddCommand(createBatchCmd)

	getBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve.")
	getBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	getBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	getBatchCmd.Flags().Bool(batchExitStatusKey, false, "If set, exit code corresponds to batch status (1 = internal error, 0 = SUCCEEDED, 2=ERROR, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	batchCmd.AddCommand(getBatchCmd)

	jobsBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve jobs for.")
	jobsBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	jobsBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	batchCmd.AddCommand(jobsBatchCmd)

	waitBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to await completion.")
	waitBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to await completion (e.g. rejoicing-aquamarine-starfish).")
	waitBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	waitBatchCmd.Flags().String(batchWaitTimeoutKey, "1h", "Amount of time to wait for a batch to finish, expressed in Golang duration string.")
	waitBatchCmd.Flags().String(batchWaitPollKey, "30s", "Interval between checking batch status, expressed in Golang duration string.")
	batchCmd.AddCommand(waitBatchCmd)

	rootCmd.AddCommand(batchCmd)
}

func createBatch(ccmd *cobra.Command, args []string) {
	batchGithub := viper.GetBool(batchGithubKey)
	if !batchGithub {
		fmt.Println("Creating a batch...")
	}

	if !viper.IsSet(batchExperienceIDsKey) && !viper.IsSet(batchExperienceTagIDsKey) && !viper.IsSet(batchExperienceTagNamesKey) && !viper.IsSet(batchExperiencesKey) && !viper.IsSet(batchExperienceTagsKey) {
		log.Fatal("failed to create batch: you must choose at least one experience or experience tag to run")
	}

	// Parse the build ID
	buildID, err := uuid.Parse(viper.GetString(batchBuildIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal("failed to parse build ID: ", err)
	}

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

	// Make the request
	response, err := Client.CreateBatchWithResponse(context.Background(), body)
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

func actualGetBatch(batchIDRaw, batchName string) *api.Batch {
	var batch *api.Batch
	if batchIDRaw != "" {
		batchID, err := uuid.Parse(batchIDRaw)
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
		response, err := Client.GetBatchWithResponse(context.Background(), batchID)
		if err != nil {
			log.Fatal("unable to retrieve batch:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve batch", response.HTTPResponse, response.Body)
		batch = response.JSON200
		return batch
	} else if batchName != "" {
		var pageToken *string = nil
		for {
			response, err := Client.ListBatchesWithResponse(context.Background(), &api.ListBatchesParams{
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
	batch := actualGetBatch(viper.GetString(batchIDKey), viper.GetString(batchNameKey))

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

	bytes, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		log.Fatal("unable to serialize batch: ", err)
	}
	fmt.Println(string(bytes))
}

func waitBatch(ccmd *cobra.Command, args []string) {
	var batch *api.Batch
	timeout, _ := time.ParseDuration(viper.GetString(batchWaitTimeoutKey))
	pollWait, _ := time.ParseDuration(viper.GetString(batchWaitPollKey))
	startTime := time.Now()
	for {
		batch = actualGetBatch(viper.GetString(batchIDKey), viper.GetString(batchNameKey))
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
	var batchID uuid.UUID
	var err error
	if viper.IsSet(batchIDKey) {
		batchID, err = uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
	} else if viper.IsSet(batchNameKey) {
		batch := actualGetBatch("", viper.GetString(batchNameKey))
		batchID = *batch.BatchID
	} else {
		log.Fatal("must specify either the batch ID or the batch name")
	}

	// Now list the jobs
	jobs := []api.Job{}
	var pageToken *string = nil
	for {
		response, err := Client.ListJobsWithResponse(context.Background(), batchID, &api.ListJobsParams{
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
	bytes, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		log.Fatal("unable to serialize jobs: ", err)
	}
	fmt.Println(string(bytes))
}
