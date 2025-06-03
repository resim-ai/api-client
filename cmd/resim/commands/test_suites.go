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
	testSuiteCmd = &cobra.Command{
		Use:     "test-suites",
		Short:   "test suites contains commands for creating and managing test suites",
		Long:    ``,
		Aliases: []string{"test-suite", "suite", "suites"},
	}

	createTestSuiteCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new test suite",
		Long:  ``,
		Run:   createTestSuite,
	}

	listTestSuiteCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List all the test suites associated with this project",
		Long:  ``,
		Run:   listTestSuites,
	}

	getTestSuiteCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves a test suite's latest revision, all revisions, or a specific test suite revision",
		Long:  ``,
		Run:   getTestSuite,
	}

	archiveTestSuiteCmd = &cobra.Command{
		Use:   "archive",
		Short: "archive - Archive a test suite",
		Long:  ``,
		Run:   archiveTestSuite,
	}

	restoreTestSuiteCmd = &cobra.Command{
		Use:   "restore",
		Short: "restore - Restore an archived test suite",
		Long:  ``,
		Run:   restoreTestSuite,
	}

	reviseTestSuiteCmd = &cobra.Command{
		Use:   "revise",
		Short: "revise - Revise a test suite, updating the name, metrics, or experiences",
		Long:  ``,
		Run:   reviseTestSuite,
	}

	runTestSuiteCmd = &cobra.Command{
		Use:   "run",
		Short: "run - Run a test suite, creating a test batch",
		Long:  ``,
		Run:   runTestSuite,
	}

	batchesTestSuiteCmd = &cobra.Command{
		Use:   "batches",
		Short: "batches - List the batches that have been created by running this test suite",
		Long:  ``,
		Run:   batchesTestSuite,
	}
)

const (
	testSuiteProjectKey                 = "project"
	testSuiteNameKey                    = "name"
	testSuiteDescriptionKey             = "description"
	testSuiteBuildIDKey                 = "build-id"
	testSuiteSystemKey                  = "system"
	testSuiteExperiencesKey             = "experiences"
	testSuiteParameterKey               = "parameter"
	testSuiteKey                        = "test-suite"
	testSuiteRevisionKey                = "revision"
	testSuiteAllRevisionKey             = "all-revisions"
	testSuiteGithubKey                  = "github"
	testSuiteMetricsBuildKey            = "metrics-build"
	testSuitePoolLabelsKey              = "pool-labels"
	testSuiteAccountKey                 = "account"
	testSuiteShowOnSummaryKey           = "show-on-summary"
	testSuiteBatchNameKey               = "batch-name"
	testSuiteAllowableFailurePercentKey = "allowable-failure-percent"
	testSuiteMetricsBuildOverrideKey    = "metrics-build-override"
)

