package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logsCmd = &cobra.Command{
		Use:     "logs",
		Short:   "logs contains commands for creating and listing test logs. This is not expected to be used directly by users, but via CI/CD systems.",
		Long:    ``,
		Aliases: []string{"log"},
	}
	createLogCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new log entry",
		Long:  ``,
		Run:   createLog,
	}
	listLogsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists the logs for a batch",
		Long:  ``,
		Run:   listLogs,
	}
)

const (
	logProjectKey       = "project"
	logNameKey          = "name"
	logBatchIDKey       = "batch-id"
	logJobIDKey         = "job-id"
	logFileSizeKey      = "file-size"
	logChecksumKey      = "checksum"
	logExecutionStepKey = "execution-step"
	logTypeKey          = "type"
	logGithubKey        = "github"
)

func init() {
	createLogCmd.Flags().String(logProjectKey, "", "The name or ID of the project to associate the log with")
	createLogCmd.MarkFlagRequired(logProjectKey)
	createLogCmd.Flags().String(logNameKey, "", "The simple name of the log file to register (not a directory)")
	createLogCmd.MarkFlagRequired(logNameKey)
	createLogCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch this log file is associated with")
	createLogCmd.MarkFlagRequired(logBatchIDKey)
	createLogCmd.Flags().String(logJobIDKey, "", "The UUID of the job in the batch this log file was created by and will be associated with")
	createLogCmd.MarkFlagRequired(logJobIDKey)
	createLogCmd.Flags().Int64(logFileSizeKey, -1, "The size of the file in bytes")
	createLogCmd.MarkFlagRequired(logFileSizeKey)
	createLogCmd.Flags().String(logChecksumKey, "", "A checksum for the file, to enable integrity checking when downloading")
	createLogCmd.Flags().String(logExecutionStepKey, "EXPERIENCE", "The execution step to register the log: EXPERIENCE, METRICS. BATCH_METRICS is not currently supported.")
	createLogCmd.MarkFlagRequired(logExecutionStepKey)
	createLogCmd.Flags().String(logTypeKey, "", "The type of the log: ARCHIVE_LOG, CONTAINER_LOG, EXECUTION_LOG, MCAP_LOG, METRICS_OUTPUT_LOG, MP4_LOG, OTHER_LOG")
	createLogCmd.MarkFlagRequired(logTypeKey)
	createLogCmd.Flags().Bool(logGithubKey, false, "Whether to output format in github action friendly format")
	logsCmd.AddCommand(createLogCmd)

	listLogsCmd.Flags().String(logProjectKey, "", "The name or ID of the project to list logs with")
	listLogsCmd.MarkFlagRequired(logProjectKey)
	listLogsCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch the logs are associated with")
	listLogsCmd.MarkFlagRequired(logBatchIDKey)
	listLogsCmd.Flags().String(logJobIDKey, "", "The UUID of the job in the batch to list logs for")
	listLogsCmd.MarkFlagRequired(logJobIDKey)
	logsCmd.AddCommand(listLogsCmd)

	rootCmd.AddCommand(logsCmd)
}

func createLog(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(logProjectKey))
	logGithub := viper.GetBool(logGithubKey)
	if !logGithub {
		fmt.Println("Creating a log entry...")
	}

	// Parse the various arguments from command line
	logName := viper.GetString(logNameKey)
	if logName == "" {
		log.Fatal("empty log file name")
	}

	logBatchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || logBatchID == uuid.Nil {
		log.Fatal("empty batch ID")
	}

	logJobID, err := uuid.Parse(viper.GetString(logJobIDKey))
	if err != nil || logJobID == uuid.Nil {
		log.Fatal("empty job ID")
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

	logType := api.LogType(viper.GetString(logTypeKey))
	if logType != api.MP4LOG && logType != api.ARCHIVELOG && logType != api.CONTAINERLOG && logType != api.EXECUTIONLOG && logType != api.MCAPLOG && logType != api.METRICSOUTPUTLOG && logType != api.OTHERLOG {
		log.Fatal("invalid log type")
	}

	logExecutionStep := api.ExecutionStep(viper.GetString(logExecutionStepKey))
	if logExecutionStep != api.EXPERIENCE && logExecutionStep != api.METRICS {
		log.Fatal("invalid execution step")
	}

	body := api.JobLog{
		FileName:      &logName,
		FileSize:      &logFileSize,
		Checksum:      &logChecksum,
		LogType:       &logType,
		ExecutionStep: &logExecutionStep,
	}

	// Verify that the batch and job exist:
	batchResponse, err := Client.GetBatchWithResponse(context.Background(), projectID, logBatchID)
	if err != nil {
		log.Fatal("unable to get batch: ", err)
	}
	ValidateResponse(http.StatusOK, fmt.Sprintf("unable to find batch with ID %v", logBatchID),
		batchResponse.HTTPResponse, batchResponse.Body)

	jobResponse, err := Client.GetJobWithResponse(context.Background(), projectID, logBatchID, logJobID)
	if err != nil {
		log.Fatal("unable to get job: ", err)
	}
	ValidateResponse(http.StatusOK, fmt.Sprintf("unable to find job with ID %v", logJobID),
		jobResponse.HTTPResponse, jobResponse.Body)

	// Create the log entry
	logResponse, err := Client.CreateJobLogWithResponse(context.Background(), projectID, logBatchID, logJobID, body)
	if err != nil {
		log.Fatal("unable to create log: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create log", logResponse.HTTPResponse, logResponse.Body)
	if logResponse.JSON201 == nil {
		log.Fatal("empty response")
	}
	myLog := logResponse.JSON201
	if myLog.Location == nil || *myLog.Location == "" {
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

func listLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(logProjectKey))
	batchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || batchID == uuid.Nil {
		log.Fatal("unable to parse batch ID: ", err)
	}

	jobID, err := uuid.Parse(viper.GetString(logJobIDKey))
	if err != nil || jobID == uuid.Nil {
		log.Fatal("unable to parse job ID: ", err)
	}

	logs := []api.JobLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListJobLogsForJobWithResponse(context.Background(), projectID, batchID, jobID, &api.ListJobLogsForJobParams{
			PageToken: pageToken,
			PageSize:  Ptr(100),
		})
		if err != nil {
			log.Fatal("unable to list logs: ", err)
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
