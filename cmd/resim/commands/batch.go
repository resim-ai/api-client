package commands

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
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
)

const (
	batchProjectKey            = "project"
	batchBuildIDKey            = "build-id"
	batchExperienceIDsKey      = "experience-ids"
	batchExperiencesKey        = "experiences"
	batchExperienceTagIDsKey   = "experience-tag-ids"
	batchExperienceTagNamesKey = "experience-tag-names"
	batchExperienceTagsKey     = "experience-tags"
	batchParameterKey          = "parameter"
	batchPoolLabelsKey         = "pool-labels"
	batchIDKey                 = "batch-id"
	batchNameKey               = "batch-name"
	batchAccountKey            = "account"
	batchGithubKey             = "github"
	batchMetricsBuildKey       = "metrics-build-id"
	batchExitStatusKey         = "exit-status"
	batchWaitTimeoutKey        = "wait-timeout"
	batchWaitPollKey           = "poll-every"
	batchSlackOutputKey        = "slack"
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
	createBatchCmd.Flags().StringSlice(batchParameterKey, []string{}, "(Optional) Parameter overrides to pass to the build. Format: <parameter-name>:<parameter-value>. Accepts repeated parameters or comma-separated parameters.")
	createBatchCmd.Flags().StringSlice(batchPoolLabelsKey, []string{}, "Pool labels to determine where to run this batch. Pool labels are interpreted as a logical AND. Accepts repeated labels or comma-separated labels.")
	createBatchCmd.MarkFlagsOneRequired(batchExperienceIDsKey, batchExperiencesKey, batchExperienceTagIDsKey, batchExperienceTagNamesKey, batchExperienceTagsKey)
	createBatchCmd.Flags().String(batchAccountKey, "", "Specify a username for a CI/CD platform account to associate with this test batch.")
	createBatchCmd.Flags().String(batchNameKey, "", "An optional name for the batch. If not supplied, ReSim generates a pseudo-unique name e.g rejoicing-aquamarine-starfish. This name need not be unique, but uniqueness is recommended to make it easier to identify batches.")
	batchCmd.AddCommand(createBatchCmd)

	getBatchCmd.Flags().String(batchProjectKey, "", "The name or ID of the project the batch is associated with")
	getBatchCmd.MarkFlagRequired(batchProjectKey)
	getBatchCmd.Flags().String(batchIDKey, "", "The ID of the batch to retrieve.")
	getBatchCmd.Flags().String(batchNameKey, "", "The name of the batch to retrieve (e.g. rejoicing-aquamarine-starfish). If the name is not unique, this returns the most recent batch with that name.")
	getBatchCmd.MarkFlagsMutuallyExclusive(batchIDKey, batchNameKey)
	getBatchCmd.Flags().Bool(batchExitStatusKey, false, "If set, exit code corresponds to batch workflow status (1 = internal CLI error, 0 = SUCCEEDED, 2=ERROR, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	getBatchCmd.Flags().Bool(batchSlackOutputKey, false, "If set, output batch summary as a Slack webhook payload")
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

// Attempt to determine the environment we're in. i.e. are we running in Gitlab CI?
// Github CI? or maybe just running locally on a customer's machine? This information
// is useful downstream, for understanding where a batch was triggered from.
func DetermineTriggerMethod() *api.TriggeredVia {
	if _, ok := os.LookupEnv("CI"); ok {
		if _, ok := os.LookupEnv("GITHUB_ACTOR"); ok {
			return Ptr(api.GITHUB)
		}
		if _, ok := os.LookupEnv("GITLAB_USER_LOGIN"); ok {
			return Ptr(api.GITLAB)
		}
		// Unfortunately, we're not sure what ENV we're being executed from
		return nil
	}
	return Ptr(api.LOCAL)
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
			parameter := strings.Split(parameterString, ":")
			if len(parameter) != 2 {
				log.Fatal("failed to parse parameter: ", parameterString, " - must be in the format <parameter-name>:<parameter-value>")
			}
			parameters[parameter[0]] = parameter[1]
		}
	}

	// Parse --pool-labels (if any provided)
	poolLabels := []api.PoolLabel{}
	if viper.IsSet(batchPoolLabelsKey) {
		poolLabels = viper.GetStringSlice(batchPoolLabelsKey)
	}
	for i := range poolLabels {
		poolLabels[i] = strings.TrimSpace(poolLabels[i])
		if poolLabels[i] == "resim" {
			log.Fatal("failed to create batch: resim is a reserved pool label")
		}
	}

	// Process the associated account: by default, we try to get from CI/CD environment variables
	// Otherwise, we use the account flag. The default is "".
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(batchAccountKey) {
		associatedAccount = viper.GetString(batchAccountKey)
	}

	// Build the request body
	body := api.BatchInput{
		BuildID:           &buildID,
		Parameters:        &parameters,
		AssociatedAccount: &associatedAccount,
		TriggeredVia:      DetermineTriggerMethod(),
	}

	// Parse --batch-name (if any provided)
	if viper.IsSet(batchNameKey) {
		body.BatchName = Ptr(viper.GetString(batchNameKey))
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

func batchToSlackWebhookPayload(batch *api.Batch) *slack.WebhookMessage {
	baseUrl, err := url.Parse(strings.Replace(viper.GetString(urlKey), "api", "app", 1))
	if err != nil {
		log.Fatal("unable to parse url:", err)
	}
	baseUrl.Path, err = url.JoinPath("projects", batch.ProjectID.String())
	if err != nil {
		log.Fatal("unable to build base url:", err)
	}
	blocks := &slack.Blocks{BlockSet: make([]slack.Block, 2)}

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

	// Intro text
	introData := struct {
		SuiteUrl   string
		SuiteName  string
		BatchUrl   string
		SystemUrl  string
		SystemName string
	}{
		baseUrl.JoinPath("test-suites", batch.TestSuiteID.String(), "revisions", strconv.Itoa(int(*batch.TestSuiteRevision))).String(),
		suite.Name,
		baseUrl.JoinPath("batches", batch.BatchID.String()).String(),
		baseUrl.JoinPath("systems", batch.SystemID.String()).String(),
		system.Name,
	}
	introTemplate := template.Must(template.New("intro").Parse("Last night’s <{{.SuiteUrl}}|{{.SuiteName}}> *<{{.BatchUrl}}|run>* for <{{.SystemUrl}}|{{.SystemName}}> ran successfully with the following breakdown:"))
	var introBuffer bytes.Buffer
	err = introTemplate.Execute(&introBuffer, introData)
	if err != nil {
		log.Fatal("couldn't execute template", err)
	}
	introTextBlock := slack.NewTextBlockObject("mrkdwn", introBuffer.String(), false, false)
	blocks.BlockSet[0] = slack.NewSectionBlock(introTextBlock, nil, nil)

	// List section
	boldStyle := slack.RichTextSectionTextStyle{Bold: true}
	buildListElement := func(count int, label string, filter string) *slack.RichTextSection {
		return slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(fmt.Sprintf("%d ", count), nil),
			slack.NewRichTextSectionLinkElement(introData.BatchUrl+"?performanceFilter="+filter, label, &boldStyle),
		)
	}

	listBlock := slack.NewRichTextList("bullet", 0,
		slack.NewRichTextSection(slack.NewRichTextSectionTextElement(fmt.Sprintf("%d total tests", *batch.TotalJobs), nil)),
		// Passed calculation borrowed from bff: https://github.com/resim-ai/rerun/blob/ebf0cde9472f555ae099e08e512ed4a7dfdf01f4/bff/lib/bff/batches/conflated_status_counts.ex#L49
		buildListElement(batch.JobStatusCounts.Succeeded-(batch.JobMetricsStatusCounts.FailBlock+batch.JobMetricsStatusCounts.FailWarn), "Passed", "Passed"),
		buildListElement(batch.JobMetricsStatusCounts.FailBlock, "Blocking", "Blocker"),
		buildListElement(batch.JobMetricsStatusCounts.FailWarn, "Warning", "Warning"),
		buildListElement(batch.JobStatusCounts.Error, "Erroring", "Error"),
	)
	blocks.BlockSet[1] = slack.NewRichTextBlock("list", listBlock)

	webhookPayload := slack.WebhookMessage{
		Blocks: blocks,
	}
	return &webhookPayload
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
		if batch.Status == nil {
			log.Fatal("no status returned")
		}
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			os.Exit(0)
		case api.BatchStatusERROR:
			os.Exit(2)
		case api.BatchStatusSUBMITTED:
			os.Exit(3)
		case api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
			os.Exit(4)
		case api.BatchStatusCANCELLED:
			os.Exit(5)
		default:
			log.Fatal("unknown batch status: ", batch.Status)
		}
	}
	if viper.GetBool(batchSlackOutputKey) {
		OutputJson(batchToSlackWebhookPayload(batch))
	} else {
		OutputJson(batch)
	}
}

func waitBatch(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(batchProjectKey))
	var batch *api.Batch
	timeout, _ := time.ParseDuration(viper.GetString(batchWaitTimeoutKey))
	pollWait, _ := time.ParseDuration(viper.GetString(batchWaitPollKey))
	startTime := time.Now()
	for {
		batch = actualGetBatch(projectID, viper.GetString(batchIDKey), viper.GetString(batchNameKey))
		if batch.Status == nil {
			log.Fatal("no status returned")
		}
		viper.Set(batchIDKey, batch.BatchID.String())
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			os.Exit(0)
		case api.BatchStatusERROR:
			os.Exit(2)
		case api.BatchStatusSUBMITTED, api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
		case api.BatchStatusCANCELLED:
			os.Exit(5)
		default:
			log.Fatal("unknown batch status: ", *batch.Status)
		}

		if time.Now().After(startTime.Add(timeout)) {
			log.Fatalf("Failed to reach a final state after %v, last state %s", timeout, *batch.Status)
			os.Exit(6)
		}
		time.Sleep(pollWait)
	}
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