func init() {
	// Create Test Suite
	createTestSuiteCmd.Flags().Bool(testSuiteGithubKey, false, "Whether to output format in github action friendly format.")
	// Project
	createTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project to associate with the test suite.")
	createTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	// System
	createTestSuiteCmd.Flags().String(testSuiteSystemKey, "", "The name or ID of the system that the test suite is designed for.")
	createTestSuiteCmd.MarkFlagRequired(testSuiteSystemKey)
	// Name
	createTestSuiteCmd.Flags().String(testSuiteNameKey, "", "The name of the test suite.")
	createTestSuiteCmd.MarkFlagRequired(testSuiteNameKey)
	// Description
	createTestSuiteCmd.Flags().String(testSuiteDescriptionKey, "", "The description of the test suite.")
	createTestSuiteCmd.MarkFlagRequired(testSuiteDescriptionKey)
	// Metrics build
	createTestSuiteCmd.Flags().String(testSuiteMetricsBuildKey, "", "The ID of the metrics build to use in this test suite.")
	// Experiences
	createTestSuiteCmd.Flags().String(testSuiteExperiencesKey, "", "List of experience names or list of experience IDs to form this test suite.")
	createTestSuiteCmd.MarkFlagRequired(testSuiteExperiencesKey)
	// Show on Summary
	createTestSuiteCmd.Flags().Bool(testSuiteShowOnSummaryKey, false, "Should latest results of this test suite be displayed on the overview dashboard?")
	testSuiteCmd.AddCommand(createTestSuiteCmd)

	// Get Test Suite
	// Project
	getTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project the test suite is associated with.")
	getTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	// Name or ID
	getTestSuiteCmd.Flags().String(testSuiteKey, "", "The name or ID of the test suite to retrieve.")
	getTestSuiteCmd.MarkFlagRequired(testSuiteKey)
	// Revision [Optional]
	getTestSuiteCmd.Flags().Int32(testSuiteRevisionKey, -1, "The specific revision of a test suite to retrieve.")
	getTestSuiteCmd.Flags().Bool(testSuiteAllRevisionKey, false, "Supply this flag to list all revisions of the test suite.")
	testSuiteCmd.AddCommand(getTestSuiteCmd)

	// Archive Test Suite
	archiveTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project the test suite is associated with.")
	archiveTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	archiveTestSuiteCmd.Flags().String(testSuiteKey, "", "The name or ID of the test suite to archive.")
	archiveTestSuiteCmd.MarkFlagRequired(testSuiteKey)
	testSuiteCmd.AddCommand(archiveTestSuiteCmd)

	restoreTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project to restore the test suite within")
	restoreTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	restoreTestSuiteCmd.Flags().String(testSuiteKey, "", "The name or ID of the test suite to restore")
	restoreTestSuiteCmd.MarkFlagRequired(testSuiteKey)
	testSuiteCmd.AddCommand(restoreTestSuiteCmd)

	// Revise Test Suite
	reviseTestSuiteCmd.Flags().Bool(testSuiteGithubKey, false, "Whether to output format in github action friendly format.")
	// Project
	reviseTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project to associate with the test suite.")
	reviseTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	// Name or ID to revise
	reviseTestSuiteCmd.Flags().String(testSuiteKey, "", "The name or ID of the test suite to retrieve.")
	reviseTestSuiteCmd.MarkFlagRequired(testSuiteKey)
	// Name [optional]
	reviseTestSuiteCmd.Flags().String(testSuiteNameKey, "", "A new name for the test suite revision.")
	// System [optional]
	reviseTestSuiteCmd.Flags().String(testSuiteSystemKey, "", "A new name or ID of the system that the new test suite is designed for.")
	// Description [optional]
	reviseTestSuiteCmd.Flags().String(testSuiteDescriptionKey, "", "A new description for the test suite revision.")
	// Show on Summary [optional]
	reviseTestSuiteCmd.Flags().Bool(testSuiteShowOnSummaryKey, false, "Should latest results of this test suite be displayed on the overview dashboard?")
	// Metrics build
	reviseTestSuiteCmd.Flags().String(testSuiteMetricsBuildKey, "", "A new ID of the metrics build to use in this test suite revision. To unset an existing metrics build, pass a nil uuid (00000000-0000-0000-0000-000000000000).")
	// Experiences
	reviseTestSuiteCmd.Flags().String(testSuiteExperiencesKey, "", "A list of updated experience names or list of experience IDs to have in the test suite revision.")
	// We need something to revise!
	reviseTestSuiteCmd.MarkFlagsOneRequired(testSuiteNameKey, testSuiteSystemKey, testSuiteDescriptionKey, testSuiteMetricsBuildKey, testSuiteExperiencesKey, testSuiteShowOnSummaryKey)
	testSuiteCmd.AddCommand(reviseTestSuiteCmd)

	// List Test Suite
	listTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project to list test suites for")
	listTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	testSuiteCmd.AddCommand(listTestSuiteCmd)

	// Run Test Suite
	runTestSuiteCmd.Flags().Bool(testSuiteGithubKey, false, "Whether to output format in github action friendly format.")
	// Project
	runTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project to associate with the test suite.")
	runTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	// Name or ID
	runTestSuiteCmd.Flags().String(testSuiteKey, "", "The name or ID of the test suite to run.")
	runTestSuiteCmd.MarkFlagRequired(testSuiteKey)
	// Revision [Optional]
	runTestSuiteCmd.Flags().String(testSuiteRevisionKey, "", "The specific revision of a test suite to run.")
	// Build ID
	runTestSuiteCmd.Flags().String(testSuiteBuildIDKey, "", "The ID of the build to use in this test suite run.")
	runTestSuiteCmd.MarkFlagRequired(testSuiteBuildIDKey)
	// Parameters
	runTestSuiteCmd.Flags().StringSlice(testSuiteParameterKey, []string{}, "(Optional) Parameter overrides to pass to the build. Format: <parameter-name>=<parameter-value> or <parameter-name>:<parameter-value>. The equals sign (=) is recommended, especially if parameter names contain colons. Accepts repeated parameters or comma-separated parameters e.g. 'param1=value1,param2=value2'. If multiple = signs are used, the first one will be used to determine the key, and the rest will be part of the value.")
	// Pool Labels
	runTestSuiteCmd.Flags().StringSlice(testSuitePoolLabelsKey, []string{}, "Pool labels to determine where to run this test suite. Pool labels are interpreted as a logical AND. Accepts repeated labels or comma-separated labels.")
	runTestSuiteCmd.Flags().String(testSuiteAccountKey, "", "Specify a username for a CI/CD platform account to associate with this test suite run.")
	// Optional: Friendly name
	runTestSuiteCmd.Flags().String(testSuiteBatchNameKey, "", "An optional name for the batch. If not supplied, ReSim generates a pseudo-unique name e.g rejoicing-aquamarine-starfish. This name need not be unique, but uniqueness is recommended to make it easier to identify batches.")
	runTestSuiteCmd.Flags().Int(testSuiteAllowableFailurePercentKey, 0, "An optional percentage (0-100) that determines the maximum percentage of tests that can have an execution error and have aggregate metrics be computed and consider the batch successfully completed. If not supplied, ReSim defaults to 0, which means that the batch will only be considered successful if all tests complete successfully.")
	// Optional: Metrics build override:
	runTestSuiteCmd.Flags().String(testSuiteMetricsBuildOverrideKey, "", "An optional ID of a metrics build to override the standard metrics build in this test suite run (which will be run as an adhoc batch).")
	testSuiteCmd.AddCommand(runTestSuiteCmd)

	// Test Suite Batches
	// Project
	batchesTestSuiteCmd.Flags().String(testSuiteProjectKey, "", "The name or ID of the project the test suite is associated with.")
	batchesTestSuiteCmd.MarkFlagRequired(testSuiteProjectKey)
	// Name or ID
	batchesTestSuiteCmd.Flags().String(testSuiteKey, "", "The name or ID of the test suite to retrieve batches from.")
	batchesTestSuiteCmd.MarkFlagRequired(testSuiteKey)
	// Revision [Optional]
	batchesTestSuiteCmd.Flags().String(testSuiteRevisionKey, "", "The specific revision of a test suite to retrieve batches from.")
	testSuiteCmd.AddCommand(batchesTestSuiteCmd)
	rootCmd.AddCommand(testSuiteCmd)
}

