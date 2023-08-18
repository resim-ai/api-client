package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	batchCmd = &cobra.Command{
		Use:   "batch",
		Short: "batch contains commands for creating and managing batches",
		Long:  ``,
	}
	createBatchCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new batch",
		Long:   ``,
		Run:    createBatch,
		PreRun: RegisterViperFlags,
	}
	getBatchCmd = &cobra.Command{
		Use:    "get",
		Short:  "get - Retrieves a batch",
		Long:   ``,
		Run:    getBatch,
		PreRun: RegisterViperFlags,
	}

	jobsBatchCmd = &cobra.Command{
		Use:    "jobs",
		Short:  "jobs - Lists the jobs in a batch",
		Long:   ``,
		Run:    jobsBatch,
		PreRun: RegisterViperFlags,
	}
)

const (
	buildIDKey             = "build-id"
	experienceRevisionsKey = "experience-revisions"
	experienceTagIDsKey    = "experience-tag-ids"
	experienceTagNamesKey  = "experience-tag-names"

	batchIDKey    = "batch-id"
	batchNameKey  = "batch-name"
	exitStatusKey = "exit-status"
)

func init() {
	createBatchCmd.Flags().String(buildIDKey, "", "The ID of the build.")
	createBatchCmd.MarkFlagRequired(buildIDKey)
	createBatchCmd.Flags().String(experienceRevisionsKey, "", "Comma-separated list of experience revision to run. An experience revision is of the form {id}/{revision}.")
	createBatchCmd.Flags().String(experienceTagIDsKey, "", "Comma-separated list of experience tag ids to run.")
	createBatchCmd.Flags().String(experienceTagNamesKey, "", "Comma-separated list of experience tag names to run.")
	// TODO(simon) We want at least one of the above flags. The function we want
	// is: .MarkFlagsOneRequired this was merged into Cobra recently:
	// https://github.com/spf13/cobra/pull/1952 - but we need to wait for a stable
	// release and upgrade before implementing here.
	batchCmd.AddCommand(createBatchCmd)

	getBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve.")
	getBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	getBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	getBatchCmd.Flags().Bool(exitStatusKey, false, "If set, exit code corresponds to batch status (1 = error, 0 = SUCCEEDED, 2=FAILED, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	batchCmd.AddCommand(getBatchCmd)

	jobsBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve jobs for.")
	jobsBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	jobsBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	batchCmd.AddCommand(jobsBatchCmd)

	rootCmd.AddCommand(batchCmd)
}

// This function takes a single experience revision of the form {id}/{revision} and returns an experience revision.
// For example aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/1
func parseExperienceRevision(experienceRevisionString string) api.ExperienceRevision {
	if experienceRevisionString == "" {
		return api.ExperienceRevision{}
	}
	idString, revisionString, found := strings.Cut(experienceRevisionString, "/")
	if !found {
		log.Fatal("Experience revision must be of the form {id}/{revision}")
	}
	experienceID, err := uuid.Parse(strings.TrimSpace(idString))
	if err != nil {
		log.Fatal(err)
	}
	revision, err := strconv.ParseInt(strings.TrimSpace(revisionString), 10, 32)
	if err != nil {
		log.Fatal(err)
	}
	experienceRevision := api.ExperienceRevision{
		ExperienceID: Ptr(experienceID),
		Revision:     Ptr(int32(revision)),
	}
	return experienceRevision
}

// This function takes a comma-separated list of experience revisions represented as strings
// and returns a separated array of experience revisions. An experience revision is of the form {id}/{revision}.
// For example "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/1,aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/1"
func parseExperienceRevisions(commaSeparatedKeys string) []api.ExperienceRevision {
	if commaSeparatedKeys == "" {
		return []api.ExperienceRevision{}
	}
	strs := strings.Split(commaSeparatedKeys, ",")
	result := make([]api.ExperienceRevision, len(strs))

	for i := 0; i < len(strs); i++ {
		result[i] = parseExperienceRevision(strings.TrimSpace(strs[i]))
	}
	return result
}

func createBatch(ccmd *cobra.Command, args []string) {
	fmt.Println("Creating a batch...")

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the UUIDs from the command line
	buildID, err := uuid.Parse(viper.GetString(buildIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal(err)
	}

	if !viper.IsSet(experienceRevisionsKey) && !viper.IsSet(experienceTagIDsKey) && !viper.IsSet(experienceTagNamesKey) {
		log.Fatal("failed to create batch: you must choose at least one experience or experience tag to run")
	}

	if viper.IsSet(experienceTagIDsKey) && viper.IsSet(experienceTagNamesKey) {
		log.Fatal(fmt.Sprintf("failed to create batch: %v and %v are mutually exclusive parameters", experienceTagNamesKey, experienceTagIDsKey))
	}

	experienceRevisions := parseExperienceRevisions(viper.GetString("experience_revisions"))

	// Obtain experience tag ids.
	var experienceTagIDs []uuid.UUID
	// If the user passes IDs directly, parse them:
	if viper.GetString(experienceTagIDsKey) != "" {
		experienceTagIDs = parseUUIDs(viper.GetString(experienceTagIDsKey))
	}
	// If the user passes names, grab the ids:
	if viper.GetString(experienceTagNamesKey) != "" {
		experienceTagIDs = parseExperienceTagNames(client, viper.GetString(experienceTagNamesKey))
	}

	// Build the request body and make the request
	body := api.CreateBatchJSONRequestBody{
		BuildID:             &buildID,
		ExperienceRevisions: &experienceRevisions,
		ExperienceTagIDs:    &experienceTagIDs,
	}

	response, err := client.CreateBatchWithResponse(context.Background(), body)
	ValidateResponse(http.StatusCreated, "failed to create batch", response.HTTPResponse, err)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	batch := *response.JSON201

	// Report the results back to the user
	fmt.Println("Created Batch Successfully!")
	if batch.BatchID == nil {
		log.Fatal("empty ID")
	}
	fmt.Println("Batch ID:", batch.BatchID.String())
	if batch.FriendlyName == nil {
		log.Fatal("empty name")
	}
	fmt.Println("Batch name:", *batch.FriendlyName)
	if batch.Status == nil {
		log.Fatal("empty status")
	}
	fmt.Println("Status:", *batch.Status)
}

func getBatch(ccmd *cobra.Command, args []string) {
	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal("unable to create client: ", err)
	}

	var batch *api.Batch
	if viper.IsSet(batchIDKey) {
		batchID, err := uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
		response, err := client.GetBatchWithResponse(context.Background(), batchID)
		ValidateResponse(http.StatusOK, "unable to retrieve batch", response.HTTPResponse, err)
		batch = response.JSON200
	} else if viper.IsSet(batchNameKey) {
		batchName := viper.GetString(batchNameKey)
		var pageToken *string = nil
	pageLoop:
		for {
			response, err := client.ListBatchesWithResponse(context.Background(), &api.ListBatchesParams{
				PageToken: pageToken,
			})
			ValidateResponse(http.StatusOK, "unable to list batches", response.HTTPResponse, err)
			if response.JSON200.Batches == nil {
				log.Fatal("unable to find batch: ", batchName)
			}
			batches := *response.JSON200.Batches

			for _, b := range batches {
				if b.FriendlyName != nil && *b.FriendlyName == batchName {
					batch = &b
					break pageLoop
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

	if viper.GetBool(exitStatusKey) {
		if batch.Status == nil {
			log.Fatal("no status returned")
		}
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			os.Exit(0)
		case api.BatchStatusFAILED:
			os.Exit(2)
		case api.BatchStatusSUBMITTED:
			os.Exit(3)
		case api.BatchStatusRUNNING:
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

func jobsBatch(ccmd *cobra.Command, args []string) {
	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal("unable to create client: ", err)
	}

	var batchID uuid.UUID
	if viper.IsSet(batchIDKey) {
		batchID, err = uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
	} else if viper.IsSet(batchNameKey) {
		batchName := viper.GetString(batchNameKey)
		var pageToken *string = nil
	pageLoop:
		for {
			response, err := client.ListBatchesWithResponse(context.Background(), &api.ListBatchesParams{
				PageToken: pageToken,
			})
			ValidateResponse(http.StatusOK, "unable to list batches", response.HTTPResponse, err)
			if response.JSON200.Batches == nil {
				log.Fatal("unable to find batch: ", batchName)
			}
			batches := *response.JSON200.Batches

			for _, b := range batches {
				if b.FriendlyName != nil && *b.FriendlyName == batchName {
					batchID = *b.BatchID
					break pageLoop
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

	// Now list the jobs
	jobs := []api.Job{}
	var pageToken *string = nil
	for {
		response, err := client.ListJobsWithResponse(context.Background(), batchID, &api.ListJobsParams{
			PageToken: pageToken,
		})
		ValidateResponse(http.StatusOK, "unable to list jobs", response.HTTPResponse, err)
		if response.JSON200.Jobs == nil {
			log.Fatal("unable to list jobs")
		}
		responseJobs := *response.JSON200.Jobs
		for _, job := range responseJobs {
			jobs = append(jobs, job)
		}

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
