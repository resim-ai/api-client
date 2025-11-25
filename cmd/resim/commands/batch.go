package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/slack-go/slack"
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

	cancelBatchCmd = &cobra.Command{
		Use:   "cancel",
		Short: "cancel - Cancel a batch",
		Long:  ``,
		Run:   cancelBatch,
	}

	testsBatchCmd = &cobra.Command{
		Use:     "tests",
		Short:   "tests - Lists the tests in a batch",
		Long:    ``,
		Run:     testsBatch,
		Aliases: []string{"jobs"},
	}

	waitBatchCmd = &cobra.Command{
		Use:   "wait",
		Short: "wait - Wait for batch completion",
		Long:  `Awaits batch completion and returns an exit code corresponding to the batch status. 1 = internal error, 0 = SUCCEEDED, 2=ERROR, 5=CANCELLED, 6=timed out)`,
		Run:   waitBatch,
	}

	logsBatchCmd = &cobra.Command{
		Use:   "logs",
		Short: "logs - Lists the logs associated with a batch",
		Long:  ``,
		Run:   listBatchLogs,
	}

	rerunBatchCmd = &cobra.Command{
		Use:   "rerun",
		Short: "rerun - Reruns a subset of tests in a batch",
		Long:  ``,
		Run:   rerunBatch,
	}

	superviseBatchCmd = &cobra.Command{
		Use:   "supervise",
		Short: "supervise - waits on batch completion and reruns failed tests",
		Long:  `Awaits batch completion with rerun capability and returns an exit code corresponding to the final batch status. 1 = internal error, 0 = SUCCEEDED, 2=ERROR, 5=CANCELLED, 6=timed out)`,
		Run:   superviseBatch,
	}
)

const (
	batchProjectKey                 = "project"
	batchBuildIDKey                 = "build-id"
	batchExperienceIDsKey           = "experience-ids"
	batchExperiencesKey             = "experiences"
	batchExperienceTagIDsKey        = "experience-tag-ids"
	batchExperienceTagNamesKey      = "experience-tag-names"
	batchExperienceTagsKey          = "experience-tags"
	batchParameterKey               = "parameter"
	batchPoolLabelsKey              = "pool-labels"
	batchIDKey                      = "batch-id"
	batchNameKey                    = "batch-name"
	batchAccountKey                 = "account"
	batchGithubKey                  = "github"
	batchMetricsBuildKey            = "metrics-build-id"
	batchMetricsSetKey              = "metrics-set"
	batchExitStatusKey              = "exit-status"
	batchWaitTimeoutKey             = "wait-timeout"
	batchWaitPollKey                = "poll-every"
	batchSlackOutputKey             = "slack"
	batchMarkdownOutputKey          = "markdown"
	batchSyncWithTestSuiteKey       = "sync-with-test-suite"
	batchAllowableFailurePercentKey = "allowable-failure-percent"
	batchTestIDsKey                 = "test-ids"
	batchMaxRerunAttemptsKey        = "max-rerun-attempts"
	batchRerunMaxFailurePercentKey  = "rerun-max-failure-percent"
	batchRerunOnStatesKey           = "rerun-on-states"
	batchSyncMetricsConfigKey       = "sync-metrics-config"
	batchMetricsConfigPath          = "metrics-config-path"
	batchMetricsTemplatesPath       = "metrics-templates-path"
)

