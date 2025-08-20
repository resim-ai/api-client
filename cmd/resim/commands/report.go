package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	reportCmd = &cobra.Command{
		Use:     "reports",
		Short:   "reports contains commands for creating and managing reports",
		Long:    ``,
		Aliases: []string{"report"},
	}

	createReportCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new report",
		Long:  ``,
		Run:   createReport,
	}

	getReportCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves a report",
		Long:  ``,
		Run:   getReport,
	}

	waitReportCmd = &cobra.Command{
		Use:   "wait",
		Short: "wait - Wait for report completion",
		Long:  `Awaits report completion and returns an exit code corresponding to the report status. 1 = internal error, 0 = SUCCEEDED, 2=ERROR, 5=CANCELLED, 6=timed out)`,
		Run:   waitReport,
	}

	logsReportCmd = &cobra.Command{
		Use:   "logs",
		Short: "logs - Lists the logs associated with a report",
		Long:  ``,
		Run:   listReportLogs,
	}
)

const (
	reportProjectKey                  = "project"
	reportTestSuiteKey                = "test-suite"
	reportTestSuiteRevisionKey        = "test-suite-revision"
	reportBranchKey                   = "branch"
	reportLengthKey                   = "length"
	reportStartTimestampKey           = "start-timestamp"
	reportEndTimestampKey             = "end-timestamp"
	reportRespectTestSuiteRevisionKey = "respect-revision-boundary"
	reportMetricsBuildIDKey           = "metrics-build-id"
	reportMetricsSetKey               = "metrics-set"
	reportIDKey                       = "report-id"
	reportNameKey                     = "report-name"
	reportAccountKey                  = "account"
	reportGithubKey                   = "github"
	reportExitStatusKey               = "exit-status"
	reportWaitTimeoutKey              = "wait-timeout"
	reportWaitPollKey                 = "poll-every"
)

func init() {
	createReportCmd.Flags().Bool(reportGithubKey, false, "Whether to output format in github action friendly format.")
	createReportCmd.Flags().String(reportProjectKey, "", "The name or ID of the project to associate with the report.")
	createReportCmd.MarkFlagRequired(reportProjectKey)
	// Optional Report Name
	createReportCmd.Flags().String(reportNameKey, "", "The name to associate with the report. If not supplied, a name will be generated.")
	// Test Suite Name or ID
	createReportCmd.Flags().String(reportTestSuiteKey, "", "The name or ID of the test suite.")
	createReportCmd.MarkFlagRequired(reportTestSuiteKey)
	// Test Suite Revision
	createReportCmd.Flags().Int32(reportTestSuiteRevisionKey, 0, "The revision of the test suite. If not supplied, the latest revision will be used.")
	// Respect Test Suite Revision
	createReportCmd.Flags().Bool(reportRespectTestSuiteRevisionKey, false, "Pass this flag to indicate that we only want to generate the report based on batches from the supplied test suite revision.")
	// Branch Name or ID
	createReportCmd.Flags().String(reportBranchKey, "", "The name or ID of the branch to generate the report on.")
	createReportCmd.MarkFlagRequired(reportBranchKey)
	// Metrics Build
	createReportCmd.Flags().String(reportMetricsBuildIDKey, "", "The ID of the metrics build to use in this report.")
	createReportCmd.MarkFlagRequired(reportMetricsBuildIDKey)
	// Metrics Set
	createReportCmd.Flags().String(reportMetricsSetKey, "", "The name of the metrics set to use in this report.")
	// Length, Start, End Timestamps
	createReportCmd.Flags().Int(reportLengthKey, 28, "The length of the report in days, from now. Cannot be used in combination with start and end timestamps. For a more precise report, use the start and end timestamps")
	createReportCmd.Flags().String(reportStartTimestampKey, time.Now().UTC().String(), "The start timestamp of the report (in a Golang parsable format using RFC3339). Cannot be used in combination with length.")
	createReportCmd.MarkFlagsOneRequired(reportLengthKey, reportStartTimestampKey)
	createReportCmd.Flags().String(reportEndTimestampKey, time.Now().UTC().String(), "The end timestamp of the report (in a Golang parsable format using RFC3339). If not supplied, the current time will be used.")
	createReportCmd.MarkFlagsMutuallyExclusive(reportLengthKey, reportEndTimestampKey)
	// Account
	createReportCmd.Flags().String(reportAccountKey, "", "Specify a username for a CI/CD platform account to associate with this test report.")
	reportCmd.AddCommand(createReportCmd)

	// Get Report Fields
	getReportCmd.Flags().String(reportProjectKey, "", "The name or ID of the project the report is associated with.")
	getReportCmd.MarkFlagRequired(reportProjectKey)
	getReportCmd.Flags().String(reportIDKey, "", "The ID of the report to retrieve.")
	getReportCmd.Flags().String(reportNameKey, "", "The name of the report to retrieve (e.g. rejoicing-aquamarine-starfish). If multiple reports exist with this name, then the most recent is fetched.")
	getReportCmd.MarkFlagsMutuallyExclusive(reportIDKey, reportNameKey)
	getReportCmd.Flags().Bool(reportExitStatusKey, false, "If set, exit code corresponds to report status (1 = internal CLI error, 0 = SUCCEEDED, 2=ERROR, 3=SUBMITTED, 4=RUNNING)")
	reportCmd.AddCommand(getReportCmd)

	// Await Report Fields
	waitReportCmd.Flags().String(reportProjectKey, "", "The name or ID of the project the report is associated with")
	waitReportCmd.MarkFlagRequired(reportProjectKey)
	waitReportCmd.Flags().String(reportIDKey, "", "The ID of the report to await completion.")
	waitReportCmd.Flags().String(reportNameKey, "", "The name of the report to await completion (e.g. rejoicing-aquamarine-starfish).  If multiple reports exist with this name, then the most recent is used.")
	waitReportCmd.MarkFlagsMutuallyExclusive(reportIDKey, reportNameKey)
	waitReportCmd.Flags().String(reportWaitTimeoutKey, "1h", "Amount of time to wait for a report to finish, expressed in Golang duration string.")
	waitReportCmd.Flags().String(reportWaitPollKey, "30s", "Interval between checking report status, expressed in Golang duration string.")
	reportCmd.AddCommand(waitReportCmd)

	// Logs for Report Fields
	logsReportCmd.Flags().String(reportProjectKey, "", "The name or ID of the project the report is associated with")
	logsReportCmd.MarkFlagRequired(reportProjectKey)
	logsReportCmd.Flags().String(reportIDKey, "", "The ID of the report to list logs for.")
	logsReportCmd.Flags().String(reportNameKey, "", "The name of the report to list logs for (e.g. rejoicing-aquamarine-starfish).  If multiple reports exist with this name, then the most recent is used.")
	logsReportCmd.MarkFlagsMutuallyExclusive(reportIDKey, reportNameKey)
	logsReportCmd.MarkFlagsOneRequired(reportIDKey, reportNameKey)
	reportCmd.AddCommand(logsReportCmd)

	rootCmd.AddCommand(reportCmd)
}