func createTestSuite(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	testSuiteGithub := viper.GetBool(testSuiteGithubKey)
	if !testSuiteGithub {
		fmt.Println("Creating a test suite...")
	}

	// Parse the various arguments from command line
	suiteName := viper.GetString(testSuiteNameKey)
	if suiteName == "" {
		log.Fatal("empty test suite name")
	}

	suiteDescription := viper.GetString(testSuiteDescriptionKey)
	if suiteDescription == "" {
		log.Fatal("empty test suite description")
	}

	systemName := viper.GetString(testSuiteSystemKey)
	if systemName == "" {
		log.Fatal("empty system name")
	}
	systemID := getSystemID(Client, projectID, systemName, true)

	var allExperienceIDs []uuid.UUID
	var allExperienceNames []string

	if len(viper.GetString(testSuiteExperiencesKey)) == 0 {
		log.Fatal("empty list of experiences")
	}
	// Parse --experiences into either IDs or names
	if viper.IsSet(testSuiteExperiencesKey) {
		experienceIDs, experienceNames := parseUUIDsAndNames(viper.GetString(testSuiteExperiencesKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
		allExperienceNames = append(allExperienceNames, experienceNames...)
	}

	for _, experienceName := range allExperienceNames {
		experienceID := getExperienceID(Client, projectID, experienceName, true, false)
		allExperienceIDs = append(allExperienceIDs, experienceID)
	}

	metricsBuildID := uuid.Nil
	if viper.IsSet(testSuiteMetricsBuildKey) {
		var err error
		metricsBuildID, err = uuid.Parse(viper.GetString(testSuiteMetricsBuildKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
	}

	// Build the request body
	body := api.CreateTestSuiteInput{
		Name:        suiteName,
		Description: suiteDescription,
		SystemID:    systemID,
		Experiences: allExperienceIDs,
	}

	if metricsBuildID != uuid.Nil {
		body.MetricsBuildID = &metricsBuildID
	}

	if viper.IsSet(testSuiteShowOnSummaryKey) {
		body.ShowOnSummary = Ptr(viper.GetBool(testSuiteShowOnSummaryKey))
	}

	// Make the request
	response, err := Client.CreateTestSuiteWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create test suite:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create test suite", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	testSuite := *response.JSON201

	if !testSuiteGithub {
		// Report the results back to the user
		fmt.Println("Created test suite successfully!")
	}
	if testSuite.TestSuiteID == uuid.Nil {
		log.Fatal("empty ID")
	}
	if !testSuiteGithub {
		fmt.Println("test suite ID:", testSuite.TestSuiteID.String())
		fmt.Println("test suite Revision:", testSuite.TestSuiteRevision)
	} else {
		fmt.Printf("test_suite_id_revision=%v/%v\n", testSuite.TestSuiteID, testSuite.TestSuiteRevision)
	}
}

func reviseTestSuite(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	testSuiteGithub := viper.GetBool(testSuiteGithubKey)
	if !testSuiteGithub {
		fmt.Println("Revising a test suite...")
	}

	// Get the existing test suite name:
	existingTestSuite := actualGetTestSuite(projectID, viper.GetString(testSuiteKey), nil, false)
	if existingTestSuite == nil {
		log.Fatal("unable to find test suite")
	}

	reviseRequest := api.ReviseTestSuiteInput{}

	if viper.IsSet(testSuiteNameKey) {
		reviseRequest.Name = Ptr(viper.GetString(testSuiteNameKey))
	}

	if viper.IsSet(testSuiteDescriptionKey) {
		reviseRequest.Description = Ptr(viper.GetString(testSuiteDescriptionKey))
	}

	if viper.IsSet(testSuiteShowOnSummaryKey) {
		reviseRequest.ShowOnSummary = Ptr(viper.GetBool(testSuiteShowOnSummaryKey))
	}

	if viper.IsSet(testSuiteSystemKey) {
		systemID := getSystemID(Client, projectID, viper.GetString(testSuiteSystemKey), true)
		reviseRequest.SystemID = &systemID
	}

	if viper.IsSet(testSuiteMetricsBuildKey) {
		var metricsBuildID uuid.UUID
		var err error
		metricsBuildID, err = uuid.Parse(viper.GetString(testSuiteMetricsBuildKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
		if metricsBuildID != uuid.Nil {
			reviseRequest.MetricsBuildID = &metricsBuildID
			reviseRequest.UpdateMetricsBuild = true
		} else { // This has the effect of unsetting the metrics build
			reviseRequest.UpdateMetricsBuild = true
		}
	}

	// Parse --experiences into either IDs or names
	var allExperienceIDs []uuid.UUID
	var allExperienceNames []string
	if viper.IsSet(testSuiteExperiencesKey) {
		experienceIDs, experienceNames := parseUUIDsAndNames(viper.GetString(testSuiteExperiencesKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
		allExperienceNames = append(allExperienceNames, experienceNames...)
		for _, experienceName := range allExperienceNames {
			experienceID := getExperienceID(Client, projectID, experienceName, true, false)
			allExperienceIDs = append(allExperienceIDs, experienceID)
		}

		reviseRequest.Experiences = &allExperienceIDs
	}

	// Make the request
	response, err := Client.ReviseTestSuiteWithResponse(context.Background(), projectID, existingTestSuite.TestSuiteID, reviseRequest)
	if err != nil {
		log.Fatal("failed to revise test suite:", err)
	}
	ValidateResponse(http.StatusOK, "failed to revise test suite", response.HTTPResponse, response.Body)

	if response.JSON200 == nil {
		log.Fatal("empty response")
	}
	testSuite := *response.JSON200

	if !testSuiteGithub {
		// Report the results back to the user
		fmt.Println("Revised test suite successfully!")
	}
	if testSuite.TestSuiteID == uuid.Nil {
		log.Fatal("empty ID")
	}
	if !testSuiteGithub {
		fmt.Println("Test Suite ID:", testSuite.TestSuiteID.String())
		fmt.Println("Test Suite Revision:", testSuite.TestSuiteRevision)
	} else {
		fmt.Printf("test_suite_id_revision=%v/%v\n", testSuite.TestSuiteID, testSuite.TestSuiteRevision)
	}
}

func listTestSuites(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(branchProjectKey))

	var pageToken *string = nil

	allTestSuites := []api.TestSuite{}

	for {
		response, err := Client.ListTestSuitesWithResponse(
			context.Background(), projectID, &api.ListTestSuitesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list test suites:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list test suites", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.TestSuites == nil {
			break
		}
		allTestSuites = append(allTestSuites, response.JSON200.TestSuites...)
		if *pageToken == "" {
			break
		}
	}

	if len(allTestSuites) == 0 {
		fmt.Println("no test suites")
		return
	}

	OutputJson(allTestSuites)
}

func actualGetTestSuite(projectID uuid.UUID, testSuiteKeyRaw string, revision *int32, expectArchived bool) *api.TestSuite {
	var testSuite *api.TestSuite
	if testSuiteKeyRaw == "" {
		log.Fatal("must specify the test suite name or ID")
	}

	testSuiteID, err := uuid.Parse(testSuiteKeyRaw)
	if err == nil {
		response, err := Client.GetTestSuiteWithResponse(context.Background(), projectID, testSuiteID)
		if err != nil {
			log.Fatal("unable to retrieve test suite:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve test suite", response.HTTPResponse, response.Body)
		testSuite = response.JSON200
	} else { // it's a name, rather than an ID (and we disallow test suite names that are simply UUIDs)
		var pageToken *string = nil
	pageLoop:
		for {
			response, err := Client.ListTestSuitesWithResponse(context.Background(), projectID, &api.ListTestSuitesParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
				Archived:  Ptr(expectArchived),
			})
			if err != nil {
				log.Fatal("unable to list test suites:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list test suites", response.HTTPResponse, response.Body)
			if response.JSON200.TestSuites == nil {
				log.Fatal("unable to find test suite: ", testSuiteKeyRaw)
			}
			testSuites := response.JSON200.TestSuites

			for _, suite := range testSuites {
				if suite.Name == testSuiteKeyRaw {
					testSuite = &suite
					break pageLoop
				}
			}

			if response.JSON200.NextPageToken != "" {
				pageToken = &response.JSON200.NextPageToken
			} else {
				if expectArchived {
					log.Fatal("unable to find archived test suite: ", testSuiteKeyRaw)
				} else {
					log.Fatal("unable to find test suite: ", testSuiteKeyRaw)
				}
			}
		}
	}

	if testSuite != nil && revision != nil && *revision != testSuite.TestSuiteRevision {
		response, err := Client.GetTestSuiteRevisionWithResponse(context.Background(), projectID, testSuite.TestSuiteID, *revision)
		if err != nil {
			log.Fatal("unable to retrieve test suite revision:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve test suite revision", response.HTTPResponse, response.Body)
		testSuite = response.JSON200
	}
	return testSuite
}

func getTestSuite(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	var revision *int32
	if viper.IsSet(testSuiteRevisionKey) {
		revision = Ptr(viper.GetInt32(testSuiteRevisionKey))
	}
	testSuite := actualGetTestSuite(projectID, viper.GetString(testSuiteKey), revision, false)

	if viper.GetBool(testSuiteAllRevisionKey) {
		response, err := Client.ListTestSuiteRevisionsWithResponse(context.Background(), projectID, testSuite.TestSuiteID, &api.ListTestSuiteRevisionsParams{
			PageSize: Ptr(100),
		})
		if err != nil {
			log.Fatal("unable to list test suite revisions:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list test suite revisions", response.HTTPResponse, response.Body)
		if response.JSON200.TestSuites == nil {
			log.Fatal("unable to list test suite revisions")
		}
		OutputJson(response.JSON200.TestSuites)
	} else {
		OutputJson(testSuite)
	}
}

func archiveTestSuite(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	var revision *int32
	testSuite := actualGetTestSuite(projectID, viper.GetString(testSuiteKey), revision, false)

	response, err := Client.ArchiveTestSuiteWithResponse(context.Background(), projectID, testSuite.TestSuiteID)
	if err != nil {
		log.Fatal("failed to archive test suite:", err)
	}
	ValidateResponse(http.StatusNoContent, "failed to archive test suite", response.HTTPResponse, response.Body)
	fmt.Printf("Archived test suite %s successfully!\n", viper.GetString(testSuiteKey))
}

func restoreTestSuite(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	var revision *int32
	testSuite := actualGetTestSuite(projectID, viper.GetString(testSuiteKey), revision, true)

	response, err := Client.RestoreTestSuiteWithResponse(context.Background(), projectID, testSuite.TestSuiteID)
	if err != nil {
		log.Fatal("failed to restore test suite:", err)
	}
	ValidateResponse(http.StatusNoContent, "failed to restore test suite", response.HTTPResponse, response.Body)
	fmt.Printf("Restored archived test suite %s successfully!\n", viper.GetString(testSuiteKey))
}

func runTestSuite(ccmd *cobra.Command, args []string) {
	testSuiteGithub := viper.GetBool(testSuiteGithubKey)
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	var revision *int32
	if viper.IsSet(testSuiteRevisionKey) {
		revision = Ptr(viper.GetInt32(testSuiteRevisionKey))
	}
	testSuite := actualGetTestSuite(projectID, viper.GetString(testSuiteKey), revision, false)
	buildID, err := uuid.Parse(viper.GetString(testSuiteBuildIDKey))
	if err != nil {
		log.Fatal("failed to parse build ID: ", err)
	}

	// Parse --parameter (if any provided)
	parameters := api.BatchParameters{}
	if viper.IsSet(testSuiteParameterKey) {
		parameterStrings := viper.GetStringSlice(testSuiteParameterKey)
		for _, parameterString := range parameterStrings {
			key, value, err := ParseParameterString(parameterString)
			if err != nil {
				log.Fatal(err)
			}
			parameters[key] = value
		}
	}

	poolLabels := getAndValidatePoolLabels(testSuitePoolLabelsKey)

	// Process the associated account: by default, we try to get from CI/CD environment variables
	// Otherwise, we use the account flag. The default is "".
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(testSuiteAccountKey) {
		associatedAccount = viper.GetString(testSuiteAccountKey)
	}

	var batch api.Batch
	// If the user supplies a metrics build override, we run an adhoc batch:
	if viper.IsSet(testSuiteMetricsBuildOverrideKey) {
		metricsBuildID, err := uuid.Parse(viper.GetString(testSuiteMetricsBuildOverrideKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
		// Build the request body
		body := api.BatchInput{
			BuildID:           &buildID,
			MetricsBuildID:    &metricsBuildID,
			ExperienceIDs:     &testSuite.Experiences,
			Parameters:        &parameters,
			AssociatedAccount: &associatedAccount,
		}

		// Add the pool labels if any
		if len(poolLabels) > 0 {
			body.PoolLabels = &poolLabels
		}

		// Add the batch name if any
		if viper.IsSet(testSuiteBatchNameKey) {
			body.BatchName = Ptr(viper.GetString(testSuiteBatchNameKey))
		}

		// Parse --allowable-failure-percent (if any provided)
		if viper.IsSet(testSuiteAllowableFailurePercentKey) {
			allowableFailurePercent := viper.GetInt(testSuiteAllowableFailurePercentKey)
			if allowableFailurePercent < 0 || allowableFailurePercent > 100 {
				log.Fatal("allowable failure percent must be between 0 and 100")
			}
			body.AllowableFailurePercent = &allowableFailurePercent
		}

		response, err := Client.CreateBatchWithResponse(context.Background(), projectID, body)
		if err != nil {
			log.Fatal("failed to run test suite override:", err)
		}
		ValidateResponse(http.StatusCreated, "failed to run test suite override", response.HTTPResponse, response.Body)

		if response.JSON201 == nil {
			log.Fatal("empty response")
		}
		batch = *response.JSON201
	} else {
		// Build the request body
		body := api.TestSuiteBatchInput{
			BuildID:           buildID,
			Parameters:        &parameters,
			AssociatedAccount: &associatedAccount,
		}

		// Add the pool labels if any
		if len(poolLabels) > 0 {
			body.PoolLabels = &poolLabels
		}

		// Add the batch name if any
		if viper.IsSet(testSuiteBatchNameKey) {
			body.BatchName = Ptr(viper.GetString(testSuiteBatchNameKey))
		}

		// Parse --allowable-failure-percent (if any provided)
		if viper.IsSet(testSuiteAllowableFailurePercentKey) {
			allowableFailurePercent := viper.GetInt(testSuiteAllowableFailurePercentKey)
			if allowableFailurePercent < 0 || allowableFailurePercent > 100 {
				log.Fatal("allowable failure percent must be between 0 and 100")
			}
			body.AllowableFailurePercent = &allowableFailurePercent
		}

		// Make the request
		response, err := Client.CreateBatchForTestSuiteRevisionWithResponse(context.Background(), projectID, testSuite.TestSuiteID, testSuite.TestSuiteRevision, body)
		if err != nil {
			log.Fatal("failed to run test suite:", err)
		}
		ValidateResponse(http.StatusCreated, "failed to run test suite", response.HTTPResponse, response.Body)

		if response.JSON201 == nil {
			log.Fatal("empty response")
		}
		batch = *response.JSON201
	}

	if !testSuiteGithub {
		// Report the results back to the user
		fmt.Println("Created batch for test suite successfully!")
	}
	if batch.BatchID == nil {
		log.Fatal("empty ID")
	}
	if !testSuiteGithub {
		fmt.Println("Batch ID:", batch.BatchID.String())
	} else {
		fmt.Printf("batch_id=%s\n", batch.BatchID.String())
	}
	if batch.FriendlyName == nil {
		log.Fatal("empty name")
	}
	if !testSuiteGithub {
		fmt.Println("Batch name:", *batch.FriendlyName)
	}
	if batch.Status == nil {
		log.Fatal("empty status")
	}
	if !testSuiteGithub {
		fmt.Println("Status:", *batch.Status)
	}
}

func batchesTestSuite(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(testSuiteProjectKey))
	var revision *int32
	if viper.IsSet(testSuiteRevisionKey) {
		revision = Ptr(viper.GetInt32(testSuiteRevisionKey))
	}

	testSuite := actualGetTestSuite(projectID, viper.GetString(testSuiteKey), revision, false)
	testSuiteID := testSuite.TestSuiteID

	batches := []api.Batch{}
	if revision == nil {
		// Now list the batches for all revisions
		var pageToken *string = nil
		for {
			response, err := Client.ListBatchesForTestSuiteWithResponse(context.Background(), projectID, testSuiteID, &api.ListBatchesForTestSuiteParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list batches for test suite:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list batches for test suite", response.HTTPResponse, response.Body)
			if response.JSON200.Batches == nil {
				log.Fatal("unable to list batches for test suite")
			}
			responseBatches := *response.JSON200.Batches
			batches = append(batches, responseBatches...)

			if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
				pageToken = response.JSON200.NextPageToken
			} else {
				break
			}
		}
	} else {
		// Now list the batches for one revision
		var pageToken *string = nil
		for {
			response, err := Client.ListBatchesForTestSuiteRevisionWithResponse(context.Background(), projectID, testSuiteID, *revision, &api.ListBatchesForTestSuiteRevisionParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list batches for test suite revision:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list batches for test suite revision", response.HTTPResponse, response.Body)
			if response.JSON200.Batches == nil {
				log.Fatal("unable to list batches for test suite revision")
			}
			responseBatches := *response.JSON200.Batches
			batches = append(batches, responseBatches...)

			if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
				pageToken = response.JSON200.NextPageToken
			} else {
				break
			}
		}
	}
	// Finally, output them!
	OutputJson(batches)
}
