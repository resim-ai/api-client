package commands

import (
	"context"
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

	listLogsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists the logs for a batch",
		Long:  ``,
		Run:   listLogs,
	}
)

const (
	logProjectKey = "project"
	logBatchIDKey = "batch-id"
	logJobIDKey   = "test-id" // User-facing is test ID, internal is job id
)

func init() {
	listLogsCmd.Flags().String(logProjectKey, "", "The name or ID of the project to list logs with")
	listLogsCmd.MarkFlagRequired(logProjectKey)
	listLogsCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch the logs are associated with")
	listLogsCmd.MarkFlagRequired(logBatchIDKey)
	listLogsCmd.Flags().String(logJobIDKey, "", "The UUID of the test in the batch to list logs for")
	listLogsCmd.MarkFlagRequired(logJobIDKey)
	listLogsCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	logsCmd.AddCommand(listLogsCmd)

	rootCmd.AddCommand(logsCmd)
}

func listLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(logProjectKey))
	batchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || batchID == uuid.Nil {
		log.Fatal("unable to parse batch ID: ", err)
	}

	testID, err := uuid.Parse(viper.GetString(logJobIDKey))
	if err != nil || testID == uuid.Nil {
		log.Fatal("unable to parse test ID: ", err)
	}

	logs := []api.JobLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListJobLogsForJobWithResponse(context.Background(), projectID, batchID, testID, &api.ListJobLogsForJobParams{
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
