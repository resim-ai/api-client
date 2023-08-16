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
)

const (
	buildIDKey          = "build-id"
	experienceIDsKey    = "experience-ids"
	experienceTagIDsKey = "experience-tag-ids"

	batchIDKey    = "batch-id"
	batchNameKey  = "batch-name"
	exitStatusKey = "exit-status"
)

func init() {
	createBatchCmd.Flags().String(buildIDKey, "", "The ID of the build.")
	createBatchCmd.MarkFlagRequired(buildIDKey)
	createBatchCmd.Flags().String(experienceIDsKey, "", "Comma-separated list of experience ids to run.")
	createBatchCmd.Flags().String(experienceTagIDsKey, "", "Comma-separated list of experience tag ids to run.")
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

	rootCmd.AddCommand(batchCmd)
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
	experienceIDs := parseUUIDs(viper.GetString(experienceIDsKey))
	experienceTagIDs := parseUUIDs(viper.GetString(experienceTagIDsKey))

	// Build the request body and make the request
	body := api.CreateBatchJSONRequestBody{
		BuildID:          &buildID,
		ExperienceIDs:    &experienceIDs,
		ExperienceTagIDs: &experienceTagIDs,
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
