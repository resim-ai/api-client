package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logCmd = &cobra.Command{
		Use:   "log",
		Short: "log contains commands for creating and listing test logs. This is not expected to be used directly by users, but via CI/CD systems.",
		Long:  ``,
	}
	createLogCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new log entry",
		Long:   ``,
		Run:    createLog,
		PreRun: RegisterViperFlags,
	}
)

const (
	logNameKey     = "name"
	logBatchIDKey  = "batch-id"
	logJobIDKey    = "job-id"
	logFileSizeKey = "file-size"
	logChecksumKey = "checksum"
	logGithubKey   = "github"
)

func init() {
	createLogCmd.Flags().String(logNameKey, "", "The simple name of the log file to register (not a directory)")
	createLogCmd.MarkFlagRequired(logNameKey)
	createLogCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch this log file is associated with")
	createLogCmd.MarkFlagRequired(logBatchIDKey)
	createLogCmd.Flags().String(logJobIDKey, "", "The UUID of the job in the batch this log file was created by and will be associated with")
	createLogCmd.MarkFlagRequired(logJobIDKey)
	createLogCmd.Flags().Int64(logFileSizeKey, -1, "The size of the file in bytes")
	createLogCmd.MarkFlagRequired(logFileSizeKey)
	createLogCmd.Flags().String(logChecksumKey, "", "A checksum for the file, to enable integrity checking when downloading")
	createLogCmd.MarkFlagRequired(logChecksumKey)
	createLogCmd.Flags().Bool(logGithubKey, false, "Whether to output format in github action friendly format")
	logCmd.AddCommand(createLogCmd)
	rootCmd.AddCommand(logCmd)
}

func createLog(ccmd *cobra.Command, args []string) {
	logGithub := viper.GetBool(logGithubKey)
	if !logGithub {
		fmt.Println("Creating a log entry...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	logName := viper.GetString(logNameKey)
	if logName == "" {
		log.Fatal("empty log filename")
	}

	logBatchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || logBatchID == uuid.Nil {
		log.Fatal("empty batch ID")
	}

	logJobID, err := uuid.Parse(viper.GetString(logJobIDKey))
	if err != nil || logJobID == uuid.Nil {
		log.Fatal("empty log ID")
	}

	logFileSize := viper.GetInt64(logFileSizeKey)
	if logFileSize == -1 {
		log.Fatal("empty file size")
	}

	logChecksum := viper.GetString(logChecksumKey)
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
	ValidateResponse(http.StatusOK, fmt.Sprintf("unabled to find batch with ID %v", logBatchID),
		batchResponse.HTTPResponse, err)

	jobResponse, err := client.GetJobWithResponse(context.Background(), logBatchID, logJobID)
	ValidateResponse(http.StatusOK, fmt.Sprintf("unabled to find job with ID %v", logJobID),
		jobResponse.HTTPResponse, err)

	// Create the log entry
	logResponse, err := client.CreateLogWithResponse(context.Background(), logBatchID, logJobID, body)
	ValidateResponse(http.StatusCreated, "unable to create log", jobResponse.HTTPResponse, err)
	if logResponse.JSON201 == nil {
		log.Fatal("empty response")
	}
	myLog := logResponse.JSON201
	if myLog.Location == nil {
		log.Fatal("empty location")
	}
	if myLog.LogID == nil {
		log.Fatal("empty log ID")
	}

	// Report the results back to the user
	if logGithub {
		fmt.Printf("log_location=%s\n", *myLog.Location)
	} else {
		fmt.Println("Created log successfully!")
		fmt.Printf("Log ID: %s\n", myLog.LogID.String())
		fmt.Printf("Output Location: %s\n", *myLog.Location)
		fmt.Println("Please upload the log file to this location")
	}
}
