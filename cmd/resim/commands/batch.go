package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
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
)

// This function takes a comma-separated list of UUIDs represented as strings
// and returns a separated array of parsed UUIDs.
func parseUUIDs(commaSeparatedUUIDs string) []openapi_types.UUID {
	if commaSeparatedUUIDs == "" {
		return []openapi_types.UUID{}
	}
	strs := strings.Split(commaSeparatedUUIDs, ",")
	result := make([]openapi_types.UUID, len(strs))

	for i := 0; i < len(strs); i++ {
		id, err := uuid.Parse(strings.TrimSpace(strs[i]))
		if err != nil {
			log.Fatal(err)
		}
		result[i] = id
	}
	return result
}

func init() {
	createBatchCmd.Flags().String("build_id", "", "The ID of the build.")
	createBatchCmd.Flags().String("experience_ids", "", "Comma-separated list of experience ids to run.")
	createBatchCmd.Flags().String("experience_tag_ids", "", "Comma-separated list of experience tag ids to run.")
	viper.BindPFlags(createBatchCmd.Flags())
	batchCmd.AddCommand(createBatchCmd)
	rootCmd.AddCommand(batchCmd)
}

func createBatch(ccmd *cobra.Command, args []string) {
	fmt.Println("Creating a batch...")

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the UUIDs from the command line
	buildId, err := uuid.Parse(viper.GetString("build_id"))
	if err != nil || buildId == uuid.Nil {
		log.Fatal(err)
	}
	experienceIds := parseUUIDs(viper.GetString("experience_ids"))
	experienceTagIds := parseUUIDs(viper.GetString("experience_tag_ids"))

	// Build the request body and make the request
	body := api.CreateBatchJSONRequestBody{
		BuildID:          &buildId,
		ExperienceIDs:    &experienceIds,
		ExperienceTagIDs: &experienceTagIds,
	}

	response, err := client.CreateBatchWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal(err)
	}

	// Report the results back to the user
	success := response.HTTPResponse.StatusCode == http.StatusCreated
	if success {
		fmt.Println("Created Batch Successfully!")
		fmt.Printf("Batch ID: %s\n", response.JSON201.BatchID.String())
		fmt.Printf("Status: %s\n", *response.JSON201.Status)
	} else {
		log.Fatal("Failed to create batch!\n", string(response.Body))
	}

}