func createReport(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(reportProjectKey))
	reportGithub := viper.GetBool(reportGithubKey)
	if !reportGithub {
		fmt.Println("Creating a report...")
	}

	// Get the test suite:
	var revision *int32
	if viper.IsSet(reportTestSuiteRevisionKey) {
		revision = Ptr(viper.GetInt32(reportTestSuiteRevisionKey))
	}
	testSuite := actualGetTestSuite(projectID, viper.GetString(reportTestSuiteKey), revision, false)
	// Get the branch:
	branchID := getBranchID(Client, projectID, viper.GetString(reportBranchKey), true)
	// Get the metrics build:
	metricsBuildID, err := uuid.Parse(viper.GetString(reportMetricsBuildIDKey))
	if err != nil {
		log.Fatal("failed to parse metrics-build ID: ", err)
	}

	// Get the start and end timestamps or duration:
	var startTimestamp time.Time
	var endTimestamp time.Time
	if viper.IsSet(reportEndTimestampKey) {
		endTimestamp, err = time.Parse(time.RFC3339, viper.GetString(reportEndTimestampKey))
		if err != nil {
			log.Fatal("failed to parse end timestamp as timestamp with REF3339: ", err)
		}
	} else {
		endTimestamp = time.Now().UTC()
		if !reportGithub {
			fmt.Println("End timestamp:", endTimestamp)
		}
	}

	if viper.IsSet(reportStartTimestampKey) {
		startTimestamp, err = time.Parse(time.RFC3339, viper.GetString(reportStartTimestampKey))
		if err != nil {
			log.Fatal("failed to parse start timestamp as timestamp with REF3339: ", err)
		}
	} else {
		// Turn the length into a duration
		numberDays := viper.GetInt(reportLengthKey)
		length := time.Duration(numberDays) * 24 * time.Hour
		startTimestamp = endTimestamp.Add(-length)
		if !reportGithub {
			fmt.Println("Start timestamp calculated as:", startTimestamp)
		}
	}

	// Process the associated account: by default, we try to get from CI/CD environment variables
	// Otherwise, we use the account flag. The default is "".
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(reportAccountKey) {
		associatedAccount = viper.GetString(reportAccountKey)
	}

	var metricsSet *string
	poolLabels := api.PoolLabels{}
	if viper.IsSet(reportMetricsSetKey) {
		metricsSet = Ptr(viper.GetString(reportMetricsSetKey))
		// Metrics 2.0 steps will only be run if we use the special pool
		// label, so let's enable it automatically if the user requested a
		// metrics set
		poolLabels = append(poolLabels, METRICS_2_POOL_LABEL)
	}

	// Build the request body
	body := api.ReportInput{
		TestSuiteID:             testSuite.TestSuiteID,
		BranchID:                branchID,
		RespectRevisionBoundary: Ptr(viper.GetBool(reportRespectTestSuiteRevisionKey)),
		MetricsBuildID:          metricsBuildID,
		StartTimestamp:          startTimestamp,
		EndTimestamp:            Ptr(endTimestamp),
		AssociatedAccount:       &associatedAccount,
		TriggeredVia:            DetermineTriggerMethod(),
		MetricsSetName:          metricsSet,
	}

	if viper.IsSet(reportNameKey) {
		body.Name = Ptr(viper.GetString(reportNameKey))
	}

	if viper.IsSet(reportTestSuiteRevisionKey) {
		body.TestSuiteRevision = Ptr(viper.GetInt32(reportTestSuiteRevisionKey))
	}

	if len(poolLabels) > 0 {
		body.PoolLabels = &poolLabels
	}

	// Make the request
	response, err := Client.CreateReportWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create report:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create report", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	report := *response.JSON201

	if !reportGithub {
		// Report the results back to the user
		fmt.Println("Created report successfully!")
	}
	if report.ReportID == uuid.Nil {
		log.Fatal("empty ID")
	}
	if !reportGithub {
		fmt.Println("Report ID:", report.ReportID.String())
	} else {
		fmt.Printf("report_id=%s\n", report.ReportID.String())
	}
	if !reportGithub {
		fmt.Println("Report name:", report.Name)
	}
	if !reportGithub {
		fmt.Println("Status:", report.Status)
	}
}