func init() {
	createBatchCmd.Flags().Bool(batchGithubKey, false, "Whether to output format in github action friendly format")
	createBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project to associate with the batch")
	createBatchCmd.MarkFlagRequired(batchProjectKey)
	createBatchCmd.Flags().String(batchBuildIDKey, "", "The ID of the build.")
	createBatchCmd.MarkFlagRequired(batchBuildIDKey)
	createBatchCmd.Flags().String(batchMetricsBuildKey, "", "The ID of the metrics build to use in this batch.")
	// the separate ID and name flags for experiences and experience tags are kept for backwards compatibility
	createBatchCmd.Flags().String(batchExperienceIDsKey, "", "Comma-separated list of experience IDs to run.")
	createBatchCmd.Flags().String(batchExperiencesKey, "", "List of experience names or list of experience IDs to run, comma-separated")
	createBatchCmd.Flags().String(batchExperienceTagIDsKey, "", "Comma-separated list of experience tag IDs to run.")
	createBatchCmd.Flags().String(batchExperienceTagNamesKey, "", "Comma-separated list of experience tag names to run.")
	createBatchCmd.Flags().String(batchExperienceTagsKey, "", "List of experience tag names or list of experience tag IDs to run, comma-separated.")
	createBatchCmd.Flags().StringSlice(batchParameterKey, []string{}, "(Optional) Parameter overrides to pass to the build. Format: <parameter-name>=<parameter-value> or <parameter-name>:<parameter-value>. The equals sign (=) is recommended, especially if parameter names contain colons. Accepts repeated parameters or comma-separated parameters e.g. 'param1=value1,param2=value2'. If multiple = signs are used, the first one will be used to determine the key, and the rest will be part of as the value.")
	createBatchCmd.Flags().StringSlice(batchPoolLabelsKey, []string{}, "Pool labels to determine where to run this batch. Pool labels are interpreted as a logical AND. Accepts repeated labels or comma-separated labels.")
	createBatchCmd.MarkFlagsOneRequired(batchExperienceIDsKey, batchExperiencesKey, batchExperienceTagIDsKey, batchExperienceTagNamesKey, batchExperienceTagsKey)
	createBatchCmd.Flags().String(batchAccountKey, "", "Specify a username for a CI/CD platform account to associate with this test batch.")
	createBatchCmd.Flags().String(batchNameKey, "", "An optional name for the batch. If not supplied, ReSim generates a pseudo-unique name e.g rejoicing-aquamarine-starfish. This name need not be unique, but uniqueness is recommended to make it easier to identify batches.")
	createBatchCmd.Flags().Int(batchAllowableFailurePercentKey, 0, "An optional percentage (0-100) that determines the maximum percentage of tests that can have an execution error and have aggregate metrics be computed and consider the batch successfully completed. If not supplied, ReSim defaults to 0, which means that the batch will only be considered successful if all tests complete successfully.")
	createBatchCmd.Flags().String(batchMetricsSetKey, "", "The name of the metrics set to use to generate test and batch metrics")
	createBatchCmd.Flags().Bool(batchSyncMetricsConfigKey, false, "If set, run metrics sync before creating the batch")
	createBatchCmd.Flags().String(batchMetricsConfigPath, ".resim/metrics/config.yml", "The path to the metrics config file. Default is .resim/metrics/config.yml. Only used if sync-metrics-config is set to true")
	createBatchCmd.Flags().String(batchMetricsTemplatesPath, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates. Only used if sync-metrics-config is set to true")
	batchCmd.AddCommand(createBatchCmd)

	getBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	getBatchCmd.MarkFlagRequired(batchProjectKey)
	getBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve.")
	getBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this returns the most recent batch with that name.")
	getBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	getBatchCmd.Flags().Bool(batchExitStatusKey, false, "If set, exit code corresponds to batch workflow status (1 = internal CLI error, 0 = SUCCEEDED, 2=ERROR, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	getBatchCmd.Flags().Bool(batchSlackOutputKey, false, "If set, output batch summary as a Slack webhook payload")
	getBatchCmd.Flags().Bool(batchMarkdownOutputKey, false, "If set, output batch summary as markdown (suitable for GitHub Actions summary)")
	batchCmd.AddCommand(getBatchCmd)

	cancelBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	cancelBatchCmd.MarkFlagRequired(batchProjectKey)
	cancelBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to cancel.")
	cancelBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to cancel (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this cancels the most recent batch with that name.")
	cancelBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	batchCmd.AddCommand(cancelBatchCmd)

	testsBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	testsBatchCmd.MarkFlagRequired(batchProjectKey)
	testsBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve tests for.")
	testsBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this returns the most recent batch with that name.")
	testsBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	testsBatchCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	batchCmd.AddCommand(testsBatchCmd)

	waitBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	waitBatchCmd.MarkFlagRequired(batchProjectKey)
	waitBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to await completion.")
	waitBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to await completion (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this waits for the most recent batch with that name.")
	waitBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	waitBatchCmd.Flags().String(batchWaitTimeoutKey, "1h", "Amount of time to wait for a batch to finish, expressed in Golang duration string.")
	waitBatchCmd.Flags().String(batchWaitPollKey, "30s", "Interval between checking batch status, expressed in Golang duration string.")
	batchCmd.AddCommand(waitBatchCmd)

	logsBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	logsBatchCmd.MarkFlagRequired(batchProjectKey)
	logsBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to list logs for.")
	logsBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to list logs for (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this lists logs for the most recent batch with that name.")
	logsBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	logsBatchCmd.MarkFlagsOneRequired(batchIDKey, batchNameKey)
	batchCmd.AddCommand(logsBatchCmd)

	rerunBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	rerunBatchCmd.MarkFlagRequired(batchProjectKey)
	rerunBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to rerun tests for.")
	rerunBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to rerun tests for (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this reruns the most recent batch with that name.")
	rerunBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	rerunBatchCmd.Flags().StringSlice(batchTestIDsKey, []string{}, "Comma-separated list of test IDs to rerun. If none are provided, only the batch-metrics phase will be rerun.")
	rerunBatchCmd.Flags().Bool(batchSyncWithTestSuiteKey, false, "If set, re-runs the batch with the latest test suite revision, adding any additional experiences from the test suite to the batch.")
	batchCmd.AddCommand(rerunBatchCmd)

	superviseBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project to supervise")
	superviseBatchCmd.MarkFlagRequired(batchProjectKey)
	superviseBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to supervise.")
	superviseBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to supervise (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this supervises the most recent batch with that name.")
	superviseBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	superviseBatchCmd.MarkFlagsOneRequired(batchIDKey, batchNameKey)
	superviseBatchCmd.Flags().Int(batchMaxRerunAttemptsKey, 0, "Maximum number of rerun attempts for failed tests")
	superviseBatchCmd.MarkFlagRequired(batchMaxRerunAttemptsKey)
	superviseBatchCmd.Flags().Float64(batchRerunMaxFailurePercentKey, 50, "Maximum percentage of failed jobs before stopping (1-100)")
	superviseBatchCmd.MarkFlagRequired(batchRerunMaxFailurePercentKey)
	superviseBatchCmd.Flags().String(batchRerunOnStatesKey, "", "States to trigger rerun on (e.g. Warning, Error, Blocker)")
	superviseBatchCmd.MarkFlagRequired(batchRerunOnStatesKey)
	superviseBatchCmd.Flags().String(batchWaitTimeoutKey, "1h", "Amount of time to wait for a batch to finish, expressed in Golang duration string.")
	superviseBatchCmd.Flags().String(batchWaitPollKey, "30s", "Interval between checking batch status, expressed in Golang duration string.")
	batchCmd.AddCommand(superviseBatchCmd)

	rootCmd.AddCommand(batchCmd)
}

func GetCIEnvironmentVariableAccount() string {
	account := ""
	// We check for environment variables for common CI systems (and check the two possible options in GitHub just in case)
	if githubActor := os.Getenv("GITHUB_ACTOR"); githubActor != "" {
		account = githubActor
	} else if githubTriggeringActor := os.Getenv("GITHUB_TRIGGERING_ACTOR"); githubTriggeringActor != "" {
		account = githubTriggeringActor
	} else if gitlabUser := os.Getenv("GITLAB_USER_LOGIN"); gitlabUser != "" {
		account = gitlabUser
	}
	return account
}

func envVarSet(name string) bool {
	_, ok := os.LookupEnv(name)
	return ok
}

// Attempt to determine the environment we're in. i.e. are we running in Gitlab CI?
// Github CI? or maybe just running locally on a customer's machine? This information
// is useful downstream, for understanding where a batch was triggered from.
func DetermineTriggerMethod() *api.TriggeredVia {
	if envVarSet("GITHUB_ACTOR") || envVarSet("GITHUB_ACTIONS") {
		return Ptr(api.GITHUB)
	}
	if envVarSet("GITLAB_USER_LOGIN") || envVarSet("GITLAB_CI") {
		return Ptr(api.GITLAB)
	}
	if envVarSet("CI") {
		// Unfortunately, we're not sure what CI system we're being executed from
		return nil
	}
	return Ptr(api.LOCAL)
}

func parseRerunStates(rerunOnStates string) []api.ConflatedJobStatus {
	if rerunOnStates == "" {
		return []api.ConflatedJobStatus{}
	}

	states := strings.Split(rerunOnStates, ",")
	var conflatedStates []api.ConflatedJobStatus

	for _, state := range states {
		state = strings.TrimSpace(strings.ToUpper(state))
		switch state {
		case "WARNING":
			conflatedStates = append(conflatedStates, api.ConflatedJobStatusWARNING)
		case "ERROR":
			conflatedStates = append(conflatedStates, api.ConflatedJobStatusERROR)
		case "BLOCKER":
			conflatedStates = append(conflatedStates, api.ConflatedJobStatusBLOCKER)
		default:
			log.Fatalf("Unsupported rerun state: %s. Valid states are: WARNING, ERROR, BLOCKER", state)
		}
	}

	return conflatedStates
}

func getAllJobs(projectID uuid.UUID, batchID uuid.UUID) []api.Job {
	var allJobs []api.Job
	var pageToken *string = nil

	for {
		response, err := Client.ListJobsWithResponse(context.Background(), projectID, batchID, &api.ListJobsParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("unable to list jobs:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list jobs", response.HTTPResponse, response.Body)
		if response.JSON200.Jobs == nil {
			log.Fatal("unable to list jobs")
		}
		responseJobs := *response.JSON200.Jobs
		allJobs = append(allJobs, responseJobs...)

		if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
			pageToken = response.JSON200.NextPageToken
		} else {
			break
		}
	}

	return allJobs
}

func filterJobsByStatus(jobs []api.Job, conflatedStatuses []api.ConflatedJobStatus) []uuid.UUID {
	if len(conflatedStatuses) == 0 {
		return []uuid.UUID{}
	}

	var jobIDs []uuid.UUID
	for _, job := range jobs {
		for _, status := range conflatedStatuses {
			if job.ConflatedStatus != nil && *job.ConflatedStatus == status {
				if job.JobID != nil {
					jobIDs = append(jobIDs, *job.JobID)
				}
				break // Found a match, no need to check other statuses
			}
		}
	}

	return jobIDs
}

// SuperviseResult contains the result information from actualSuperviseBatch
type SuperviseResult struct {
	Batch *api.Batch
	Error error
}

// SuperviseParams contains the parameters needed for actualSuperviseBatch
type SuperviseParams struct {
	ProjectID                uuid.UUID
	MaxRerunAttempts         int
	RerunMaxFailurePercent   float64
	UndesiredConflatedStates []api.ConflatedJobStatus
	Timeout                  time.Duration
	PollInterval             time.Duration
	BatchID                  string
	BatchName                string
}

func getSuperviseParams(ccmd *cobra.Command, args []string) (*SuperviseParams, error) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	maxRerunAttempts := viper.GetInt(batchMaxRerunAttemptsKey)
	rerunMaxFailurePercent := viper.GetFloat64(batchRerunMaxFailurePercentKey)
	rerunOnStates := viper.GetString(batchRerunOnStatesKey)

	// Validate rerun-max-failure-percent <= 0.0 or > 100.0
	if rerunMaxFailurePercent <= 0.0 || rerunMaxFailurePercent > 100 {
		return nil, fmt.Errorf("rerun-max-failure-percent must be greater than 0 and less than 100, got: %f", rerunMaxFailurePercent)
	}

	if maxRerunAttempts < 0 {
		return nil, fmt.Errorf("max-rerun-attempts must be at least 0, got: %d", maxRerunAttempts)
	}

	// Parse rerun states
	conflatedStates := parseRerunStates(rerunOnStates)

	// Parse timeout and poll interval
	pollInterval, _ := time.ParseDuration(viper.GetString(batchWaitPollKey))
	timeout, _ := time.ParseDuration(viper.GetString(batchWaitTimeoutKey))

	return &SuperviseParams{
		ProjectID:                projectID,
		MaxRerunAttempts:         maxRerunAttempts,
		RerunMaxFailurePercent:   rerunMaxFailurePercent,
		UndesiredConflatedStates: conflatedStates,
		Timeout:                  timeout,
		PollInterval:             pollInterval,
		BatchID:                  viper.GetString(batchIDKey),   // validated in waitForBatchCompletion
		BatchName:                viper.GetString(batchNameKey), // validated in waitForBatchCompletion
	}, nil
}

func getMatchingJobIDs(batch *api.Batch, params *SuperviseParams, attempt int) []uuid.UUID {
	// Check if we've reached max attempts first (before any API calls)
	if attempt >= params.MaxRerunAttempts {
		return nil // Max attempts reached, no more reruns
	}

	// If batch is cancelled, do not rerun
	if *batch.Status == api.BatchStatusCANCELLED {
		return nil // No rerun needed for cancelled batch
	}

	// Get all jobs and filter by status
	allJobs := getAllJobs(params.ProjectID, *batch.BatchID)
	matchingJobIDs := filterJobsByStatus(allJobs, params.UndesiredConflatedStates)
	log.Printf("Found %d job IDs matching rerun states: %v\n", len(matchingJobIDs), matchingJobIDs)

	// Check threshold before rerunning
	totalJobs := len(allJobs)
	failedJobs := len(matchingJobIDs)
	if totalJobs > 0 {
		failedPercentage := float64(failedJobs*100) / float64(totalJobs)
		log.Printf("Failed job percentage: %.1f%% (%d/%d jobs)\n", failedPercentage, failedJobs, totalJobs)
		if failedPercentage > params.RerunMaxFailurePercent {
			return nil // Threshold exceeded, no rerun needed
		}
	}

	return matchingJobIDs
}

func actualSuperviseBatch(ccmd *cobra.Command, args []string) *SuperviseResult {

	// Get parameters
	params, err := getSuperviseParams(ccmd, args)
	if err != nil {
		return &SuperviseResult{
			Error: err,
		}
	}

	// Unified loop for initial batch and reruns
	var batch *api.Batch

	for attempt := 0; attempt <= params.MaxRerunAttempts; attempt++ {
		var err error

		batch, err = waitForBatchCompletion(params.ProjectID, params.BatchID, params.BatchName, params.Timeout, params.PollInterval)

		// Check timeout
		if err != nil {
			if timeoutErr, ok := err.(*TimeoutError); ok {
				return &SuperviseResult{
					Error: timeoutErr,
				}
			} else {
				return &SuperviseResult{
					Error: fmt.Errorf("Error retrieving batch: %v", err),
				}
			}
		}

		log.Printf("Batch completed with status: %s\n", *batch.Status)

		// Check if rerun is required (includes max attempts check)
		matchingJobIDs := getMatchingJobIDs(batch, params, attempt)
		if len(matchingJobIDs) == 0 {
			return &SuperviseResult{
				Batch: batch,
			}
		}

		response, err := submitBatchRerun(params.ProjectID, *batch.BatchID, matchingJobIDs, 30*time.Second, false)
		if err != nil {
			return &SuperviseResult{
				Error: err,
			}
		}
		newBatchID := response.JSON200.BatchID
		log.Printf("Submitted rerun batch: %s\n", newBatchID.String())

		// Update batch ID for next iteration
		params.BatchID = newBatchID.String()
		params.BatchName = "" // Clear batch name since we're using ID now
	}

	// This should never happen, but we'll return the batch if we get here
	log.Fatal("Control should never reach here. Returning batch.")
	return &SuperviseResult{
		Batch: batch,
	}
}

func superviseBatch(ccmd *cobra.Command, args []string) {

	result := actualSuperviseBatch(ccmd, args)

	// Set the batch ID for future reference
	if result.Batch != nil && result.Batch.BatchID != nil {
		viper.Set(batchIDKey, result.Batch.BatchID.String())
	}

	// Exit with appropriate code based on final status
	results := []*SuperviseResult{result}
	exitWithBatchStatus(results, false, true)
}

func createBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batchGithub := viper.GetBool(batchGithubKey)
	if !batchGithub {
		fmt.Println("Creating a batch...")
	}

	// Parse the build ID
	buildID, err := uuid.Parse(viper.GetString(batchBuildIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal("failed to parse build ID: ", err)
	}

	var allExperienceIDs []uuid.UUID
	var allExperienceNames []string

	// Parse --experience-ids
	if viper.IsSet(batchExperienceIDsKey) {
		experienceIDs := parseUUIDs(viper.GetString(batchExperienceIDsKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
	}

	// Parse --experiences into either IDs or names
	if viper.IsSet(batchExperiencesKey) {
		experienceIDs, experienceNames := parseUUIDsAndNames(viper.GetString(batchExperiencesKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
		allExperienceNames = append(allExperienceNames, experienceNames...)
	}

	metricsBuildID := uuid.Nil
	if viper.IsSet(batchMetricsBuildKey) {
		metricsBuildID, err = uuid.Parse(viper.GetString(batchMetricsBuildKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
	}

	if viper.IsSet(batchExperienceTagIDsKey) && viper.IsSet(batchExperienceTagNamesKey) {
		log.Fatalf("failed to create batch: %v and %v are mutually exclusive parameters", batchExperienceTagNamesKey, batchExperienceTagIDsKey)
	}

	var allExperienceTagIDs []uuid.UUID
	var allExperienceTagNames []string

	// Parse --experience-tag-ids
	if viper.IsSet(batchExperienceTagIDsKey) {
		experienceTagIDs := parseUUIDs(viper.GetString(batchExperienceTagIDsKey))
		allExperienceTagIDs = append(allExperienceTagIDs, experienceTagIDs...)
	}

	// Parse --experience-tag-names:
	if viper.IsSet(batchExperienceTagNamesKey) {
		experienceTagNames := strings.Split(viper.GetString(batchExperienceTagNamesKey), ",")
		for i := range experienceTagNames {
			experienceTagNames[i] = strings.TrimSpace(experienceTagNames[i])
		}
		allExperienceTagNames = append(allExperienceTagNames, experienceTagNames...)
	}

	// Parse --experience-tags
	if viper.IsSet(batchExperienceTagsKey) {
		experienceTagIDs, experienceTagNames := parseUUIDsAndNames(viper.GetString(batchExperienceTagsKey))
		allExperienceTagIDs = append(allExperienceTagIDs, experienceTagIDs...)
		allExperienceTagNames = append(allExperienceTagNames, experienceTagNames...)
	}

	// Parse --parameter (if any provided)
	parameters := api.BatchParameters{}
	if viper.IsSet(batchParameterKey) {
		parameterStrings := viper.GetStringSlice(batchParameterKey)
		for _, parameterString := range parameterStrings {
			key, value, err := ParseParameterString(parameterString)
			if err != nil {
				log.Fatal(err)
			}
			parameters[key] = value
		}
	}

	poolLabels := getAndValidatePoolLabels(batchPoolLabelsKey)

	// Process the associated account: by default, we try to get from CI/CD environment variables
	// Otherwise, we use the account flag. The default is "".
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(batchAccountKey) {
		associatedAccount = viper.GetString(batchAccountKey)
	}

	metricsSet := ProcessMetricsSet(batchMetricsSetKey, &poolLabels)

	// Build the request body
	body := api.BatchInput{
		BuildID:           &buildID,
		Parameters:        &parameters,
		AssociatedAccount: &associatedAccount,
		TriggeredVia:      DetermineTriggerMethod(),
		MetricsSetName:    metricsSet,
	}

	// Parse --batch-name (if any provided)
	if viper.IsSet(batchNameKey) {
		body.BatchName = Ptr(viper.GetString(batchNameKey))
	}

	// Parse --allowable-failure-percent (if any provided)
	if viper.IsSet(batchAllowableFailurePercentKey) {
		allowableFailurePercent := viper.GetInt(batchAllowableFailurePercentKey)
		if allowableFailurePercent < 0 || allowableFailurePercent > 100 {
			log.Fatal("allowable failure percent must be between 0 and 100")
		}
		body.AllowableFailurePercent = &allowableFailurePercent
	}

	if allExperienceIDs != nil {
		body.ExperienceIDs = &allExperienceIDs
	}

	if allExperienceNames != nil {
		body.ExperienceNames = &allExperienceNames
	}

	if allExperienceTagIDs != nil {
		body.ExperienceTagIDs = &allExperienceTagIDs
	}

	if allExperienceTagNames != nil {
		body.ExperienceTagNames = &allExperienceTagNames
	}

	if metricsBuildID != uuid.Nil {
		body.MetricsBuildID = &metricsBuildID
	}

	if len(poolLabels) != 0 {
		body.PoolLabels = &poolLabels
	}

	// Sync metrics2.0 config
	if viper.GetBool(batchSyncMetricsConfigKey) {
		build, err := Client.GetBuildWithResponse(context.Background(), projectID, buildID)
		if err != nil {
			log.Fatal("unable to retrieve build:", err)
		}
		branchID := build.JSON200.BranchID
		if branchID == uuid.Nil {
			log.Fatal("build has no branch associated with it")
		}

		metricsConfigPath := viper.GetString(batchMetricsConfigPath)
		metricsTemplatesPath := viper.GetString(batchMetricsTemplatesPath)
		if err := SyncMetricsConfig(projectID, branchID, metricsConfigPath, metricsTemplatesPath, false); err != nil {
			log.Fatalf("failed to sync metrics before batch: %v", err)
		}
	}

	// Make the request
	response, err := Client.CreateBatchWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create batch:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create batch", response.HTTPResponse, response.Body)

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

func getBatchStatusText(batch *api.Batch) string {
	if batch.Status != nil {
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			return "ran successfully"
		case api.BatchStatusERROR:
			return "completed with errors"
		case api.BatchStatusCANCELLED:
			return "was cancelled"
		case api.BatchStatusSUBMITTED, api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
			return "is running"
		default:
			return "ran"
		}
	}
	return "ran"
}

// BatchMetadata contains suite, system, and URL information for a batch
type BatchMetadata struct {
	Suite     api.TestSuite
	System    api.System
	SuiteUrl  string
	BatchUrl  string
	SystemUrl string
}

func getBatchMetadata(batch *api.Batch) *BatchMetadata {
	baseUrl, err := url.Parse(strings.Replace(viper.GetString(urlKey), "api", "app", 1))
	if err != nil {
		log.Fatal("unable to parse url:", err)
	}
	baseUrl.Path, err = url.JoinPath("projects", batch.ProjectID.String())
	if err != nil {
		log.Fatal("unable to build base url:", err)
	}

	// Get the suite object
	suiteResponse, err := Client.GetTestSuiteWithResponse(context.Background(), *batch.ProjectID, *batch.TestSuiteID)
	if err != nil {
		log.Fatal("unable to retrieve suite for batch:", err)
	}
	ValidateResponse(http.StatusOK, "unable to retrieve suite for batch", suiteResponse.HTTPResponse, suiteResponse.Body)
	suite := *suiteResponse.JSON200

	// Get the system object
	systemResponse, err := Client.GetSystemWithResponse(context.Background(), *batch.ProjectID, *batch.SystemID)
	if err != nil {
		log.Fatal("unable to retrieve system for batch:", err)
	}
	ValidateResponse(http.StatusOK, "unable to retrieve system for batch", systemResponse.HTTPResponse, systemResponse.Body)
	system := *systemResponse.JSON200

	// Build URLs
	suiteUrl := baseUrl.JoinPath("test-suites", batch.TestSuiteID.String(), "revisions", strconv.Itoa(int(*batch.TestSuiteRevision))).String()
	batchUrl := baseUrl.JoinPath("batches", batch.BatchID.String()).String()
	systemUrl := baseUrl.JoinPath("systems", batch.SystemID.String()).String()

	return &BatchMetadata{
		Suite:     suite,
		System:    system,
		SuiteUrl:  suiteUrl,
		BatchUrl:  batchUrl,
		SystemUrl: systemUrl,
	}
}

// BatchStatusCounts contains all status count information for a batch
type BatchStatusCounts struct {
	TotalJobs int
	Passed    int
	FailBlock int
	FailWarn  int
	Error     int
	Running   int
	Cancelled int
}

func getBatchStatusCounts(batch *api.Batch) *BatchStatusCounts {
	var totalJobs int
	if batch.TotalJobs != nil {
		totalJobs = int(*batch.TotalJobs)
	}

	var succeeded, errorCount, running, cancelled int
	if batch.JobStatusCounts != nil {
		succeeded = batch.JobStatusCounts.Succeeded
		errorCount = batch.JobStatusCounts.Error
		// make sure that totalJobs is the sum of all status counts shown
		running = batch.JobStatusCounts.Submitted + batch.JobStatusCounts.Running + batch.JobStatusCounts.MetricsQueued + batch.JobStatusCounts.MetricsRunning
		cancelled = batch.JobStatusCounts.Cancelled
	}

	var failBlock, failWarn int
	if batch.JobMetricsStatusCounts != nil {
		failBlock = batch.JobMetricsStatusCounts.FailBlock
		failWarn = batch.JobMetricsStatusCounts.FailWarn
	}

	// Calculate passed count (same as bff: https://github.com/resim-ai/rerun/blob/ebf0cde9472f555ae099e08e512ed4a7dfdf01f4/bff/lib/bff/batches/conflated_status_counts.ex#L49)
	passed := succeeded - (failBlock + failWarn)

	return &BatchStatusCounts{
		TotalJobs: totalJobs,
		Passed:    passed,
		FailBlock: failBlock,
		FailWarn:  failWarn,
		Error:     errorCount,
		Running:   running,
		Cancelled: cancelled,
	}
}

func batchToSlackWebhookPayload(batch *api.Batch) *slack.WebhookMessage {
	blocks := &slack.Blocks{BlockSet: make([]slack.Block, 2)}

	metadata := getBatchMetadata(batch)
	statusCounts := getBatchStatusCounts(batch)
	statusText := getBatchStatusText(batch)

	// Intro text
	introText := fmt.Sprintf("The <%s|%s> *<%s|run>* for <%s|%s> %s with the following breakdown:", metadata.SuiteUrl, metadata.Suite.Name, metadata.BatchUrl, metadata.SystemUrl, metadata.System.Name, statusText)
	introTextBlock := slack.NewTextBlockObject("mrkdwn", introText, false, false)
	blocks.BlockSet[0] = slack.NewSectionBlock(introTextBlock, nil, nil)

	// List section - only include non-zero counts
	boldStyle := slack.RichTextSectionTextStyle{Bold: true}
	buildListElement := func(count int, label string, filter string) *slack.RichTextSection {
		return slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(fmt.Sprintf("%d ", count), nil),
			slack.NewRichTextSectionLinkElement(metadata.BatchUrl+filter, label, &boldStyle),
		)
	}

	listElements := []slack.RichTextElement{
		slack.NewRichTextSection(slack.NewRichTextSectionTextElement(fmt.Sprintf("%d total tests", statusCounts.TotalJobs), nil)),
	}
	// Always show Passed and Blocking regardless of count
	listElements = append(listElements, buildListElement(statusCounts.Passed, "Passed", "?TEST_STATUS_MULTI=Passed"))
	listElements = append(listElements, buildListElement(statusCounts.FailBlock, "Blocking", "?TEST_STATUS_MULTI=Blocker"))
	// Only show Warning, Erroring, and Running if count > 0
	if statusCounts.FailWarn > 0 {
		listElements = append(listElements, buildListElement(statusCounts.FailWarn, "Warning", "?TEST_STATUS_MULTI=Warning"))
	}
	if statusCounts.Error > 0 {
		listElements = append(listElements, buildListElement(statusCounts.Error, "Erroring", "?TEST_STATUS_MULTI=Error"))
	}
	if statusCounts.Running > 0 {
		listElements = append(listElements, buildListElement(statusCounts.Running, "Running", "?TEST_STATUS_MULTI=Running&TEST_STATUS_MULTI=Queued"))
	}
	if statusCounts.Cancelled > 0 {
		listElements = append(listElements, buildListElement(statusCounts.Cancelled, "Cancelled", "?TEST_STATUS_MULTI=Cancelled"))
	}

	listBlock := slack.NewRichTextList("bullet", 0, listElements...)
	blocks.BlockSet[1] = slack.NewRichTextBlock("list", listBlock)

	webhookPayload := slack.WebhookMessage{
		Blocks: blocks,
	}
	return &webhookPayload
}

func batchToMarkdown(batch *api.Batch) string {
	metadata := getBatchMetadata(batch)
	statusCounts := getBatchStatusCounts(batch)
	statusText := getBatchStatusText(batch)

	// Build markdown output
	var markdown strings.Builder
	markdown.WriteString(fmt.Sprintf("The [%s](%s) **[run](%s)** for [%s](%s) %s with the following breakdown:\n\n", metadata.Suite.Name, metadata.SuiteUrl, metadata.BatchUrl, metadata.System.Name, metadata.SystemUrl, statusText))
	markdown.WriteString(fmt.Sprintf("- %d total tests\n", statusCounts.TotalJobs))
	// Always show Passed and Blocking regardless of count
	markdown.WriteString(fmt.Sprintf("- %d **[Passed](%s?TEST_STATUS_MULTI=Passed)**\n", statusCounts.Passed, metadata.BatchUrl))
	markdown.WriteString(fmt.Sprintf("- %d **[Blocking](%s?TEST_STATUS_MULTI=Blocker)**\n", statusCounts.FailBlock, metadata.BatchUrl))
	// Only show Warning, Erroring, and Running if count > 0
	if statusCounts.FailWarn > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Warning](%s?TEST_STATUS_MULTI=Warning)**\n", statusCounts.FailWarn, metadata.BatchUrl))
	}
	if statusCounts.Error > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Erroring](%s?TEST_STATUS_MULTI=Error)**\n", statusCounts.Error, metadata.BatchUrl))
	}
	if statusCounts.Running > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Running](%s?TEST_STATUS_MULTI=Running&TEST_STATUS_MULTI=Queued)**\n", statusCounts.Running, metadata.BatchUrl))
	}
	if statusCounts.Cancelled > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Cancelled](%s?TEST_STATUS_MULTI=Cancelled)**\n", statusCounts.Cancelled, metadata.BatchUrl))
	}
	return markdown.String()
}

func actualGetBatch(projectID uuid.UUID, batchIDRaw string, batchName string) *api.Batch {
	var batch *api.Batch
	if batchIDRaw != "" {
		batchID, err := uuid.Parse(batchIDRaw)
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
		response, err := Client.GetBatchWithResponse(context.Background(), projectID, batchID)
		if err != nil {
			log.Fatal("unable to retrieve batch:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve batch", response.HTTPResponse, response.Body)
		batch = response.JSON200
		return batch
	} else if batchName != "" {
		var pageToken *string = nil
		for {
			response, err := Client.ListBatchesWithResponse(context.Background(), projectID, &api.ListBatchesParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list batches:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list batches", response.HTTPResponse, response.Body)
			if response.JSON200.Batches == nil {
				log.Fatal("unable to find batch: ", batchName)
			}
			batches := *response.JSON200.Batches

			for _, b := range batches {
				if b.FriendlyName != nil && *b.FriendlyName == batchName {
					batch = &b
					return batch
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
	return batch
}

func getBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batch := actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))

	if viper.GetBool(batchExitStatusKey) {
		// Exit with appropriate code based on status (including extended statuses)
		exitWithSingleBatchStatus(batch, nil, true, false)
	}
	if viper.GetBool(batchMarkdownOutputKey) {
		fmt.Print(batchToMarkdown(batch))
	} else if viper.GetBool(batchSlackOutputKey) {
		OutputJson(batchToSlackWebhookPayload(batch))
	} else {
		OutputJson(batch)
	}
}

// TimeoutError represents a timeout condition
type TimeoutError struct {
	message string
}

func (e *TimeoutError) Error() string {
	return e.message
}

func (e *TimeoutError) IsTimeout() bool {
	return true
}

// exitWithBatchStatus determines and executes the appropriate exit code based on batch status(es)
// It handles both single batch and multiple batch scenarios.
// Priority: 1 (internal error) > 6 (timeout) > 2 (ERROR) > 5 (CANCELLED) > 3 (SUBMITTED) > 4 (RUNNING) > 0 (SUCCEEDED)
func exitWithBatchStatus(results []*SuperviseResult, includeExtendedStatuses bool, shouldLog bool) {
	hasTimeout := false
	hasError := false
	hasCancelled := false
	hasSubmitted := false
	hasRunning := false
	allSucceeded := true
	isMultiple := len(results) > 1

	for _, res := range results {
		if res.Error != nil {
			if timeoutErr, ok := res.Error.(*TimeoutError); ok {
				hasTimeout = true
				log.Printf("Batch timed out: %v\n", timeoutErr.message)
			} else {
				// Internal error - fatal
				log.Fatal(res.Error)
			}
		} else if res.Batch != nil && res.Batch.Status != nil {
			switch *res.Batch.Status {
			case api.BatchStatusSUCCEEDED:
				// Continue checking other batches
			case api.BatchStatusERROR:
				hasError = true
				allSucceeded = false
			case api.BatchStatusCANCELLED:
				hasCancelled = true
				allSucceeded = false
			case api.BatchStatusSUBMITTED:
				if includeExtendedStatuses {
					hasSubmitted = true
				}
				allSucceeded = false
			case api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
				if includeExtendedStatuses {
					hasRunning = true
				}
				allSucceeded = false
			default:
				allSucceeded = false
			}
		} else {
			allSucceeded = false
		}
	}

	// Exit with appropriate code based on priority
	if hasTimeout {
		if shouldLog && isMultiple {
			log.Println("One or more batches timed out")
		} else if shouldLog {
			log.Println("Batch timed out")
		}
		os.Exit(6)
	}
	if hasError {
		os.Exit(2)
	}
	if hasCancelled {
		os.Exit(5)
	}
	if includeExtendedStatuses {
		if hasSubmitted {
			os.Exit(3)
		}
		if hasRunning {
			os.Exit(4)
		}
	}
	if allSucceeded {
		if shouldLog && isMultiple {
			log.Println("All batches completed successfully")
		} else if shouldLog {
			log.Println("Batch Completed Successfully")
		}
		os.Exit(0)
	}

	// Fallback (shouldn't happen)
	log.Fatal("unknown batch status")
}

// exitWithSingleBatchStatus is a convenience function for single batch exit code determination
func exitWithSingleBatchStatus(batch *api.Batch, err error, includeExtendedStatuses bool, shouldLog bool) {
	if err != nil {
		if timeoutErr, ok := err.(*TimeoutError); ok {
			log.Println("Batch timed out:", timeoutErr.message)
			os.Exit(6)
		}
		// Other errors get exit code 1
		log.Fatal(err)
	}

	if batch == nil || batch.Status == nil {
		log.Fatal("no batch status returned")
	}

	results := []*SuperviseResult{
		{Batch: batch, Error: nil},
	}
	exitWithBatchStatus(results, includeExtendedStatuses, shouldLog)
}

func waitForBatchCompletion(projectID uuid.UUID, batchID string, batchName string, timeout time.Duration, pollInterval time.Duration) (*api.Batch, error) {
	startTime := time.Now()

	for {
		batch := actualGetBatch(projectID, batchID, batchName)
		if batch.Status == nil {
			return nil, fmt.Errorf("no status returned")
		}

		// Check if batch is in final state
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			return batch, nil
		case api.BatchStatusERROR:
			return batch, nil
		case api.BatchStatusCANCELLED:
			return batch, nil
		case api.BatchStatusSUBMITTED, api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
			// Continue waiting
		default:
			return nil, fmt.Errorf("unknown batch status: %s", *batch.Status)
		}

		// Check timeout
		if time.Now().After(startTime.Add(timeout)) {
			return batch, &TimeoutError{message: fmt.Sprintf("timeout after %v, last state %s", timeout, *batch.Status)}
		}

		time.Sleep(pollInterval)
	}
}

func waitBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	timeout, _ := time.ParseDuration(viper.GetString(batchWaitTimeoutKey))
	pollWait, _ := time.ParseDuration(viper.GetString(batchWaitPollKey))

	batch, err := waitForBatchCompletion(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey), timeout, pollWait)

	// Set the batch ID for future reference
	if batch != nil && batch.BatchID != nil {
		viper.Set(batchIDKey, batch.BatchID.String())
	}

	// Exit with appropriate code based on final status
	exitWithSingleBatchStatus(batch, err, false, true)
}

func testsBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	var batchID uuid.UUID
	var err error
	if viper.IsSet(batchIDKey) {
		batchID, err = uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
	} else if viper.IsSet(batchNameKey) {
		batch := actualGetBatch(projectID, "", viper.GetString(batchNameKey))
		batchID = *batch.BatchID
	} else {
		log.Fatal("must specify either the batch ID or the batch name")
	}

	// Now list the tests
	tests := []api.Job{}
	var pageToken *string = nil
	for {
		response, err := Client.ListJobsWithResponse(context.Background(), projectID, batchID, &api.ListJobsParams{
			PageSize:  Ptr(100),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("unable to list tests:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list tests", response.HTTPResponse, response.Body)
		if response.JSON200.Jobs == nil {
			log.Fatal("unable to list tests")
		}
		responseJobs := *response.JSON200.Jobs
		tests = append(tests, responseJobs...)

		if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
			pageToken = response.JSON200.NextPageToken
		} else {
			break
		}
	}
	OutputJson(tests)
}

func listBatchLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	var batchID uuid.UUID
	var err error
	if viper.IsSet(batchIDKey) {
		batchID, err = uuid.Parse(viper.GetString(batchIDKey))
		if err != nil {
			log.Fatal("unable to parse batch ID: ", err)
		}
	} else {
		batch := actualGetBatch(projectID, "", viper.GetString(batchNameKey))
		batchID = *batch.BatchID
	}
	logs := []api.BatchLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListBatchLogsForBatchWithResponse(context.Background(), projectID, batchID, &api.ListBatchLogsForBatchParams{
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

func cancelBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batch := actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))

	response, err := Client.CancelBatchWithResponse(context.Background(), projectID, *batch.BatchID)
	if err != nil {
		log.Fatal("failed to cancel batch:", err)
	}
	ValidateResponse(http.StatusOK, "failed to cancel batch", response.HTTPResponse, response.Body)
	fmt.Println("Batch cancelled successfully!")
}

func submitBatchRerun(projectID uuid.UUID, batchID uuid.UUID, jobIDs []uuid.UUID, conflictRetryDelay time.Duration, syncWithTestSuite bool) (*api.RerunBatchResponse, error) {
	// Start with an empty list of job IDs
	rerunInput := api.RerunBatchInput{
		JobIDs:    &[]uuid.UUID{},
		SyncBatch: syncWithTestSuite,
	}
	if len(jobIDs) > 0 {
		rerunInput.JobIDs = &jobIDs
	}

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempting to rerun batch (%d/%d)...", attempt, maxRetries)
		response, err := Client.RerunBatchWithResponse(context.Background(), projectID, batchID, rerunInput)
		if err != nil {
			return nil, fmt.Errorf("failed to rerun batch: %v", err)
		}
		if response != nil && response.StatusCode() == http.StatusConflict {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to rerun batch: max retries reached (%d/%d)", attempt, maxRetries)
			}
			log.Println("Existing batch run is cleaning up. Retrying...")
			time.Sleep(conflictRetryDelay)
			continue
		}
		err = ValidateResponseSafe(http.StatusOK, "failed to rerun batch", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
	return nil, fmt.Errorf("failed to rerun batch: unknown error")
}

func rerunBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	batch := actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))

	jobIDs := []uuid.UUID{}
	for _, jobID := range viper.GetStringSlice(batchTestIDsKey) {
		jobID, err := uuid.Parse(jobID)
		if err != nil {
			log.Fatal("unable to parse job ID: ", err)
		}
		jobIDs = append(jobIDs, jobID)
	}

	_, err := submitBatchRerun(projectID, *batch.BatchID, jobIDs, 30*time.Second, viper.GetBool(batchSyncWithTestSuiteKey))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Batch rerun successfully!")
}
