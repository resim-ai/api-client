package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
)

var (
	logCmd = &cobra.Command{
		Use:   "log",
		Short: "log contains commands for creating and listing test logs. This is not expected to be used directly by users, but via CI/CD systems.",
		Long:  ``,
	}
	createLogCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new log entry",
		Long:  ``,
		Run:   createLog,
	}

	logName          string
	logFileSize      int64
	logBatchIDString string
	logJobIDString   string
	logChecksum      string
	logGithub        bool
)

func init() {
	createLogCmd.Flags().StringVar(&logName, "name", "", "The simple name of the log file to register (not a directory)")
	createLogCmd.Flags().StringVar(&logBatchIDString, "batch_id", "", "The UUID of the batch this log file is associated with")
	createLogCmd.Flags().StringVar(&logJobIDString, "job_id", "", "The UUID of the job in the batch this log file was created by and will be associated with")
	createLogCmd.Flags().Int64Var(&logFileSize, "file_size", -1, "The size of the file in bytes")
	createLogCmd.Flags().StringVar(&logChecksum, "checksum", "", "A checksum for the file, to enable integrity checking when downloading")
	createLogCmd.Flags().BoolVar(&logGithub, "github", false, "Whether to output format in github action friendly format")
	logCmd.AddCommand(createLogCmd)
	rootCmd.AddCommand(logCmd)
}

func createLog(ccmd *cobra.Command, args []string) {
	if !logGithub {
		fmt.Println("Creating a log entry...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	if logName == "" {
		log.Fatal("empty log filename")
	}

	logBatchID, err := uuid.Parse(logBatchIDString)
	if err != nil || logBatchID == uuid.Nil {
		log.Fatal("empty batch ID")
	}

	logJobID, err := uuid.Parse(logJobIDString)
	if err != nil || logJobID == uuid.Nil {
		log.Fatal("empty log ID")
	}

	if logFileSize == -1 {
		log.Fatal("empty file size")
	}

	if logChecksum == "" {
		if !logGithub {
			fmt.Println("No checksum was provided, integrity checking will not be possible")
		}
	}

	body := api.CreateLogJSONRequestBody{
		FileName: &logName,
		FileSize: &logFileSize,
		Checksum: &logChecksum,
	}

	// Verify that the batch and job exist:
	batchResponse, err := client.GetBatchWithResponse(context.Background(), logBatchID)
	if err != nil || batchResponse.StatusCode() != http.StatusOK {
		log.Fatal("unable to find batch with ID ", logBatchID, err, string(batchResponse.Body))
	}

	jobResponse, err := client.GetJobWithResponse(context.Background(), logBatchID, logJobID)
	if err != nil || jobResponse.StatusCode() != http.StatusOK {
		log.Fatal("unable to find job with ID ", logJobID, err, string(jobResponse.Body))
	}

	// Create the log entry
	logResponse, err := client.CreateLogWithResponse(context.Background(), logBatchID, logJobID, body)
	if err != nil || logResponse.StatusCode() != http.StatusCreated {
		log.Fatal("unable to create log ", err, string(logResponse.Body))
	}

	// Report the results back to the user
	if logGithub {
		fmt.Printf("log_location=%s\n", *logResponse.JSON201.Location)
	} else {
		fmt.Println("Created log successfully!")
		fmt.Printf("Log ID: %s\n", logResponse.JSON201.LogID.String())
		fmt.Printf("Output Location: %s\n", *logResponse.JSON201.Location)
		fmt.Println("Please upload the log file to this location")
	}
}