func actualGetReport(projectID uuid.UUID, reportIDRaw string, reportName string) *api.Report {
	var report *api.Report
	if reportIDRaw != "" {
		reportID, err := uuid.Parse(reportIDRaw)
		if err != nil {
			log.Fatal("unable to parse report ID: ", err)
		}
		response, err := Client.GetReportWithResponse(context.Background(), projectID, reportID)
		if err != nil {
			log.Fatal("unable to retrieve report:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve report", response.HTTPResponse, response.Body)
		report = response.JSON200
		return report
	} else if reportName != "" {
		var pageToken *string = nil
		for {
			response, err := Client.ListReportsWithResponse(context.Background(), projectID, &api.ListReportsParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list reports:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list reports", response.HTTPResponse, response.Body)
			if response.JSON200.Reports == nil {
				log.Fatal("unable to find report: ", reportName)
			}
			reports := *response.JSON200.Reports

			for _, r := range reports {
				if r.Name == reportName {
					report = &r
					return report
				}
			}

			if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
				pageToken = response.JSON200.NextPageToken
			} else {
				log.Fatal("unable to find report: ", reportName)
			}
		}
	} else {
		log.Fatal("must specify either the report ID or the report name")
	}
	return report
}

func getReport(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(reportProjectKey))
	report := actualGetReport(projectID, viper.GetString(reportIDKey), viper.GetString(reportNameKey))

	if viper.GetBool(reportExitStatusKey) {
		switch report.Status {
		case api.ReportStatusSUCCEEDED:
			os.Exit(0)
		case api.ReportStatusERROR:
			os.Exit(2)
		case api.ReportStatusSUBMITTED:
			os.Exit(3)
		case api.ReportStatusRUNNING:
			os.Exit(4)
		default:
			log.Fatal("unknown report status: ", report.Status)
		}
	}

	OutputJson(report)
}

func waitReport(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(reportProjectKey))
	var report *api.Report
	timeout, _ := time.ParseDuration(viper.GetString(reportWaitTimeoutKey))
	pollWait, _ := time.ParseDuration(viper.GetString(reportWaitPollKey))
	startTime := time.Now()
	for {
		report = actualGetReport(projectID, viper.GetString(reportIDKey), viper.GetString(reportNameKey))
		viper.Set(reportIDKey, report.ReportID.String())
		switch report.Status {
		case api.ReportStatusSUCCEEDED:
			os.Exit(0)
		case api.ReportStatusERROR:
			os.Exit(2)
		case api.ReportStatusSUBMITTED, api.ReportStatusRUNNING:
		default:
			log.Fatal("unknown report status: ", report.Status)
		}

		if time.Now().After(startTime.Add(timeout)) {
			log.Fatalf("Failed to reach a final state after %v, last state %s", timeout, report.Status)
			os.Exit(6)
		}
		time.Sleep(pollWait)
	}
}

func listReportLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(reportProjectKey))
	var reportID uuid.UUID
	var err error
	if viper.IsSet(reportIDKey) {
		reportID, err = uuid.Parse(viper.GetString(reportIDKey))
		if err != nil {
			log.Fatal("unable to parse report ID: ", err)
		}
	} else {
		report := actualGetReport(projectID, "", viper.GetString(reportNameKey))
		reportID = report.ReportID
	}
	logs := []api.ReportLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListLogsForReportWithResponse(context.Background(), projectID, reportID, &api.ListLogsForReportParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("unable to list logs:", err)
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
