package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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
		Use:    "create",
		Short:  "create - Creates a new batch",
		Long:   ``,
		Run:    createBatch,
		PreRun: RegisterViperFlagsAndSetClient,
	}
	getBatchCmd = &cobra.Command{
		Use:    "get",
		Short:  "get - Retrieves a batch",
		Long:   ``,
		Run:    getBatch,
		PreRun: RegisterViperFlagsAndSetClient,
	}

	jobsBatchCmd = &cobra.Command{
		Use:    "jobs",
		Short:  "jobs - Lists the jobs in a batch",
		Long:   ``,
		Run:    jobsBatch,
		PreRun: RegisterViperFlagsAndSetClient,
	}
)

const (
	buildIDKey            = "build-id"
	experienceIDsKey      = "experience-ids"
	experiencesKey        = "experiences"
	experienceTagIDsKey   = "experience-tag-ids"
	experienceTagNamesKey = "experience-tag-names"
	experienceTagsKey     = "experience-tags"
	batchIDKey            = "batch-id"
	batchNameKey          = "batch-name"
	batchGithubKey        = "github"
	batchMetricsBuildKey  = "metrics-build-id"
	exitStatusKey         = "exit-status"
)

func init() {
	createBatchCmd.Flags().Bool(batchGithubKey, false, "Whether to output format in github action friendly format")
	createBatchCmd.Flags().String(buildIDKey, "", "The ID of the build.")
	createBatchCmd.MarkFlagRequired(buildIDKey)
	createBatchCmd.Flags().String(batchMetricsBuildKey, "", "The ID of the metrics build to use in this batch.")
	createBatchCmd.Flags().String(experienceIDsKey, "", "Comma-separated list of experience ids to run.")
	createBatchCmd.Flags().String(experiencesKey, "", "List of experience names or list of experience IDs to run, comma-separated")
	// these separate ID and name flags are kept for backwards compatibility
	createBatchCmd.Flags().String(experienceTagIDsKey, "", "Comma-separated list of experience tag ids to run.")
	createBatchCmd.Flags().String(experienceTagNamesKey, "", "Comma-separated list of experience tag names to run.")
	createBatchCmd.Flags().String(experienceTagsKey, "", "List of experience tag names or list of experience tag IDs to run, comma-separated.")
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

func createBatch(ccmd *cobra.Command, args []string) {
	batchGithub := viper.GetBool(batchGithubKey)
	if !batchGithub {
		fmt.Println("Creating a batch...")
	}

	// Parse the build ID
	buildID, err := uuid.Parse(viper.GetString(buildIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal("failed to parse build ID: ", err)
	}

	// Parse the experiences key into 2 slices
	experienceIDs, experienceNames := parseUUIDsAndNames(viper.GetString(experiencesKey))

	experienceTagIDs, experienceTagNames := parseUUIDsAndNames(viper.GetString(experienceTagsKey))

	metricsBuildID := uuid.Nil
	if viper.IsSet(batchMetricsBuildKey) {
		metricsBuildID, err = uuid.Parse(viper.GetString(batchMetricsBuildKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
	}

	if !viper.IsSet(experienceIDsKey) && !viper.IsSet(experienceTagIDsKey) && !viper.IsSet(experienceTagNamesKey) && !viper.IsSet(experiencesKey) && !viper.IsSet(experienceTagsKey) {
		log.Fatal("failed to create batch: you must choose at least one experience or experience tag to run")
	}

	if viper.IsSet(experienceTagIDsKey) && viper.IsSet(experienceTagNamesKey) {
		log.Fatal(fmt.Sprintf("failed to create batch: %v and %v are mutually exclusive parameters", experienceTagNamesKey, experienceTagIDsKey))
	}
	// Build the request body
	body := api.CreateBatchJSONRequestBody{
		BuildID:          &buildID,
		ExperienceIDs:    &experienceIDs,
		ExperienceTagIDs: &experienceTagIDs,
	}

	if experienceIDs != nil {
		body.ExperienceIDs = &experienceIDs
	}

	if experienceNames != nil {
		body.ExperienceNames = &experienceNames
	}

	if experienceTagIDs != nil {
		body.ExperienceTagIDs = &experienceTagIDs
	}

	if experienceTagNames != nil {
		body.ExperienceTagNames = &experienceTagNames
	}

	if metricsBuildID != uuid.Nil {
		body.MetricsBuildID = &metricsBuildID
	}

	// Make the request
	response, err := Client.CreateBatchWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal("failed to create batch:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create batch", response.HTTPResponse)

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

func getBatch(ccmd *cobra.Command, args []string) {
	var batch *api.Batch
	if viper.IsSet(batchIDKey) {
		batchID, err := uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
		response, err := Client.GetBatchWithResponse(context.Background(), batchID)
		if err != nil {
			log.Fatal("unable to retrieve batch:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve batch", response.HTTPResponse)
		batch = response.JSON200
	} else if viper.IsSet(batchNameKey) {
		batchName := viper.GetString(batchNameKey)
		var pageToken *string = nil
	pageLoop:
		for {
			response, err := Client.ListBatchesWithResponse(context.Background(), &api.ListBatchesParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list batches:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list batches", response.HTTPResponse)
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
	var batchID uuid.UUID
	var err error
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
			response, err := Client.ListBatchesWithResponse(context.Background(), &api.ListBatchesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list batches:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list batches", response.HTTPResponse)
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
		response, err := Client.ListJobsWithResponse(context.Background(), batchID, &api.ListJobsParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("unable to list jobs:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list jobs", response.HTTPResponse)
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
