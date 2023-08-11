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

	buildIDString          string
	experienceIDsString    string
	experienceTagIDsString string

	batchIDString string
	batchName     string
	exitStatus    bool
)

func init() {
	createBatchCmd.Flags().StringVar(&buildIDString, "build_id", "", "The ID of the build.")
	createBatchCmd.Flags().StringVar(&experienceIDsString, "experience_ids", "", "Comma-separated list of experience ids to run.")
	createBatchCmd.Flags().StringVar(&experienceTagIDsString, "experience_tag_ids", "", "Comma-separated list of experience tag ids to run.")
	viper.BindPFlags(createBatchCmd.Flags())
	batchCmd.AddCommand(createBatchCmd)

	getBatchCmd.Flags().StringVar(&batchIDString, "batch_id", "", "The ID of the batch to retrieve.")
	getBatchCmd.Flags().StringVar(&batchName, "batch_name", "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish).")
	getBatchCmd.Flags().BoolVar(&exitStatus, "exit_status", false, "If set, exit code corresponds to batch status (1 = error, 0 = SUCCEEDED, 2=FAILED, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	viper.BindPFlags(getBatchCmd.Flags())
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
	buildID, err := uuid.Parse(buildIDString)
	if err != nil || buildID == uuid.Nil {
		log.Fatal(err)
	}
	experienceIDs := parseUUIDs(experienceIDsString)
	experienceTagIDs := parseUUIDs(experienceTagIDsString)

	// Build the request body and make the request
	body := api.CreateBatchJSONRequestBody{
		BuildID:          &buildID,
		ExperienceIDs:    &experienceIDs,
		ExperienceTagIDs: &experienceTagIDs,
	}

	response, err := client.CreateBatchWithResponse(context.Background(), body)
	if err != nil || response.HTTPResponse.StatusCode != http.StatusCreated {
		log.Fatal("failed to create batch: ", err, string(response.Body))
	}

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
	if batchIDString != "" {
		batchID, err := uuid.Parse(batchIDString)
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
		response, err := client.GetBatchWithResponse(context.Background(), batchID)
		if err != nil || response.StatusCode() != http.StatusOK {
			log.Fatal("unable to retrieve batch: ", err, string(response.Body))
		}
		batch = response.JSON200
	} else if batchName != "" {
		var pageToken *string = nil
	callLoop:
		for {
			response, err := client.ListBatchesWithResponse(context.Background(), &api.ListBatchesParams{
				PageToken: pageToken,
			})
			if err != nil || response.StatusCode() != 200 {
				log.Fatal("unable to list batches: ", err, string(response.Body))
			}
			if response.JSON200.Batches == nil {
				log.Fatal("unable to find batch: ", batchName)
			}
			batches := *response.JSON200.Batches

			for _, b := range batches {
				if b.FriendlyName != nil && *b.FriendlyName == batchName {
					batch = &b
					break callLoop
				}
			}

			if *response.JSON200.NextPageToken != "" {
				pageToken = response.JSON200.NextPageToken
			} else {
				log.Fatal("unable to find batch: ", batchName)
			}
		}
	} else {
		log.Fatal("must specify either the batch ID or the batch name")
	}

	if exitStatus {
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
