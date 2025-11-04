//go:build end_to_end
// +build end_to_end

// The end-to-end test is meant to exercise every potential command of the CLI in its built form
// It's meant as a sanity check on deployed code.
// It is not intended to catch edge cases or weird interactions; that's the realm of unit testing.

// To run:
// RESIM_CLIENT_ID=<> RESIM_CLIENT_SECRET=<> CONFIG=staging go test -v -tags end_to_end ./testing
//
// See the README for more information on how to run the tests.

package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	compose_types "github.com/compose-spec/compose-go/v2/types"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Test Environment Variables
const (
	Deployment   string = "DEPLOYMENT"
	Config       string = "CONFIG"
	Dev          string = "dev"
	Staging      string = "staging"
	Prod         string = "prod"
	ClientID     string = "RESIM_CLIENT_ID"
	ClientSecret string = "RESIM_CLIENT_SECRET"
	username     string = "USERNAME"
	password     string = "PASSWORD"
)

// CLI Constants
const (
	TempDirSuffix string = "cli-test"
	CliName       string = "resim"
	ExpectNoError bool   = false
	ExpectError   bool   = true
)

var EmptySlice []string = []string{}
var AssociatedAccount = "github-user"

type CliConfig struct {
	AuthKeyProviderDomain string
	ApiEndpoint           string
	E2EBucket             string
}

var DevConfig = CliConfig{
	AuthKeyProviderDomain: "https://resim-dev.us.auth0.com/",
	ApiEndpoint:           "https://$DEPLOYMENT.api.dev.resim.io/v1/",
	E2EBucket:             "dev-$DEPLOYMENT-e2e",
}

var StagingConfig = CliConfig{
	AuthKeyProviderDomain: "https://resim-dev.us.auth0.com/",
	ApiEndpoint:           "https://api.resim.io/v1/",
	E2EBucket:             "rerun-staging-e2e",
}

var ProdConfig = CliConfig{
	AuthKeyProviderDomain: "https://resim.us.auth0.com/",
	ApiEndpoint:           "https://api.resim.ai/v1/",
	E2EBucket:             "resim-e2e",
}

type EndToEndTestHelper struct {
	CliPath string
	Config  CliConfig
}

var s = EndToEndTestHelper{}

type Flag struct {
	Name  string
	Value string
}

type CommandBuilder struct {
	Command string
	Flags   []Flag
}

type Output struct {
	StdOut string
	StdErr string
}

// CLI Input Bool Flags:
const (
	AutoCreateBranchTrue  bool = true
	AutoCreateBranchFalse bool = false
	GithubTrue            bool = true
	GithubFalse           bool = false
	ExitStatusTrue        bool = true
	ExitStatusFalse       bool = false
)

// CLI Output Messages. Perhaps overkill, but we validate that successful actions
// from the CLI (i.e. exit code 0) have an expected output that contains a given substring.
const (
	// Project Messages
	CreatedProject          string = "Created project"
	GithubCreatedProject    string = "project_id="
	FailedToCreateProject   string = "failed to create project"
	EmptyProjectName        string = "empty project name"
	EmptyProjectDescription string = "empty project description"
	FailedToFindProject     string = "failed to find project"
	ArchivedProject         string = "Archived project"
	ProjectNameCollision    string = "project name matches an existing"
	// Branch Messages
	CreatedBranch       string = "Created branch"
	GithubCreatedBranch string = "branch_id="
	EmptyBranchName     string = "empty branch name"
	EmptyProjectID      string = "empty project ID"
	InvalidBranchType   string = "invalid branch type"
	// System Message
	CreatedSystem             string = "Created system"
	GithubCreatedSystem       string = "system_id="
	ArchivedSystem            string = "Archived system"
	UpdatedSystem             string = "Updated system"
	EmptySystemName           string = "empty system name"
	EmptySystemDescription    string = "empty system description"
	SystemAlreadyRegistered   string = "it may already be registered"
	SystemAlreadyDeregistered string = "it may not be registered"
	// Build Messages
	CreatedBuild                string = "Created build"
	GithubCreatedBuild          string = "build_id="
	EmptyBuildName              string = "empty build name"
	EmptyBuildDescription       string = "empty build description"
	EmptyBuildSpecAndBuildImage string = "either --build-spec or --image is required"
	InvalidBuildImage           string = "failed to parse the image URI"
	EmptyBuildVersion           string = "empty build version"
	EmptySystem                 string = "system not supplied"
	SystemDoesNotExist          string = "failed to find system"
	BranchNotExist              string = "Branch does not exist"
	UpdatedBuild                string = "Updated build"
	// Metrics Build Messages
	CreatedMetricsBuild       string = "Created metrics build"
	GithubCreatedMetricsBuild string = "metrics_build_id="
	EmptyMetricsBuildName     string = "empty metrics build name"
	EmptyMetricsBuildImage    string = "empty metrics build image URI"
	InvalidMetricsBuildImage  string = "failed to parse the image URI"
	EmptyMetricsBuildVersion  string = "empty metrics build version"
	// Experience Messages
	CreatedExperience          string = "Created experience"
	GithubCreatedExperience    string = "experience_id="
	EmptyExperienceName        string = "empty experience name"
	EmptyExperienceDescription string = "empty experience description"
	EmptyExperienceLocation    string = "empty experience locations"
	DeprecatedLaunchProfile    string = "launch profiles are deprecated"
	ArchivedExperience         string = "Archived experience"
	// Batch Messages
	CreatedBatch               string = "Created batch"
	GithubCreatedBatch         string = "batch_id="
	FailedToCreateBatch        string = "failed to create batch"
	InvalidBuildID             string = "failed to parse build ID"
	BranchTagMutuallyExclusive string = "mutually exclusive parameters"
	InvalidBatchName           string = "unable to find batch"
	InvalidBatchID             string = "unable to parse batch ID"
	SelectOneRequired          string = "at least one of the flags in the group"
	RequireBatchName           string = "must specify either the batch ID or the batch name"
	CancelledBatch             string = "Batch cancelled"
	// Log Messages
	CreatedLog            string = "Created log"
	GithubCreatedLog      string = "log_location="
	EmptyLogFileName      string = "empty log file name"
	EmptyLogChecksum      string = "No checksum was provided"
	EmptyLogBatchID       string = "empty batch ID"
	EmptyLogTestID        string = "empty test ID"
	EmptyLogType          string = "invalid log type"
	EmptyLogExecutionStep string = "invalid execution step"
	InvalidTestID         string = "unable to parse test ID"
	// Sweep Messages
	CreatedSweep                  string = "Created sweep"
	GithubCreatedSweep            string = "sweep_id="
	FailedToCreateSweep           string = "failed to create sweep"
	ConfigParamsMutuallyExclusive string = "if any flags in the group"
	InvalidSweepNameOrID          string = "must specify either the sweep ID or the sweep name"
	InvalidGridSearchFile         string = "failed to parse grid search config file"
	CancelledSweep                string = "Sweep cancelled"
	// Test Suite Messages
	CreatedTestSuite           string = "Created test suite"
	GithubCreatedTestSuite     string = "test_suite_id_revision="
	EmptyTestSuiteName         string = "empty test suite name"
	EmptyTestSuiteDescription  string = "empty test suite description"
	EmptyTestSuiteSystemName   string = "empty system name"
	EmptyTestSuiteMetricsBuild string = "failed to parse metrics-build"
	EmptyTestSuiteExperiences  string = "empty list of experiences"
	RevisedTestSuite           string = "Revised test suite"
	CreatedTestSuiteBatch      string = "Created batch for test suite"
	AllowableFailurePercent    string = "allowable failure percent must be between 0 and 100"
	// Report Messages
	CreatedReport                   string = "Created report"
	TestSuiteNameReport             string = "must specify the test suite name or ID"
	EndTimestamp                    string = "End timestamp"
	StartTimestamp                  string = "Start timestamp"
	FailedStartTimestamp            string = "failed to parse start timestamp"
	FailedEndTimestamp              string = "failed to parse end timestamp"
	GithubCreatedReport             string = "report_id="
	FailedToCreateReport            string = "failed to create report"
	EndLengthMutuallyExclusive      string = "none of the others can be"
	InvalidReportName               string = "unable to find report"
	InvalidReportID                 string = "unable to parse report ID"
	AtLeastOneReport                string = "at least one of the flags in the group"
	BranchNotFoundReport            string = "not found"
	FailedToParseMetricsBuildReport string = "failed to parse metrics-build ID"
	// Workflow Messages
	CreatedWorkflow             string = "Created workflow successfully!"
	UpdatedWorkflow             string = "Updated workflow successfully!"
	CreatedWorkflowRun          string = "Created workflow run successfully!"
	// Log Ingest Messages
	LogIngested string = "Ingested log successfully!"
)

var AcceptableBatchStatusCodes = [...]int{0, 2, 3, 4, 5}
var AcceptableSweepStatusCodes = [...]int{0, 2, 3, 4, 5}

func (s *EndToEndTestHelper) TearDownHelper() {
	os.Remove(fmt.Sprintf("%s/%s", s.CliPath, CliName))
	os.Remove(s.CliPath)
}

func (s *EndToEndTestHelper) SetupHelper() {
	switch viper.GetString(Config) {
	case Dev:
		s.Config = DevConfig
		deployment := viper.GetString(Deployment)
		if deployment == "" {
			fmt.Fprintf(os.Stderr, "error: must set %v for dev", Deployment)
			os.Exit(1)
		}
		s.Config.ApiEndpoint = strings.Replace(s.Config.ApiEndpoint, "$DEPLOYMENT", deployment, 1)
		s.Config.E2EBucket = strings.Replace(s.Config.E2EBucket, "$DEPLOYMENT", deployment, 1)
	case Staging:
		s.Config = StagingConfig
	case Prod:
		s.Config = ProdConfig
	default:
		fmt.Fprintf(os.Stderr, "error: invalid value for %s: %s", Config, viper.GetString(Config))
		os.Exit(1)
	}
	s.CliPath = s.buildCLI()
}

func (s *EndToEndTestHelper) buildCLI() string {
	fmt.Println("Building CLI")
	tmpDir, err := os.MkdirTemp("", TempDirSuffix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create temp directory: %v", err)
		os.Exit(1)
	}
	outputPath := filepath.Join(tmpDir, CliName)
	buildCmd := exec.Command("go", "build", "-o", outputPath, "../cmd/resim")
	err = buildCmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to build CLI: %v", err)
		os.Exit(1)
	}
	fmt.Println("Successfully built CLI")
	return tmpDir
}

func foldFlags(flags []Flag) []string {
	flagsSlice := []string{}
	for _, flag := range flags {
		flagsSlice = append(flagsSlice, flag.Name)
		flagsSlice = append(flagsSlice, flag.Value)
	}
	return flagsSlice
}

func (s *EndToEndTestHelper) buildCommand(commandBuilders []CommandBuilder) *exec.Cmd {
	// We populate the URL and the auth URL flags initially, then for each command/flags pair
	// in the command builder, we append to a single flattened slice:
	allCommands := []string{"--url", s.Config.ApiEndpoint}
	allCommands = append(allCommands, []string{"--auth-url", s.Config.AuthKeyProviderDomain}...)
	for _, commandBuilder := range commandBuilders {
		allCommands = append(allCommands, commandBuilder.Command)
		for _, flag := range foldFlags(commandBuilder.Flags) {
			allCommands = append(allCommands, flag)
		}
	}
	return exec.Command(fmt.Sprintf("%s/%s", s.CliPath, CliName), allCommands...)
}

func (s *EndToEndTestHelper) runCommand(ts *assert.Assertions, commandBuilders []CommandBuilder, expectError bool) Output {
	var stdout, stderr bytes.Buffer
	// Remove username/password flags inline before building the command and use env instead
	var username, password string
	for i := range commandBuilders {
		filtered := commandBuilders[i].Flags[:0] // reuse underlying array
		for _, f := range commandBuilders[i].Flags {
			switch f.Name {
			case "--username":
				username = f.Value
			case "--password":
				password = f.Value
			default:
				filtered = append(filtered, f)
			}
		}
		commandBuilders[i].Flags = filtered
	}
	cmd := s.buildCommand(commandBuilders)
	cmdString := cmd.String()
	fmt.Println("About to run command: ", cmdString)

	env := os.Environ()
	// If username/password are set, strip out client creds
	if username != "" && password != "" {
		newEnv := []string{}
		for _, kv := range env {
			if strings.HasPrefix(kv, "RESIM_CLIENT_ID=") || strings.HasPrefix(kv, "RESIM_CLIENT_SECRET=") {
				continue // skip these
			}
			newEnv = append(newEnv, kv)
		}
		newEnv = append(newEnv, fmt.Sprintf("RESIM_USERNAME=%s", username))
		newEnv = append(newEnv, fmt.Sprintf("RESIM_PASSWORD=%s", password))
		env = newEnv
	}
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	stdErrString := stderr.String()
	if expectError {
		ts.Error(err)
	} else {
		ts.NoError(err, fmt.Sprintf("Unexpected error: %v", stdErrString))
	}
	return Output{
		StdOut: stdout.String(),
		StdErr: stdErrString,
	}
}

func syncMetrics(projectName string, verbose bool, username string, password string) []CommandBuilder {
	metricsCommand := CommandBuilder{Command: "metrics"}

	flags := []Flag{
		{Name: "--project", Value: projectName},
	}
	if verbose {
		flags = append(flags, Flag{Name: "--verbose"})
	}
	// The CI fails if we use a different authentication method since it will
	// report a different auth0 id for the user. The bff api is expecting the
	// auth0 id reported with username/password auth. This also matches the
	// rerun CI setup
	if username != "" {
		flags = append(flags, Flag{Name: "--username", Value: username})
	}
	if password != "" {
		flags = append(flags, Flag{Name: "--password", Value: password})
	}

	syncCommand := CommandBuilder{Command: "sync", Flags: flags}

	return []CommandBuilder{metricsCommand, syncCommand}
}

func createProject(projectName string, description string, github bool) []CommandBuilder {
	// We build a create project command with the name and description flags
	projectCommand := CommandBuilder{
		Command: "projects",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
			{
				Name:  "--description",
				Value: description,
			},
		},
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{projectCommand, createCommand}
}

func listProjects() []CommandBuilder {
	projectCommand := CommandBuilder{
		Command: "project", // Implicitly testing alias to old singular noun
	}
	listCommand := CommandBuilder{
		Command: "list",
	}
	return []CommandBuilder{projectCommand, listCommand}
}

func getProject(projectName string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "projects",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectName,
			},
		},
	}
	return []CommandBuilder{projectCommand, getCommand}
}

func archiveProject(projectName string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "projects",
	}
	archiveCommand := CommandBuilder{
		Command: "archive",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
		},
	}
	return []CommandBuilder{projectCommand, archiveCommand}
}

func createBranch(projectID uuid.UUID, name string, branchType string, github bool) []CommandBuilder {
	branchCommand := CommandBuilder{
		Command: "branches",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: name,
			},
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--type",
				Value: branchType,
			},
		},
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{branchCommand, createCommand}
}

func listBranches(projectID uuid.UUID) []CommandBuilder {
	branchCommand := CommandBuilder{
		Command: "branch", // Implicitly testing old singular noun
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	return []CommandBuilder{branchCommand, listCommand}
}

func createSystem(projectName string, systemName string, systemDescription string, buildVCPUs *int, buildGPUs *int, buildMemoryMiB *int, buildSharedMemoryMB *int, metricsBuildVCPUs *int, metricsBuildGPUs *int, metricsBuildMemoryMiB *int, metricsBuildSharedMemoryMB *int, architecture *string, github bool) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "systems",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectName,
			},
			{
				Name:  "--name",
				Value: systemName,
			},
			{
				Name:  "--description",
				Value: systemDescription,
			},
		},
	}
	if buildVCPUs != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--build-vcpus",
			Value: fmt.Sprintf("%d", *buildVCPUs),
		})
	}
	if buildGPUs != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--build-gpus",
			Value: fmt.Sprintf("%d", *buildGPUs),
		})
	}
	if buildMemoryMiB != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--build-memory-mib",
			Value: fmt.Sprintf("%d", *buildMemoryMiB),
		})
	}
	if buildSharedMemoryMB != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--build-shared-memory-mb",
			Value: fmt.Sprintf("%d", *buildSharedMemoryMB),
		})
	}
	if metricsBuildVCPUs != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build-vcpus",
			Value: fmt.Sprintf("%d", *metricsBuildVCPUs),
		})
	}
	if metricsBuildGPUs != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build-gpus",
			Value: fmt.Sprintf("%d", *metricsBuildGPUs),
		})
	}
	if metricsBuildMemoryMiB != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build-memory-mib",
			Value: fmt.Sprintf("%d", *metricsBuildMemoryMiB),
		})
	}
	if metricsBuildSharedMemoryMB != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build-shared-memory-mb",
			Value: fmt.Sprintf("%d", *metricsBuildSharedMemoryMB),
		})
	}
	if architecture != nil && *architecture != "" {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--architecture",
			Value: *architecture,
		})
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{systemCommand, createCommand}
}

func updateSystem(projectName string, existingSystemName string, newName *string, systemDescription *string, buildVCPUs *int, buildGPUs *int, buildMemoryMiB *int, buildSharedMemoryMB *int, metricsBuildVCPUs *int, metricsBuildGPUs *int, metricsBuildMemoryMiB *int, metricsBuildSharedMemoryMB *int) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "systems",
	}
	updateCommand := CommandBuilder{
		Command: "update",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectName,
			},
			{
				Name:  "--system",
				Value: existingSystemName,
			},
		},
	}
	if newName != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--name",
			Value: *newName,
		})
	}
	if systemDescription != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--description",
			Value: *systemDescription,
		})
	}
	if buildVCPUs != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--build-vcpus",
			Value: fmt.Sprintf("%d", *buildVCPUs),
		})
	}
	if buildGPUs != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--build-gpus",
			Value: fmt.Sprintf("%d", *buildGPUs),
		})
	}
	if buildMemoryMiB != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--build-memory-mib",
			Value: fmt.Sprintf("%d", *buildMemoryMiB),
		})
	}
	if buildSharedMemoryMB != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--build-shared-memory-mb",
			Value: fmt.Sprintf("%d", *buildSharedMemoryMB),
		})
	}
	if metricsBuildVCPUs != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--metrics-build-vcpus",
			Value: fmt.Sprintf("%d", *metricsBuildVCPUs),
		})
	}
	if metricsBuildGPUs != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--metrics-build-gpus",
			Value: fmt.Sprintf("%d", *metricsBuildGPUs),
		})
	}
	if metricsBuildMemoryMiB != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--metrics-build-memory-mib",
			Value: fmt.Sprintf("%d", *metricsBuildMemoryMiB),
		})
	}
	if metricsBuildSharedMemoryMB != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--metrics-build-shared-memory-mb",
			Value: fmt.Sprintf("%d", *metricsBuildSharedMemoryMB),
		})
	}
	return []CommandBuilder{systemCommand, updateCommand}
}

func listSystems(projectID uuid.UUID) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "system", // Implicitly testing singular noun alias
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	return []CommandBuilder{systemCommand, listCommand}
}

func getSystem(project string, system string) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "system", // Implicitly testing singular noun alias
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
		},
	}
	return []CommandBuilder{systemCommand, getCommand}
}
func archiveSystem(project string, system string) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "system",
	}
	archiveCommand := CommandBuilder{
		Command: "archive",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
		},
	}
	return []CommandBuilder{systemCommand, archiveCommand}
}

func systemBuilds(project string, system string) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "system", // Implicitly testing singular noun alias
	}
	buildsCommand := CommandBuilder{
		Command: "builds",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
		},
	}
	return []CommandBuilder{systemCommand, buildsCommand}
}

func addSystemToExperience(project string, system string, experience string) []CommandBuilder {
	addExperienceCommand := CommandBuilder{
		Command: "experience",
	}
	addCommand := CommandBuilder{
		Command: "add-system",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
			{
				Name:  "--experience",
				Value: experience,
			},
		},
	}
	return []CommandBuilder{addExperienceCommand, addCommand}
}

func removeSystemFromExperience(project string, system string, experience string) []CommandBuilder {
	removeExperienceCommand := CommandBuilder{
		Command: "experience",
	}
	removeCommand := CommandBuilder{
		Command: "remove-system",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
			{
				Name:  "--experience",
				Value: experience,
			},
		},
	}
	return []CommandBuilder{removeExperienceCommand, removeCommand}
}

func systemExperiences(project string, system string) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "system", // Implicitly testing singular noun alias
	}
	experiencesCommand := CommandBuilder{
		Command: "experiences",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
		},
	}
	return []CommandBuilder{systemCommand, experiencesCommand}
}

func addSystemToMetricsBuild(project string, system string, metricsBuildID string) []CommandBuilder {
	addMetricsBuildCommand := CommandBuilder{
		Command: "metrics-build",
	}
	addCommand := CommandBuilder{
		Command: "add-system",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
			{
				Name:  "--metrics-build-id",
				Value: metricsBuildID,
			},
		},
	}
	return []CommandBuilder{addMetricsBuildCommand, addCommand}
}
func removeSystemFromMetricsBuild(project string, system string, metricsBuildID string) []CommandBuilder {
	removeMetricsBuildCommand := CommandBuilder{
		Command: "metrics-build",
	}
	removeCommand := CommandBuilder{
		Command: "remove-system",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
			{
				Name:  "--metrics-build-id",
				Value: metricsBuildID,
			},
		},
	}
	return []CommandBuilder{removeMetricsBuildCommand, removeCommand}
}

func systemMetricsBuilds(project string, system string) []CommandBuilder {
	systemCommand := CommandBuilder{
		Command: "system", // Implicitly testing singular noun alias
	}
	metricsBuildsCommand := CommandBuilder{
		Command: "metrics-builds",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: project,
			},
			{
				Name:  "--system",
				Value: system,
			},
		},
	}
	return []CommandBuilder{systemCommand, metricsBuildsCommand}
}

func createBuild(projectName string, branchName string, systemName string, description string, image string, buildSpecLocations []string, version string, github bool, autoCreateBranch bool) []CommandBuilder {
	// Now create the build:
	buildCommand := CommandBuilder{
		Command: "builds",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectName,
			},
			{
				Name:  "--branch",
				Value: branchName,
			},
			{
				Name:  "--system",
				Value: systemName,
			},
			{
				Name:  "--description",
				Value: description,
			},
			{
				Name:  "--version",
				Value: version,
			},
		},
	}

	if len(buildSpecLocations) > 0 {
		for _, buildSpecLocation := range buildSpecLocations {
			createCommand.Flags = append(createCommand.Flags, Flag{
				Name:  "--build-spec",
				Value: buildSpecLocation,
			})
		}
	}
	if image != "" {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--image",
			Value: image,
		})
	}

	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	if autoCreateBranch {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--auto-create-branch",
			Value: "",
		})
	}
	return []CommandBuilder{buildCommand, createCommand}
}

func updateBuild(projectName string, existingBuildID uuid.UUID, newBranchID *uuid.UUID, buildDescription *string) []CommandBuilder {
	buildCommand := CommandBuilder{
		Command: "builds",
	}
	updateCommand := CommandBuilder{
		Command: "update",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectName,
			},
			{
				Name:  "--build-id",
				Value: existingBuildID.String(),
			},
		},
	}
	if newBranchID != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--branch-id",
			Value: (*newBranchID).String(),
		})
	}
	if buildDescription != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--description",
			Value: *buildDescription,
		})
	}
	return []CommandBuilder{buildCommand, updateCommand}
}

func getBuild(projectName string, existingBuildID uuid.UUID, showBuildSpec bool) []CommandBuilder {
	buildCommand := CommandBuilder{
		Command: "builds",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectName,
			},
			{
				Name:  "--build-id",
				Value: existingBuildID.String(),
			},
		},
	}
	if showBuildSpec {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--show-build-spec-only",
			Value: "true",
		})
	}
	return []CommandBuilder{buildCommand, getCommand}
}

func listBuilds(projectID uuid.UUID, branchName *string, systemName *string) []CommandBuilder {
	buildCommand := CommandBuilder{
		Command: "build", // Implicitly testing singular noun alias
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	if branchName != nil {
		listCommand.Flags = append(listCommand.Flags, Flag{
			Name:  "--branch",
			Value: *branchName,
		})
	}
	if systemName != nil {
		listCommand.Flags = append(listCommand.Flags, Flag{
			Name:  "--system",
			Value: *systemName,
		})
	}
	return []CommandBuilder{buildCommand, listCommand}
}

func createMetricsBuild(projectID uuid.UUID, name string, image string, version string, systems []string, github bool) []CommandBuilder {
	// Now create the metrics build:
	metricsBuildCommand := CommandBuilder{
		Command: "metrics-builds",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--name",
				Value: name,
			},
			{
				Name:  "--image",
				Value: image,
			},
			{
				Name:  "--version",
				Value: version,
			},
		},
	}
	for _, system := range systems {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--systems",
			Value: system,
		})
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{metricsBuildCommand, createCommand}
}

func listMetricsBuilds(projectID uuid.UUID) []CommandBuilder {
	metricsBuildsCommand := CommandBuilder{
		Command: "metrics-builds",
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	return []CommandBuilder{metricsBuildsCommand, listCommand}
}

func createExperience(projectID uuid.UUID, name string, description string, location string, systems []string, tags []string, timeout *time.Duration, profile *string, envVars []string, github bool) []CommandBuilder {
	// We build a create experience command with the name, description, location flags
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--name",
				Value: name,
			},
			{
				Name:  "--description",
				Value: description,
			},
			{
				Name:  "--locations",
				Value: location,
			},
		},
	}
	for _, system := range systems {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--systems",
			Value: system,
		})
	}
	for _, tag := range tags {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--tags",
			Value: tag,
		})
	}
	if timeout != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--timeout",
			Value: timeout.String(),
		})
	}
	if profile != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--profile",
			Value: *profile,
		})
	}
	for _, envVar := range envVars {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--environment-variable",
			Value: envVar,
		})
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{experienceCommand, createCommand}
}

func archiveExperience(projectID uuid.UUID, experienceKey string) []CommandBuilder {
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	archiveCommand := CommandBuilder{
		Command: "archive",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--experience",
				Value: experienceKey,
			},
		},
	}
	return []CommandBuilder{experienceCommand, archiveCommand}
}

func restoreExperience(projectID uuid.UUID, experienceKey string) []CommandBuilder {
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	restoreCommand := CommandBuilder{
		Command: "restore",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--experience",
				Value: experienceKey,
			},
		},
	}
	return []CommandBuilder{experienceCommand, restoreCommand}
}

func getExperience(projectID uuid.UUID, experienceKey string) []CommandBuilder {
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--experience",
				Value: experienceKey,
			},
		},
	}
	return []CommandBuilder{experienceCommand, getCommand}
}

func listExperiences(projectID uuid.UUID) []CommandBuilder {
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	return []CommandBuilder{experienceCommand, listCommand}
}

func updateExperience(projectID uuid.UUID, experienceKey string, name *string, description *string, location *string, systems []string, tags []string, timeout *time.Duration, profile *string, envVars []string) []CommandBuilder {
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	updateCommand := CommandBuilder{
		Command: "update",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--experience",
				Value: experienceKey,
			},
		},
	}
	if name != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--name",
			Value: *name,
		})
	}
	if description != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--description",
			Value: *description,
		})
	}
	if location != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--location",
			Value: *location,
		})
	}
	for _, system := range systems {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--systems",
			Value: system,
		})
	}
	for _, tag := range tags {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--tags",
			Value: tag,
		})
	}
	if timeout != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--timeout",
			Value: timeout.String(),
		})
	}
	if profile != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--profile",
			Value: *profile,
		})
	}
	for _, envVar := range envVars {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--environment-variable",
			Value: envVar,
		})
	}
	return []CommandBuilder{experienceCommand, updateCommand}
}

func createExperienceTag(projectID uuid.UUID, name string, description string) []CommandBuilder {
	experienceTagCommand := CommandBuilder{
		Command: "experience-tags",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--name",
				Value: name,
			},
			{
				Name:  "--description",
				Value: description,
			},
		},
	}

	return []CommandBuilder{experienceTagCommand, createCommand}
}

func listExperienceTags(projectID uuid.UUID) []CommandBuilder {
	experienceTagCommand := CommandBuilder{
		Command: "experience-tags",
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}

	return []CommandBuilder{experienceTagCommand, listCommand}
}
func tagExperience(projectID uuid.UUID, tag string, experienceID uuid.UUID) []CommandBuilder {
	tagExperienceCommand := CommandBuilder{
		Command: "experience",
	}
	tagCommand := CommandBuilder{
		Command: "tag",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--tag",
				Value: tag,
			},
			{
				Name:  "--id",
				Value: experienceID.String(),
			},
		},
	}
	return []CommandBuilder{tagExperienceCommand, tagCommand}
}

func untagExperience(projectID uuid.UUID, tag string, experienceID uuid.UUID) []CommandBuilder {
	untagExperienceCommand := CommandBuilder{
		Command: "experience",
	}
	untagCommand := CommandBuilder{
		Command: "untag",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--tag",
				Value: tag,
			},
			{
				Name:  "--id",
				Value: experienceID.String(),
			},
		},
	}
	return []CommandBuilder{untagExperienceCommand, untagCommand}
}

func listExperiencesWithTag(projectID uuid.UUID, tag string) []CommandBuilder {
	listExperiencesWithTagCommand := CommandBuilder{
		Command: "experience-tag",
	}
	listCommand := CommandBuilder{
		Command: "list-experiences",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--name",
				Value: tag,
			},
		},
	}
	return []CommandBuilder{listExperiencesWithTagCommand, listCommand}
}

func debugCommand(projectID uuid.UUID, buildID string, experienceName string) []CommandBuilder {
	debugCommand := CommandBuilder{
		Command: "debug",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--build",
				Value: buildID,
			},
			{
				Name:  "--experience",
				Value: experienceName,
			},
			{
				Name:  "--command",
				Value: "echo testing",
			},
		},
	}

	// this is used in debug.go to skip setting raw mode
	os.Setenv("RESIM_TEST", "true")

	return []CommandBuilder{debugCommand}
}

func createBatch(projectID uuid.UUID, buildID string, experienceIDs []string, experienceTagIDs []string, experienceTagNames []string, experiences []string, experienceTags []string, metricsBuildID string, github bool, parameters map[string]string, account string, batchName *string, allowableFailurePercent *int) []CommandBuilder {
	// We build a create batch command with the build-id, experience-ids, experience-tag-ids, and experience-tag-names flags
	// We do not require any specific combination of these flags, and validate in tests that the CLI only allows one of TagIDs or TagNames
	// and that at least one of the experiences flags is provided.
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--build-id",
				Value: buildID,
			},
		},
	}
	if len(experienceIDs) > 0 {
		// Join experience ids with a ',' for CLI input
		experienceIDsString := strings.Join(experienceIDs, ", ")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experience-ids",
			Value: experienceIDsString,
		})
	}
	if len(experienceTagIDs) > 0 {
		experienceTagIDsString := strings.Join(experienceTagIDs, ", ")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experience-tag-ids",
			Value: experienceTagIDsString,
		})
	}
	if len(experienceTagNames) > 0 {
		experienceTags := strings.Join(experienceTagNames, ", ")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experience-tag-names",
			Value: experienceTags,
		})
	}
	if len(experiences) > 0 {
		experiencesString := strings.Join(experiences, ",")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experiences",
			Value: experiencesString,
		})
	}
	if len(experienceTags) > 0 {
		experienceTagsString := strings.Join(experienceTags, ",")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experience-tags",
			Value: experienceTagsString,
		})
	}
	if len(metricsBuildID) > 0 {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build-id",
			Value: metricsBuildID,
		})
	}
	if len(parameters) > 0 {
		for key, value := range parameters {
			createCommand.Flags = append(createCommand.Flags, Flag{
				Name:  "--parameter",
				Value: fmt.Sprintf("%s:%s", key, value),
			})
		}
	}
	if len(account) > 0 {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--account",
			Value: account,
		})
	}

	if batchName != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--batch-name",
			Value: *batchName,
		})
	}

	if allowableFailurePercent != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--allowable-failure-percent",
			Value: fmt.Sprintf("%d", *allowableFailurePercent),
		})
	}

	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{batchCommand, createCommand}
}

func createIngestedLog(projectID uuid.UUID, system *string, branchname *string, version *string, metricsBuildID uuid.UUID, logName *string, logLocation *string, logsList []string, configFileLocation *string, experienceTags []string, buildID *uuid.UUID, batchName *string, github bool, reingest bool) []CommandBuilder {
	ingestCommand := CommandBuilder{
		Command: "ingest",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--metrics-build-id",
				Value: metricsBuildID.String(),
			},
		},
	}
	if logName != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--log-name",
			Value: *logName,
		})
	}
	if logLocation != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--log-location",
			Value: *logLocation,
		})
	}
	if len(logsList) > 0 {
		logsListString := strings.Join(logsList, ",")
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--log",
			Value: logsListString,
		})
	}
	if configFileLocation != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--log-config",
			Value: *configFileLocation,
		})
	}

	if system != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--system",
			Value: *system,
		})
	}

	if branchname != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--branch",
			Value: *branchname,
		})
	}
	if version != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--version",
			Value: *version,
		})
	}
	if len(experienceTags) > 0 {
		experienceTagsString := strings.Join(experienceTags, ",")
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--tags",
			Value: experienceTagsString,
		})
	}
	if buildID != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--build-id",
			Value: buildID.String(),
		})
	}
	if batchName != nil {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--ingestion-name",
			Value: *batchName,
		})
	}
	if github {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	if reingest {
		ingestCommand.Flags = append(ingestCommand.Flags, Flag{
			Name:  "--reingest",
			Value: "",
		})
	}
	return []CommandBuilder{ingestCommand}
}
func getBatchByName(projectID uuid.UUID, batchName string, exitStatus bool) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-name",
				Value: batchName,
			},
		},
	}
	if exitStatus {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--exit-status",
			Value: "",
		})
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func getBatchByID(projectID uuid.UUID, batchID string, exitStatus bool) []CommandBuilder {
	// We build a get batch command with the id flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID,
			},
		},
	}
	if exitStatus {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--exit-status",
			Value: "",
		})
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func getBatchJobsByName(projectID uuid.UUID, batchName string) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "tests",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-name",
				Value: batchName,
			},
		},
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func cancelBatchByID(projectID uuid.UUID, batchID string) []CommandBuilder {
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	cancelCommand := CommandBuilder{
		Command: "cancel",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID,
			},
		},
	}
	return []CommandBuilder{batchCommand, cancelCommand}
}

func cancelBatchByName(projectID uuid.UUID, batchName string) []CommandBuilder {
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	cancelCommand := CommandBuilder{
		Command: "cancel",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-name",
				Value: batchName,
			},
		},
	}
	return []CommandBuilder{batchCommand, cancelCommand}
}

func cancelSweep(projectID uuid.UUID, sweepID string) []CommandBuilder {
	sweepCommand := CommandBuilder{
		Command: "sweeps",
	}
	cancelCommand := CommandBuilder{
		Command: "cancel",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--sweep-id",
				Value: sweepID,
			},
		},
	}
	return []CommandBuilder{sweepCommand, cancelCommand}
}

func getBatchJobsByID(projectID uuid.UUID, batchID string) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "tests",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID,
			},
		},
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func listBatchLogs(projectID uuid.UUID, batchID, batchName string) []CommandBuilder {
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	batchIdFlag := Flag{
		Name:  "--batch-id",
		Value: batchID,
	}
	batchNameFlag := Flag{
		Name:  "--batch-name",
		Value: batchName,
	}
	allFlags := []Flag{
		{
			Name:  "--project",
			Value: projectID.String(),
		},
	}
	// For ease of interface downstream, only set the flags if they were passed in to this method
	if batchID != "" {
		allFlags = append(allFlags, batchIdFlag)
	}
	if batchName != "" {
		allFlags = append(allFlags, batchNameFlag)
	}
	listLogsCommand := CommandBuilder{
		Command: "logs",
		Flags:   allFlags,
	}
	return []CommandBuilder{batchCommand, listLogsCommand}
}

func listLogs(projectID uuid.UUID, batchID string, testID string) []CommandBuilder {
	logCommand := CommandBuilder{
		Command: "log",
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID,
			},
			{
				Name:  "--test-id",
				Value: testID,
			},
		},
	}
	return []CommandBuilder{logCommand, listCommand}
}

func downloadLogs(projectID uuid.UUID, batchID string, testID string, outputDir string, files []string) []CommandBuilder {
	logCommand := CommandBuilder{
		Command: "log",
	}
	downloadCommand := CommandBuilder{
		Command: "download",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID,
			},
			{
				Name:  "--test-id",
				Value: testID,
			},
			{
				Name:  "--output",
				Value: outputDir,
			},
		},
	}
	if len(files) > 0 {
		filesString := strings.Join(files, ",")
		downloadCommand.Flags = append(downloadCommand.Flags, Flag{
			Name:  "--files",
			Value: filesString,
		})
	}
	return []CommandBuilder{logCommand, downloadCommand}
}

func createSweep(projectID uuid.UUID, buildID string, experiences []string, experienceTags []string, metricsBuildID string, parameterName string, parameterValues []string, configFileLocation string, github bool, account string) []CommandBuilder {
	// We build a create sweep command with the build-id, experiences, experience-tags, and metrics-build-id flags
	// We additionally require either the parameter-name and parameter-values flags, or the grid-search-config flag
	// We do not require any specific combination of these flags, and validate in tests that the CLI only allows one
	// of parameter or config files and that at least one of the experiences flags is provided.
	sweepCommand := CommandBuilder{
		Command: "sweeps",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--build-id",
				Value: buildID,
			},
		},
	}
	if len(experiences) > 0 {
		experiencesString := strings.Join(experiences, ",")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experiences",
			Value: experiencesString,
		})
	}
	if len(experienceTags) > 0 {
		experienceTagsString := strings.Join(experienceTags, ",")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--experience-tags",
			Value: experienceTagsString,
		})
	}
	if len(metricsBuildID) > 0 {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build-id",
			Value: metricsBuildID,
		})
	}
	if parameterName != "" {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--parameter-name",
			Value: parameterName,
		})
	}
	if len(parameterValues) > 0 {
		parameterValuesString := strings.Join(parameterValues, ",")
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--parameter-values",
			Value: parameterValuesString,
		})
	}
	if configFileLocation != "" {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--grid-search-config",
			Value: configFileLocation,
		})
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	if len(account) > 0 {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--account",
			Value: account,
		})
	}
	return []CommandBuilder{sweepCommand, createCommand}
}

func getSweepByName(projectID uuid.UUID, sweepName string, exitStatus bool) []CommandBuilder {
	// We build a get sweep command with the name flag
	sweepCommand := CommandBuilder{
		Command: "sweep",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--sweep-name",
				Value: sweepName,
			},
		},
	}
	if exitStatus {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--exit-status",
			Value: "",
		})
	}
	return []CommandBuilder{sweepCommand, getCommand}
}
func getSweepByID(projectID uuid.UUID, sweepID string, exitStatus bool) []CommandBuilder {
	// We build a get sweep command with the id flag
	batchCommand := CommandBuilder{
		Command: "sweeps",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--sweep-id",
				Value: sweepID,
			},
		},
	}
	if exitStatus {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--exit-status",
			Value: "",
		})
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func listSweeps(projectID uuid.UUID) []CommandBuilder {
	sweepCommand := CommandBuilder{
		Command: "sweep", // Implicitly testing alias to singular noun
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	return []CommandBuilder{sweepCommand, listCommand}
}

func createTestSuite(projectID uuid.UUID, name string, description string, systemID string, experiences []string, metricsBuildID string, github bool, metricsSet *string) []CommandBuilder {
	suitesCommand := CommandBuilder{
		Command: "suites",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--name",
				Value: name,
			},
			{
				Name:  "--description",
				Value: description,
			},
			{
				Name:  "--system",
				Value: systemID,
			},
		},
	}
	// Pass an experience flag even if empty
	experiencesString := strings.Join(experiences, ",")
	createCommand.Flags = append(createCommand.Flags, Flag{
		Name:  "--experiences",
		Value: experiencesString,
	})

	if len(metricsBuildID) > 0 {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-build",
			Value: metricsBuildID,
		})
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	if metricsSet != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--metrics-set",
			Value: *metricsSet,
		})
	}
	return []CommandBuilder{suitesCommand, createCommand}
}

// Workflow helpers
func createWorkflow(projectID uuid.UUID, name string, description string, suitesJSON string, suitesFile string) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    createCommand := CommandBuilder{
        Command: "create",
        Flags: []Flag{
            {Name: "--project", Value: projectID.String()},
            {Name: "--name", Value: name},
            {Name: "--description", Value: description},
        },
    }
    if suitesJSON != "" {
        createCommand.Flags = append(createCommand.Flags, Flag{Name: "--suites", Value: suitesJSON})
    }
    if suitesFile != "" {
        createCommand.Flags = append(createCommand.Flags, Flag{Name: "--suites-file", Value: suitesFile})
    }
    return []CommandBuilder{workflowsCommand, createCommand}
}

func updateWorkflowCmd(projectID uuid.UUID, workflowKey string, name *string, description *string, ciLink *string, suitesJSON *string, suitesFile *string) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    updateCommand := CommandBuilder{
        Command: "update",
        Flags: []Flag{
            {Name: "--project", Value: projectID.String()},
            {Name: "--workflow", Value: workflowKey},
        },
    }
    if name != nil {
        updateCommand.Flags = append(updateCommand.Flags, Flag{Name: "--name", Value: *name})
    }
    if description != nil {
        updateCommand.Flags = append(updateCommand.Flags, Flag{Name: "--description", Value: *description})
    }
    if ciLink != nil {
        updateCommand.Flags = append(updateCommand.Flags, Flag{Name: "--ci-link", Value: *ciLink})
    }
    if suitesJSON != nil {
        updateCommand.Flags = append(updateCommand.Flags, Flag{Name: "--suites", Value: *suitesJSON})
    }
    if suitesFile != nil {
        updateCommand.Flags = append(updateCommand.Flags, Flag{Name: "--suites-file", Value: *suitesFile})
    }
    return []CommandBuilder{workflowsCommand, updateCommand}
}

func listWorkflowsCmd(projectID uuid.UUID) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    listCommand := CommandBuilder{Command: "list", Flags: []Flag{{Name: "--project", Value: projectID.String()}}}
    return []CommandBuilder{workflowsCommand, listCommand}
}

func getWorkflowCmd(projectID uuid.UUID, workflowKey string) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    getCommand := CommandBuilder{Command: "get", Flags: []Flag{{Name: "--project", Value: projectID.String()}, {Name: "--workflow", Value: workflowKey}}}
    return []CommandBuilder{workflowsCommand, getCommand}
}

func runWorkflowCmd(projectID uuid.UUID, workflowKey string, buildID uuid.UUID, parameters map[string]string, poolLabels []string, associatedAccount string, allowableFailurePercent *int) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    runsCommand := CommandBuilder{Command: "runs"}
    createCommand := CommandBuilder{
        Command: "create",
        Flags: []Flag{
            {Name: "--project", Value: projectID.String()},
            {Name: "--workflow", Value: workflowKey},
            {Name: "--build-id", Value: buildID.String()},
            {Name: "--account", Value: associatedAccount},
        },
    }
    for key, val := range parameters {
        createCommand.Flags = append(createCommand.Flags, Flag{Name: "--parameter", Value: fmt.Sprintf("%s=%s", key, val)})
    }
    if len(poolLabels) > 0 {
        createCommand.Flags = append(createCommand.Flags, Flag{Name: "--pool-labels", Value: strings.Join(poolLabels, ",")})
    }
    if allowableFailurePercent != nil {
        createCommand.Flags = append(createCommand.Flags, Flag{Name: "--allowable-failure-percent", Value: fmt.Sprintf("%d", *allowableFailurePercent)})
    }
    return []CommandBuilder{workflowsCommand, runsCommand, createCommand}
}

func listWorkflowRunsCmd(projectID uuid.UUID, workflowKey string) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    runsCommand := CommandBuilder{Command: "runs"}
    listCommand := CommandBuilder{Command: "list", Flags: []Flag{{Name: "--project", Value: projectID.String()}, {Name: "--workflow", Value: workflowKey}}}
    return []CommandBuilder{workflowsCommand, runsCommand, listCommand}
}

func getWorkflowRunCmd(projectID uuid.UUID, workflowKey string, runID uuid.UUID) []CommandBuilder {
    workflowsCommand := CommandBuilder{Command: "workflows"}
    runsCommand := CommandBuilder{Command: "runs"}
    getCommand := CommandBuilder{Command: "get", Flags: []Flag{{Name: "--project", Value: projectID.String()}, {Name: "--workflow", Value: workflowKey}, {Name: "--run-id", Value: runID.String()}}}
    return []CommandBuilder{workflowsCommand, runsCommand, getCommand}
}

func reviseTestSuite(projectID uuid.UUID, testSuite string, name *string, description *string, systemID *string, experiences *[]string, metricsBuildID *string, showOnSummary *bool, metricsSetName *string, github bool) []CommandBuilder {
	suitesCommand := CommandBuilder{
		Command: "suites",
	}
	reviseCommand := CommandBuilder{
		Command: "revise",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuite,
			},
		},
	}
	if systemID != nil {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--system",
			Value: *systemID,
		})
	}
	if name != nil {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--name",
			Value: *name,
		})
	}
	if description != nil {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--description",
			Value: *description,
		})
	}
	if showOnSummary != nil {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--show-on-summary",
			Value: fmt.Sprintf("%t", *showOnSummary),
		})
	}
	if experiences != nil && len(*experiences) > 0 {
		experiencesString := strings.Join(*experiences, ",")
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--experiences",
			Value: experiencesString,
		})
	}
	if metricsBuildID != nil {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--metrics-build-id",
			Value: *metricsBuildID,
		})
	}
	if metricsSetName != nil {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--metrics-set",
			Value: *metricsSetName,
		})
	}
	if github {
		reviseCommand.Flags = append(reviseCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{suitesCommand, reviseCommand}
}

func getTestSuite(projectID uuid.UUID, testSuiteName string, revision *int32, allRevisions bool) []CommandBuilder {
	suitesCommand := CommandBuilder{
		Command: "suites",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuiteName,
			},
		},
	}
	if revision != nil {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--revision",
			Value: fmt.Sprintf("%d", *revision),
		})
	}
	if allRevisions {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--all-revisions",
			Value: "",
		})
	}
	return []CommandBuilder{suitesCommand, getCommand}
}

func listTestSuites(projectID uuid.UUID) []CommandBuilder {
	suitesCommand := CommandBuilder{
		Command: "suites",
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
		},
	}
	return []CommandBuilder{suitesCommand, listCommand}
}

func archiveTestSuite(projectID uuid.UUID, testSuiteKey string) []CommandBuilder {
	suitesCommand := CommandBuilder{
		Command: "test-suites",
	}
	archiveCommand := CommandBuilder{
		Command: "archive",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuiteKey,
			},
		},
	}
	return []CommandBuilder{suitesCommand, archiveCommand}
}

func restoreTestSuite(projectID uuid.UUID, testSuiteKey string) []CommandBuilder {
	suitesCommand := CommandBuilder{
		Command: "test-suites",
	}
	restoreCommand := CommandBuilder{
		Command: "restore",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuiteKey,
			},
		},
	}
	return []CommandBuilder{suitesCommand, restoreCommand}
}

func getTestSuiteBatches(projectID uuid.UUID, testSuiteName string, revision *int32) []CommandBuilder {
	// We build a get batch command with the name flag
	testSuitesCommand := CommandBuilder{
		Command: "suites",
	}
	batchesCommand := CommandBuilder{
		Command: "batches",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuiteName,
			},
		},
	}
	if revision != nil {
		batchesCommand.Flags = append(batchesCommand.Flags, Flag{
			Name:  "--revision",
			Value: fmt.Sprintf("%d", *revision),
		})
	}
	return []CommandBuilder{testSuitesCommand, batchesCommand}
}

func runTestSuite(projectID uuid.UUID, testSuiteName string, revision *int32, buildID string, parameters map[string]string, github bool, account string, batchName *string, allowableFailurePercent *int, metricsBuildID *string) []CommandBuilder {
	// We build a get batch command with the name flag
	testSuiteCommand := CommandBuilder{
		Command: "suites",
	}
	runCommand := CommandBuilder{
		Command: "run",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuiteName,
			},
			{
				Name:  "--build-id",
				Value: buildID,
			},
		},
	}
	if revision != nil {
		runCommand.Flags = append(runCommand.Flags, Flag{
			Name:  "--revision",
			Value: fmt.Sprintf("%d", *revision),
		})
	}
	if len(parameters) > 0 {
		for key, value := range parameters {
			runCommand.Flags = append(runCommand.Flags, Flag{
				Name:  "--parameter",
				Value: fmt.Sprintf("%s:%s", key, value),
			})
		}
	}
	if github {
		runCommand.Flags = append(runCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	if len(account) > 0 {
		runCommand.Flags = append(runCommand.Flags, Flag{
			Name:  "--account",
			Value: account,
		})
	}

	if batchName != nil {
		runCommand.Flags = append(runCommand.Flags, Flag{
			Name:  "--batch-name",
			Value: *batchName,
		})
	}

	if allowableFailurePercent != nil {
		runCommand.Flags = append(runCommand.Flags, Flag{
			Name:  "--allowable-failure-percent",
			Value: fmt.Sprintf("%d", *allowableFailurePercent),
		})
	}

	if metricsBuildID != nil {
		runCommand.Flags = append(runCommand.Flags, Flag{
			Name:  "--metrics-build-override",
			Value: *metricsBuildID,
		})
	}

	return []CommandBuilder{testSuiteCommand, runCommand}
}

func createReport(projectID uuid.UUID, testSuiteID string, testSuiteRevision *int32, branchID string, metricsBuildID string, length *int, startTimestamp *string, endTimestamp *string, respectRevisionBoundary *bool, name *string, github bool, account string) []CommandBuilder {
	// We build a create report command with the required an optional flags.
	reportCommand := CommandBuilder{
		Command: "reports",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--test-suite",
				Value: testSuiteID,
			},
			{
				Name:  "--branch",
				Value: branchID,
			},
			{
				Name:  "--metrics-build-id",
				Value: metricsBuildID,
			},
		},
	}
	if testSuiteRevision != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--test-suite-revision",
			Value: fmt.Sprintf("%d", *testSuiteRevision),
		})
	}
	if length != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--length",
			Value: fmt.Sprintf("%v", *length),
		})
	}
	if startTimestamp != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--start-timestamp",
			Value: *startTimestamp,
		})
	}
	if endTimestamp != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--end-timestamp",
			Value: *endTimestamp,
		})
	}
	if respectRevisionBoundary != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--respect-revision-boundary",
			Value: fmt.Sprintf("%t", *respectRevisionBoundary),
		})
	}
	if name != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--report-name",
			Value: *name,
		})
	}

	if len(account) > 0 {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--account",
			Value: account,
		})
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{reportCommand, createCommand}
}
func getReportByName(projectID uuid.UUID, reportName string, exitStatus bool) []CommandBuilder {
	// We build a get report command with the name flag
	reportCommand := CommandBuilder{
		Command: "reports",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--report-name",
				Value: reportName,
			},
		},
	}
	if exitStatus {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--exit-status",
			Value: "",
		})
	}
	return []CommandBuilder{reportCommand, getCommand}
}

func getReportByID(projectID uuid.UUID, reportID string, exitStatus bool) []CommandBuilder {
	// We build a get report command with the id flag
	reportCommand := CommandBuilder{
		Command: "reports",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--report-id",
				Value: reportID,
			},
		},
	}
	if exitStatus {
		getCommand.Flags = append(getCommand.Flags, Flag{
			Name:  "--exit-status",
			Value: "",
		})
	}
	return []CommandBuilder{reportCommand, getCommand}
}

func listReportLogs(projectID uuid.UUID, reportID, reportName string) []CommandBuilder {
	reportCommand := CommandBuilder{
		Command: "reports",
	}
	reportIdFlag := Flag{
		Name:  "--report-id",
		Value: reportID,
	}
	reportNameFlag := Flag{
		Name:  "--report-name",
		Value: reportName,
	}
	allFlags := []Flag{
		{
			Name:  "--project",
			Value: projectID.String(),
		},
	}
	// For ease of interface downstream, only set the flags if they were passed in to this method
	if reportID != "" {
		allFlags = append(allFlags, reportIdFlag)
	}
	if reportName != "" {
		allFlags = append(allFlags, reportNameFlag)
	}
	listLogsCommand := CommandBuilder{
		Command: "logs",
		Flags:   allFlags,
	}
	return []CommandBuilder{reportCommand, listLogsCommand}
}

func rerunBatch(projectID uuid.UUID, batchID string, jobIDs []string) []CommandBuilder {
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	rerunCommand := CommandBuilder{
		Command: "rerun",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID,
			},
		},
	}
	if len(jobIDs) > 0 {
		rerunCommand.Flags = append(rerunCommand.Flags, Flag{
			Name:  "--test-ids",
			Value: strings.Join(jobIDs, ","),
		})
	}
	return []CommandBuilder{batchCommand, rerunCommand}
}

// As a first test, we expect the help command to run successfully
func TestHelp(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing help command")
	runCommand := CommandBuilder{
		Command: "help",
	}
	output := s.runCommand(ts, []CommandBuilder{runCommand}, ExpectNoError)
	ts.Contains(output.StdOut, "USAGE")
}

func TestProjectCommands(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing project create command")
	// Check we can successfully create a project with a unique name
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, CreatedProject)
	ts.Empty(output.StdErr)
	// Validate that repeating that name leads to an error:
	output = s.runCommand(ts, createProject(projectName, "description", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, ProjectNameCollision)

	// Validate that omitting the name leads to an error:
	output = s.runCommand(ts, createProject("", "description", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyProjectName)
	// Validate that omitting the description leads to an error:
	output = s.runCommand(ts, createProject(projectName, "", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyProjectDescription)

	// Check we can list the projects, and our new project is in it:
	output = s.runCommand(ts, listProjects(), ExpectNoError)
	ts.Contains(output.StdOut, projectName)

	// Now get, verify, and archive the project:
	fmt.Println("Testing project get command")
	output = s.runCommand(ts, getProject(projectName), ExpectNoError)
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	ts.NoError(err)
	ts.Equal(projectName, project.Name)
	ts.Empty(output.StdErr)

	// Attempt to get project by id:
	output = s.runCommand(ts, getProject((project.ProjectID).String()), ExpectNoError)
	var project2 api.Project
	err = json.Unmarshal([]byte(output.StdOut), &project2)
	ts.NoError(err)
	ts.Equal(projectName, project.Name)
	ts.Empty(output.StdErr)
	// Attempt to get a project with empty name and id:
	output = s.runCommand(ts, getProject(""), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	// Non-existent project:
	output = s.runCommand(ts, getProject(uuid.New().String()), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	// Blank name:
	output = s.runCommand(ts, getProject(""), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)

	// Validate that using the id as another project name throws an error.
	output = s.runCommand(ts, createProject(project.ProjectID.String(), "description", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, ProjectNameCollision)

	fmt.Println("Testing project archive command")
	output = s.runCommand(ts, archiveProject(projectName), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
	// Verify that attempting to re-archive will fail:
	output = s.runCommand(ts, archiveProject(projectName), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	// Verify that a valid project ID is needed:
	output = s.runCommand(ts, archiveProject(""), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
}

func TestProjectCreateGithub(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing project create command, with --github flag")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// Now get, verify, and archive the project:
	output = s.runCommand(ts, getProject(projectIDString), ExpectNoError)
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	ts.NoError(err)
	ts.Equal(projectName, project.Name)
	ts.Equal(projectID, project.ProjectID)
	ts.Empty(output.StdErr)
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

// Test branch creation:
func TestBranchCreate(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing branch creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, CreatedBranch)
	// Validate that  missing name, project, or type returns errors:
	output = s.runCommand(ts, createBranch(projectID, "", "RELEASE", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyBranchName)
	output = s.runCommand(ts, createBranch(uuid.Nil, branchName, "RELEASE", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(ts, createBranch(projectID, branchName, "INVALID", GithubFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidBranchType)

	// Check we can list the branches, and our new branch is in it:
	output = s.runCommand(ts, listBranches(projectID), ExpectNoError)
	ts.Contains(output.StdOut, branchName)

	// Archive the test project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestBranchCreateGithub(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing branch creation, with --github flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Archive the test project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestSystems(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing system creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	const systemDescription = "test system description"
	const buildVCPUs = 1
	const buildGPUs = 0
	const buildMemoryMiB = 1000
	const buildSharedMemoryMB = 64
	const metricsBuildVCPUs = 2
	const metricsBuildGPUs = 10
	const metricsBuildMemoryMiB = 900
	const metricsBuildSharedMemoryMB = 1024
	output = s.runCommand(ts, createSystem(projectIDString, systemName, systemDescription, Ptr(buildVCPUs), Ptr(buildGPUs), Ptr(buildMemoryMiB), Ptr(buildSharedMemoryMB), Ptr(metricsBuildVCPUs), Ptr(metricsBuildGPUs), Ptr(metricsBuildMemoryMiB), Ptr(metricsBuildSharedMemoryMB), nil, GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, CreatedSystem)
	// Get the system:
	output = s.runCommand(ts, getSystem(projectIDString, systemName), ExpectNoError)
	var system api.System
	err := json.Unmarshal([]byte(output.StdOut), &system)
	ts.NoError(err)
	ts.Equal(systemName, system.Name)
	ts.Equal(systemDescription, system.Description)
	ts.Equal(buildVCPUs, system.BuildVcpus)
	ts.Equal(buildGPUs, system.BuildGpus)
	ts.Equal(buildMemoryMiB, system.BuildMemoryMib)
	ts.Equal(buildSharedMemoryMB, system.BuildSharedMemoryMb)
	ts.Equal(metricsBuildVCPUs, system.MetricsBuildVcpus)
	ts.Equal(metricsBuildGPUs, system.MetricsBuildGpus)
	ts.Equal(metricsBuildMemoryMiB, system.MetricsBuildMemoryMib)
	ts.Equal(metricsBuildSharedMemoryMB, system.MetricsBuildSharedMemoryMb)
	ts.Empty(output.StdErr)
	systemID := system.SystemID

	// Validate that the defaults work:
	system2Name := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, system2Name, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, CreatedSystem)
	// Get the system:
	output = s.runCommand(ts, getSystem(projectIDString, system2Name), ExpectNoError)
	var system2 api.System
	err = json.Unmarshal([]byte(output.StdOut), &system2)
	ts.NoError(err)
	ts.Equal(system2Name, system2.Name)
	ts.Equal(systemDescription, system2.Description)
	ts.Equal(commands.DefaultCPUs, system2.BuildVcpus)
	ts.Equal(commands.DefaultGPUs, system2.BuildGpus)
	ts.Equal(commands.DefaultMemoryMiB, system2.BuildMemoryMib)
	ts.Equal(commands.DefaultSharedMemoryMB, system2.BuildSharedMemoryMb)
	ts.Equal(commands.DefaultCPUs, system2.MetricsBuildVcpus)
	ts.Equal(commands.DefaultGPUs, system2.MetricsBuildGpus)
	ts.Equal(commands.DefaultMemoryMiB, system2.MetricsBuildMemoryMib)
	ts.Equal(commands.DefaultSharedMemoryMB, system2.MetricsBuildSharedMemoryMb)
	ts.Empty(output.StdErr)

	// Validate that missing name, project, or description returns errors:
	output = s.runCommand(ts, createSystem(projectIDString, "", systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptySystemDescription)
	output = s.runCommand(ts, createSystem(uuid.Nil.String(), systemName, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)

	// Validate that invalid architecture returns errors:
	output = s.runCommand(ts, createSystem(projectIDString, systemName, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, Ptr("invalid"), GithubFalse), ExpectError)
	ts.Contains(output.StdErr, "invalid architecture: invalid")

	// Check we can list the systems, and our new system is in it:
	output = s.runCommand(ts, listSystems(projectID), ExpectNoError)
	ts.Contains(output.StdOut, systemName)

	// Now add a couple of builds to the system (and a branch by the autocreate):
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	build1Version := "0.0.1"
	output = s.runCommand(ts, createBuild(projectIDString, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, build1Version, GithubTrue, AutoCreateBranchTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	build1IDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(build1IDString)
	build2Version := "0.0.2"
	output = s.runCommand(ts, createBuild(projectIDString, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, build2Version, GithubTrue, AutoCreateBranchTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	build2IDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(build2IDString)
	// Check we can list the builds, and our new builds are in it:
	output = s.runCommand(ts, systemBuilds(projectIDString, systemID.String()), ExpectNoError)
	ts.Contains(output.StdOut, build1Version)
	ts.Contains(output.StdOut, build2Version)

	// Create and tag a couple of experiences:
	experience1Name := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experience1Name, "description", "location", EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experience1IDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experience1IDString)
	experience2Name := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experience2Name, "description", "location", EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experience2IDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experience2IDString)
	// Check we can list the experiences for the systems, and our new experiences are not in it:
	output = s.runCommand(ts, systemExperiences(projectIDString, systemName), ExpectNoError)
	ts.NotContains(output.StdOut, experience1Name)
	ts.NotContains(output.StdOut, experience2Name)
	ts.NotContains(output.StdOut, experience1IDString)
	ts.NotContains(output.StdOut, experience2IDString)

	// Add the experiences to the system:
	output = s.runCommand(ts, addSystemToExperience(projectIDString, systemName, experience1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(ts, addSystemToExperience(projectIDString, systemName, experience1IDString), ExpectError)
	ts.Contains(output.StdErr, SystemAlreadyRegistered)
	output = s.runCommand(ts, addSystemToExperience(projectName, systemName, experience2IDString), ExpectNoError)
	// Check we can list the experiences for the systems, and our new experiences are in it:
	output = s.runCommand(ts, systemExperiences(projectIDString, systemName), ExpectNoError)
	ts.Contains(output.StdOut, experience1Name)
	ts.Contains(output.StdOut, experience2Name)
	ts.Contains(output.StdOut, experience1IDString)
	ts.Contains(output.StdOut, experience2IDString)

	// Remove one experience:
	output = s.runCommand(ts, removeSystemFromExperience(projectIDString, systemName, experience1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(ts, removeSystemFromExperience(projectIDString, systemName, experience1IDString), ExpectError)
	// Check we can list the experiences for the systems, and only one experience is in it:
	output = s.runCommand(ts, systemExperiences(projectIDString, systemName), ExpectNoError)
	ts.NotContains(output.StdOut, experience1Name)
	ts.Contains(output.StdOut, experience2Name)
	// Remove the second experience:
	output = s.runCommand(ts, removeSystemFromExperience(projectIDString, systemName, experience2IDString), ExpectNoError)
	// Check we can list the experiences for the systems, and no experiences are in it:
	output = s.runCommand(ts, systemExperiences(projectIDString, systemName), ExpectNoError)
	ts.NotContains(output.StdOut, experience1Name)
	ts.NotContains(output.StdOut, experience2Name)

	// Edge cases:
	output = s.runCommand(ts, addSystemToExperience("", systemName, experience1IDString), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(ts, addSystemToExperience(projectIDString, "", experience1IDString), ExpectError)
	ts.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(ts, addSystemToExperience(projectIDString, systemName, ""), ExpectError)
	ts.Contains(output.StdErr, EmptyExperienceName)
	output = s.runCommand(ts, removeSystemFromExperience("", systemName, experience1IDString), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(ts, removeSystemFromExperience(projectIDString, "", experience1IDString), ExpectError)
	ts.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(ts, removeSystemFromExperience(projectIDString, systemName, ""), ExpectError)
	ts.Contains(output.StdErr, EmptyExperienceName)

	// Create and tag a couple of metrics builds:
	metricsBuild1Name := fmt.Sprintf("test-metrics-build-%s", uuid.New().String())
	output = s.runCommand(ts, createMetricsBuild(projectID, metricsBuild1Name, "public.ecr.aws/docker/library/hello-world:latest", "0.0.1", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the metrics build ID as a UUID
	metricsBuild1IDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuild1IDString)
	metricsBuild2Name := fmt.Sprintf("test-metrics-build-%s", uuid.New().String())
	output = s.runCommand(ts, createMetricsBuild(projectID, metricsBuild2Name, "public.ecr.aws/docker/library/hello-world:latest", "0.0.2", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the metrics build ID as a UUID
	metricsBuild2IDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuild2IDString)
	// Check we can list the metrics builds for the systems, and our new metrics builds are not in it:
	output = s.runCommand(ts, systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	ts.NotContains(output.StdOut, metricsBuild1Name)
	ts.NotContains(output.StdOut, metricsBuild2Name)
	ts.NotContains(output.StdOut, metricsBuild1IDString)
	ts.NotContains(output.StdOut, metricsBuild2IDString)

	// Add the metrics builds to the system:
	output = s.runCommand(ts, addSystemToMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(ts, addSystemToMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectError)
	ts.Contains(output.StdErr, SystemAlreadyRegistered)
	output = s.runCommand(ts, addSystemToMetricsBuild(projectName, systemName, metricsBuild2IDString), ExpectNoError)
	// Check we can list the metrics builds for the systems, and our new metrics builds are in it:
	output = s.runCommand(ts, systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	ts.Contains(output.StdOut, metricsBuild1Name)
	ts.Contains(output.StdOut, metricsBuild2Name)
	ts.Contains(output.StdOut, metricsBuild1IDString)
	ts.Contains(output.StdOut, metricsBuild2IDString)

	// Remove one metrics build:
	output = s.runCommand(ts, removeSystemFromMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(ts, removeSystemFromMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectError)
	// Check we can list the metrics builds for the systems, and only one metrics build is in it:
	output = s.runCommand(ts, systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	ts.NotContains(output.StdOut, metricsBuild1Name)
	ts.Contains(output.StdOut, metricsBuild2Name)
	// Remove the second metrics build:
	output = s.runCommand(ts, removeSystemFromMetricsBuild(projectIDString, systemName, metricsBuild2IDString), ExpectNoError)
	// Check we can list the metrics builds for the systems, and no metrics builds are in it:
	output = s.runCommand(ts, systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	ts.NotContains(output.StdOut, metricsBuild1Name)
	ts.NotContains(output.StdOut, metricsBuild2Name)

	// Edge cases:
	output = s.runCommand(ts, addSystemToMetricsBuild("", systemName, metricsBuild1IDString), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(ts, addSystemToMetricsBuild(projectIDString, "", metricsBuild1IDString), ExpectError)
	ts.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(ts, addSystemToMetricsBuild(projectIDString, systemName, ""), ExpectError)
	ts.Contains(output.StdErr, EmptyMetricsBuildName)
	output = s.runCommand(ts, removeSystemFromMetricsBuild("", systemName, metricsBuild1IDString), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(ts, removeSystemFromMetricsBuild(projectIDString, "", metricsBuild1IDString), ExpectError)
	ts.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(ts, removeSystemFromMetricsBuild(projectIDString, systemName, ""), ExpectError)
	ts.Contains(output.StdErr, EmptyMetricsBuildName)

	// Update the system:
	const updatedSystemDescription = "updated system description"
	output = s.runCommand(ts,
		updateSystem(projectIDString, systemName, nil, Ptr(updatedSystemDescription), nil, nil, nil, nil, nil, nil, nil, nil),
		ExpectNoError)
	fmt.Println(output.StdErr)
	fmt.Println(output.StdOut)
	ts.Contains(output.StdOut, UpdatedSystem)
	// Get the system:
	output = s.runCommand(ts, getSystem(projectIDString, systemName), ExpectNoError)
	var updatedSystem api.System
	err = json.Unmarshal([]byte(output.StdOut), &updatedSystem)
	ts.NoError(err)
	ts.Equal(systemName, updatedSystem.Name)
	ts.Equal(updatedSystemDescription, updatedSystem.Description)
	ts.Equal(buildVCPUs, updatedSystem.BuildVcpus)
	ts.Equal(buildGPUs, updatedSystem.BuildGpus)
	ts.Equal(buildMemoryMiB, updatedSystem.BuildMemoryMib)
	ts.Equal(buildSharedMemoryMB, updatedSystem.BuildSharedMemoryMb)
	ts.Equal(metricsBuildVCPUs, updatedSystem.MetricsBuildVcpus)
	ts.Equal(metricsBuildGPUs, updatedSystem.MetricsBuildGpus)
	ts.Equal(metricsBuildMemoryMiB, updatedSystem.MetricsBuildMemoryMib)
	ts.Equal(metricsBuildSharedMemoryMB, updatedSystem.MetricsBuildSharedMemoryMb)
	ts.Empty(output.StdErr)

	// Sample edit for all the resout of the resources
	newName := "foobar"
	newBuildCPUs := 100
	newBuildMemory := 101
	newBuildGPUs := 102
	newBuildSharedMemory := 103
	newMetricsBuildCPUs := 100
	newMetricsBuildMemory := 101
	newMetricsBuildGPUs := 102
	newMetricsBuildSharedMemory := 103

	output = s.runCommand(ts,
		updateSystem(projectIDString, systemName, Ptr(newName), nil, Ptr(newBuildCPUs), Ptr(newBuildGPUs), Ptr(newBuildMemory), Ptr(newBuildSharedMemory), Ptr(newMetricsBuildCPUs), Ptr(newMetricsBuildGPUs), Ptr(newMetricsBuildMemory), Ptr(newMetricsBuildSharedMemory)),
		ExpectNoError)
	fmt.Println(output.StdErr)
	fmt.Println(output.StdOut)
	ts.Contains(output.StdOut, UpdatedSystem)
	// Get the system:
	output = s.runCommand(ts, getSystem(projectIDString, newName), ExpectNoError)
	var newUpdatedSystem api.System
	err = json.Unmarshal([]byte(output.StdOut), &newUpdatedSystem)
	ts.NoError(err)
	ts.Equal(newName, newUpdatedSystem.Name)
	ts.Equal(updatedSystemDescription, newUpdatedSystem.Description)
	ts.Equal(newBuildCPUs, newUpdatedSystem.BuildVcpus)
	ts.Equal(newBuildGPUs, newUpdatedSystem.BuildGpus)
	ts.Equal(newBuildMemory, newUpdatedSystem.BuildMemoryMib)
	ts.Equal(newBuildSharedMemory, newUpdatedSystem.BuildSharedMemoryMb)
	ts.Equal(newMetricsBuildCPUs, newUpdatedSystem.MetricsBuildVcpus)
	ts.Equal(newMetricsBuildGPUs, newUpdatedSystem.MetricsBuildGpus)
	ts.Equal(newMetricsBuildMemory, newUpdatedSystem.MetricsBuildMemoryMib)
	ts.Equal(newMetricsBuildSharedMemory, newUpdatedSystem.MetricsBuildSharedMemoryMb)
	ts.Empty(output.StdErr)

	// Archive the system:
	output = s.runCommand(ts, archiveSystem(projectIDString, newName), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedSystem)
	ts.Empty(output.StdErr)

	// Archive the test project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestSystemCreateGithub(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing system creation, with github flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	const systemDescription = "test system description"
	output = s.runCommand(ts, createSystem(projectIDString, systemName, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// Get the system:
	output = s.runCommand(ts, getSystem(projectIDString, systemName), ExpectNoError)
	var system2 api.System
	err := json.Unmarshal([]byte(output.StdOut), &system2)
	ts.NoError(err)
	ts.Equal(systemName, system2.Name)
	ts.Equal(systemDescription, system2.Description)
	ts.Equal(commands.DefaultCPUs, system2.BuildVcpus)
	ts.Equal(commands.DefaultGPUs, system2.BuildGpus)
	ts.Equal(commands.DefaultMemoryMiB, system2.BuildMemoryMib)
	ts.Equal(commands.DefaultSharedMemoryMB, system2.BuildSharedMemoryMb)
	ts.Equal(commands.DefaultCPUs, system2.MetricsBuildVcpus)
	ts.Equal(commands.DefaultGPUs, system2.MetricsBuildGpus)
	ts.Equal(commands.DefaultMemoryMiB, system2.MetricsBuildMemoryMib)
	ts.Equal(commands.DefaultSharedMemoryMB, system2.MetricsBuildSharedMemoryMb)
	ts.Empty(output.StdErr)

	// Check we can list the systems, and our new system is in it:
	output = s.runCommand(ts, listSystems(projectID), ExpectNoError)
	ts.Contains(output.StdOut, systemName)

	// Archive the test project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func (s *EndToEndTestHelper) verifyBuild(ts *assert.Assertions, projectID uuid.UUID, branchName string, branchID uuid.UUID, systemName string, systemID uuid.UUID, systemIDString string, branchIDString string, buildIDString string, buildVersion string) {
	// Check we can list the builds by passing in the branch, and our new build is in it:
	output := s.runCommand(ts, listBuilds(projectID, Ptr(branchName), nil), ExpectNoError) // with no system filter
	ts.Contains(output.StdOut, systemIDString)
	ts.Contains(output.StdOut, branchIDString)
	ts.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the system, and our new build is in it:
	output = s.runCommand(ts, listBuilds(projectID, nil, Ptr(systemName)), ExpectNoError) // with no branch filter
	ts.Contains(output.StdOut, systemIDString)
	ts.Contains(output.StdOut, branchIDString)
	ts.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the branchID, and our new build is in it:
	output = s.runCommand(ts, listBuilds(projectID, Ptr(branchID.String()), nil), ExpectNoError)
	ts.Contains(output.StdOut, systemIDString)
	ts.Contains(output.StdOut, branchIDString)
	ts.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the systemID, and our new build is in it:
	output = s.runCommand(ts, listBuilds(projectID, nil, Ptr(systemID.String())), ExpectNoError)
	ts.Contains(output.StdOut, systemIDString)
	ts.Contains(output.StdOut, branchIDString)
	ts.Contains(output.StdOut, buildIDString)

	// Check we can list the builds with no filters, and our new build is in it:
	output = s.runCommand(ts, listBuilds(projectID, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, systemIDString)
	ts.Contains(output.StdOut, branchIDString)
	ts.Contains(output.StdOut, buildIDString)
}

// Test the build creation:
func TestBuildCreateUpdate(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing build creation")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the project ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	systemID := uuid.MustParse(systemIDString)
	// Now create a build using the image URI:
	originalBuildDescription := "description"
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, originalBuildDescription, "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	s.verifyBuild(ts, projectID, branchName, branchID, systemName, systemID, systemIDString, branchIDString, buildIDString, "1.0.0")

	// Verify that each of the required flags are required:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyBuildName)
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyBuildSpecAndBuildImage)
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "blah", []string{"./data/test_build_spec.yaml"}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, ConfigParamsMutuallyExclusive)
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyBuildVersion)
	output = s.runCommand(ts, createBuild("", branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(ts, createBuild(projectName, "", systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, BranchNotExist)
	output = s.runCommand(ts, createBuild(projectName, branchName, "", "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, SystemDoesNotExist)
	// Validate the image URI is required to be valid and have a tag:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidBuildImage)

	// Update the branch id:
	secondBranchName := fmt.Sprintf("updated-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, secondBranchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	updatedBranchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	updatedBranchID := uuid.MustParse(updatedBranchIDString)
	output = s.runCommand(ts, updateBuild(projectIDString, buildID, Ptr(updatedBranchID), nil), ExpectNoError)
	ts.Contains(output.StdOut, UpdatedBuild)
	// Get the build and check:
	output = s.runCommand(ts, getBuild(projectIDString, buildID, false), ExpectNoError)
	var build api.Build
	err := json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(updatedBranchID, build.BranchID)
	ts.Equal(originalBuildDescription, build.Description)
	ts.Empty(output.StdErr)

	updatedBuildDescription := "updated description"
	output = s.runCommand(ts, updateBuild(projectIDString, buildID, nil, Ptr(updatedBuildDescription)), ExpectNoError)
	ts.Contains(output.StdOut, UpdatedBuild)
	// Get the build and check:
	output = s.runCommand(ts, getBuild(projectIDString, buildID, false), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(updatedBranchID, build.BranchID)
	ts.Equal(updatedBuildDescription, build.Description)
	ts.Empty(output.StdErr)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible
}
func TestBuildCreateWithBuildSpec(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing build creation")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the project ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	systemID := uuid.MustParse(systemIDString)
	// Now create a build using the image URI:
	originalBuildDescription := "description"
	// Now create a build using the build spec:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, originalBuildDescription, "", []string{"./data/test_build_spec.yaml"}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	s.verifyBuild(ts, projectID, branchName, branchID, systemName, systemID, systemIDString, branchIDString, buildIDString, "1.0.0")

	// Update the branch id:
	secondBranchName := fmt.Sprintf("updated-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, secondBranchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	updatedBranchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	updatedBranchID := uuid.MustParse(updatedBranchIDString)
	output = s.runCommand(ts, updateBuild(projectIDString, buildID, Ptr(updatedBranchID), nil), ExpectNoError)
	ts.Contains(output.StdOut, UpdatedBuild)
	// Get the build and check:
	output = s.runCommand(ts, getBuild(projectIDString, buildID, false), ExpectNoError)
	var build api.Build
	err := json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(updatedBranchID, build.BranchID)
	ts.Equal(originalBuildDescription, build.Description)
	ts.Empty(output.StdErr)

	// Verify some details of the constructed+retrieved build spec:
	var buildSpec compose_types.Project
	err = yaml.Unmarshal([]byte(build.BuildSpecification), &buildSpec)
	ts.NoError(err)
	ts.Len(buildSpec.Services, 4)
	systemService := buildSpec.Services["system"]
	ts.Len(systemService.Environment, 3) // Two environment variables come from the top level file, the other comes from the `extends` definition
	ts.Equal(buildSpec.Services["orchestrator"].Image, buildSpec.Services["command-orchestrator"].Image)
	ts.NotEqual(buildSpec.Services["orchestrator"].Image, buildSpec.Services["system"].Image)

	updatedBuildDescription := "updated description"
	output = s.runCommand(ts, updateBuild(projectIDString, buildID, nil, Ptr(updatedBuildDescription)), ExpectNoError)
	ts.Contains(output.StdOut, UpdatedBuild)
	// Get the build and check:
	output = s.runCommand(ts, getBuild(projectIDString, buildID, false), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(updatedBranchID, build.BranchID)
	ts.Equal(updatedBuildDescription, build.Description)
	ts.Empty(output.StdErr)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible
}

func TestBuildCreateGithub(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing build creation, with --github flag")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the project ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}
func TestBuildCreateAutoCreateBranch(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing build creation with the auto-create-branch flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build: (with auto-create-branch flag). We expect this to succeed without any additional information
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	ts.Contains(output.StdOut, CreatedBuild)
	ts.NotContains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", branchName))

	// Now try to create a build with a new branch name:
	newBranchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBuild(projectName, newBranchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.1", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	ts.Contains(output.StdOut, CreatedBuild)
	ts.Contains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", newBranchName))
	ts.Contains(output.StdOut, CreatedBranch)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestExperienceCreate(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing experience creation command")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create two systems to add as part of the experience creation:
	systemName1 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName1, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString1 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString1)
	systemName2 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName2, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString2 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString2)

	systemNames := []string{systemName1, systemName2}
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	timeoutSeconds := int32(200)
	timeout := time.Duration(timeoutSeconds) * time.Second
	profile := "test-profile"
	envVars := []string{"ENV_VAR1=value1", "ENV_VAR2=value2"}
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", "location", systemNames, EmptySlice, &timeout, &profile, envVars, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// Get the experience ID from the create output
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID := uuid.MustParse(experienceIDString)

	// List experiences and check it's there
	output = s.runCommand(ts, listExperiences(projectID), ExpectNoError)
	ts.Contains(output.StdOut, experienceName)

	// Archive the experience
	output = s.runCommand(ts, archiveExperience(projectID, experienceIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedExperience)

	// List experiences again and check it's not there
	output = s.runCommand(ts, listExperiences(projectID), ExpectNoError)
	ts.Contains(output.StdOut, "no experiences")

	// Get the experience by ID and check it's still retrievable
	output = s.runCommand(ts, getExperience(projectID, experienceIDString), ExpectNoError)
	ts.Contains(output.StdOut, experienceName)

	// Restore the experience
	output = s.runCommand(ts, restoreExperience(projectID, experienceIDString), ExpectNoError)
	ts.Empty(output.StdErr)

	// List experiences again and verify it's back
	output = s.runCommand(ts, listExperiences(projectID), ExpectNoError)
	ts.Contains(output.StdOut, experienceName)

	// Get the experience and verify its properties
	output = s.runCommand(ts, getExperience(projectID, experienceIDString), ExpectNoError)
	var experience api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &experience)
	ts.NoError(err)
	ts.Equal(experienceID, experience.ExperienceID)
	ts.Equal(experienceName, experience.Name)
	ts.Equal("description", experience.Description)
	ts.Equal("location", experience.Location)
	ts.Equal(timeoutSeconds, experience.ContainerTimeoutSeconds)
	ts.Equal(profile, experience.Profile)
	ts.Equal(len(envVars), len(experience.EnvironmentVariables))

	// Create an experience with multiple locations
	experienceName2 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experienceName2, "description", "location1,location2", EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString2)
	// Get the experience and verify there are two locations:
	output = s.runCommand(ts, getExperience(projectID, experienceIDString2), ExpectNoError)
	var experience2 api.Experience
	err = json.Unmarshal([]byte(output.StdOut), &experience2)
	ts.NoError(err)
	ts.Equal(2, len(experience2.Locations))
	ts.ElementsMatch([]string{"location1", "location2"}, experience2.Locations)

	// Archive the project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestExperienceCreateGithub(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing experience creation command, with --github flag")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", "location", EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
	// Now get the experience by id:
	output = s.runCommand(ts, getExperience(projectID, experienceIDString), ExpectNoError)
	ts.Contains(output.StdOut, experienceName)
	ts.Empty(output.StdErr)
	var experience api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &experience)
	ts.NoError(err)
	ts.Equal(experienceName, experience.Name)
	ts.Equal("description", experience.Description)
	ts.Equal("location", experience.Location)
	ts.Equal(int32(3600), experience.ContainerTimeoutSeconds) // default timeout

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func (s *EndToEndTestHelper) verifyExperienceUpdate(ts *assert.Assertions, projectID uuid.UUID, experienceID, expectedName, expectedDescription, expectedLocation string, expectedTimeout int32, expectedProfile string, expectedEnvVars []string) {
	output := s.runCommand(ts, getExperience(projectID, experienceID), ExpectNoError)
	var experience api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &experience)
	ts.NoError(err)
	ts.Equal(expectedName, experience.Name)
	ts.Equal(expectedDescription, experience.Description)
	ts.Equal(expectedLocation, experience.Location)
	ts.Equal(expectedTimeout, experience.ContainerTimeoutSeconds)
	if expectedProfile != "" {
		ts.Equal(expectedProfile, experience.Profile)
	}
	if len(expectedEnvVars) > 0 {
		ts.Equal(len(expectedEnvVars), len(experience.EnvironmentVariables))
		for i, envVar := range expectedEnvVars {
			ts.Equal(strings.Split(envVar, "=")[0], experience.EnvironmentVariables[i].Name)
			ts.Equal(strings.Split(envVar, "=")[1], experience.EnvironmentVariables[i].Value)
		}
	}
}

func TestExperienceUpdate(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing experience update command")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create an experience:
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	originalDescription := "original description"
	originalLocation := "original location"
	originalTimeoutSeconds := int32(200)
	originalTimeout := time.Duration(originalTimeoutSeconds) * time.Second
	originalProfile := "original-profile"
	originalEnvVars := []string{"ORIGINAL_ENV_VAR1=value1", "ORIGINAL_ENV_VAR2=value2"}
	output = s.runCommand(ts, createExperience(projectID, experienceName, originalDescription, originalLocation, EmptySlice, EmptySlice, &originalTimeout, &originalProfile, originalEnvVars, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]

	// Verify the created experience has the correct values
	output = s.runCommand(ts, getExperience(projectID, experienceIDString), ExpectNoError)
	var experience api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &experience)
	ts.NoError(err)
	ts.Equal(experienceName, experience.Name)
	ts.Equal(originalDescription, experience.Description)
	ts.Equal(originalLocation, experience.Location)
	ts.Equal(originalTimeoutSeconds, experience.ContainerTimeoutSeconds)
	ts.Equal(originalProfile, experience.Profile)
	ts.Equal(len(originalEnvVars), len(experience.EnvironmentVariables))
	for i, envVar := range originalEnvVars {
		ts.Equal(strings.Split(envVar, "=")[0], experience.EnvironmentVariables[i].Name)
		ts.Equal(strings.Split(envVar, "=")[1], experience.EnvironmentVariables[i].Value)
	}

	// 1. Update the experience name alone and verify
	newName := "updated-experience-name"
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, Ptr(newName), nil, nil, EmptySlice, EmptySlice, nil, nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, newName, originalDescription, originalLocation, originalTimeoutSeconds, originalProfile, originalEnvVars)

	// 2. Update the description alone and verify
	newDescription := "updated description"
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, nil, Ptr(newDescription), nil, EmptySlice, EmptySlice, nil, nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, newName, newDescription, originalLocation, originalTimeoutSeconds, originalProfile, originalEnvVars)

	// 3. Update the location alone and verify
	newLocation := "updated location"
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, nil, nil, Ptr(newLocation), EmptySlice, EmptySlice, nil, nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, newName, newDescription, newLocation, originalTimeoutSeconds, originalProfile, originalEnvVars)

	// 4. Update the timeout alone and verify
	newTimeoutSeconds := int32(300)
	newTimeout := time.Duration(newTimeoutSeconds) * time.Second
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, nil, nil, nil, EmptySlice, EmptySlice, &newTimeout, nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds, originalProfile, originalEnvVars)

	// 5. Update the profile alone and verify
	newProfile := "updated-profile"
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, nil, nil, nil, EmptySlice, EmptySlice, nil, &newProfile, nil), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds, newProfile, originalEnvVars)

	// 6. Update the environment variables alone and verify
	newEnvVars := []string{"UPDATED_ENV_VAR1=value1", "UPDATED_ENV_VAR2=value2"}
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, nil, nil, nil, EmptySlice, EmptySlice, nil, nil, newEnvVars), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds, newProfile, newEnvVars)

	// 7. Update the name, description, location, timeout, profile, and environment variables and verify
	finalName := "final-experience-name"
	finalDescription := "final description"
	finalLocation := "final location"
	finalTimeoutSeconds := int32(400)
	finalTimeout := time.Duration(finalTimeoutSeconds) * time.Second
	finalProfile := "final-profile"
	finalEnvVars := []string{"FINAL_ENV_VAR1=value1", "FINAL_ENV_VAR2=value2"}
	output = s.runCommand(ts, updateExperience(projectID, experienceIDString, &finalName, &finalDescription, &finalLocation, EmptySlice, EmptySlice, &finalTimeout, &finalProfile, finalEnvVars), ExpectNoError)
	s.verifyExperienceUpdate(ts, projectID, experienceIDString, finalName, finalDescription, finalLocation, finalTimeoutSeconds, finalProfile, finalEnvVars)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestBatchAndLogs(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// This test does not use parameters, so we create an empty parameter map:
	emptyParameterMap := map[string]string{}
	// First create two experiences:
	experienceName1 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/test-object/", s.Config.E2EBucket)
	output = s.runCommand(ts, createExperience(projectID, experienceName1, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experienceName2, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/resim/open-builds/log-ingest:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)

	// Create a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create an experience tag:
	tagName := fmt.Sprintf("test-tag-%s", uuid.New().String())
	output = s.runCommand(ts, createExperienceTag(projectID, tagName, "testing tag"), ExpectNoError)

	// List experience tags and check the list contains the tag we just created
	output = s.runCommand(ts, listExperienceTags(projectID), ExpectNoError)
	ts.Contains(output.StdOut, tagName)

	// Tag one of the experiences:
	output = s.runCommand(ts, tagExperience(projectID, tagName, experienceID1), ExpectNoError)

	// Adding the same tag again should error:
	output = s.runCommand(ts, tagExperience(projectID, tagName, experienceID1), ExpectError)

	// List experiences for the tag
	output = s.runCommand(ts, listExperiencesWithTag(projectID, tagName), ExpectNoError)
	var tagExperiences []api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	ts.NoError(err)
	ts.Equal(1, len(tagExperiences))

	// Tag 2nd, check list contains 2 experiences
	output = s.runCommand(ts, tagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(ts, listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	ts.NoError(err)
	ts.Equal(2, len(tagExperiences))

	// Untag and check list again
	output = s.runCommand(ts, untagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(ts, listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	ts.NoError(err)
	ts.Equal(1, len(tagExperiences))

	// Fail to list batch logs without a batch specifier
	output = s.runCommand(ts, listBatchLogs(projectID, "", ""), ExpectError)
	ts.Contains(output.StdErr, SelectOneRequired)

	// Fail to list batch logs with a bad UUID
	output = s.runCommand(ts, listBatchLogs(projectID, "not-a-uuid", ""), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchID)

	// Fail to list batch logs with a made up batch name
	output = s.runCommand(ts, listBatchLogs(projectID, "", "not-a-valid-batch-name"), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchName)

	// Fail to create a batch without any experience ids, tags, or names
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	ts.Contains(output.StdErr, SelectOneRequired)

	// Create a batch with (only) experience names using the --experiences flag
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName1, experienceName2}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH1 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH1)

	// Create a batch with mixed experience names and IDs in the --experiences flag
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName1, experienceIDString2}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH2 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH2)

	// Create a batch with an ID in the --experiences flag and a tag name in the --experience-tags flag (experience 1 is in the tag)
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceIDString2}, []string{tagName}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH3 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH3)

	// Create a batch without metrics with the github flag set, with a specified batch name and check the output
	batchName := fmt.Sprintf("test-batch-%s", uuid.New().String())
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, &batchName, Ptr(100)), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH4 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH4)

	// Get the batch by name:
	output = s.runCommand(ts, getBatchByName(projectID, batchName, ExitStatusFalse), ExpectNoError)
	ts.Contains(output.StdOut, batchName)
	ts.Contains(output.StdOut, batchIDStringGH4)

	// Now create a batch without the github flag, but with metrics
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, metricsBuildIDString, GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, CreatedBatch)
	ts.Empty(output.StdErr)

	// Extract from "Batch ID:" to the next newline:
	re := regexp.MustCompile(`Batch ID: (.+?)\n`)
	matches := re.FindStringSubmatch(output.StdOut)
	ts.Equal(2, len(matches))
	batchIDString := strings.TrimSpace(matches[1])
	batchID := uuid.MustParse(batchIDString)
	// Extract the batch name:
	re = regexp.MustCompile(`Batch name: (.+?)\n`)
	matches = re.FindStringSubmatch(output.StdOut)
	ts.Equal(2, len(matches))
	batchNameString := strings.TrimSpace(matches[1])
	// RePun:
	batchNameParts := strings.Split(batchNameString, "-")
	ts.Equal(3, len(batchNameParts))
	// Try a batch without any experiences:
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	ts.Contains(output.StdErr, SelectOneRequired)
	// Try a batch without a build id:
	output = s.runCommand(ts, createBatch(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	ts.Contains(output.StdErr, InvalidBuildID)
	// Try a batch with both experience tag ids and experience tag names (even if fake):
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{"tag-id"}, []string{"tag-name"}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	ts.Contains(output.StdErr, BranchTagMutuallyExclusive)

	// Try a batch with non-percentage allowable failure percent:
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{experienceIDString1}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, Ptr(101)), ExpectError)
	ts.Contains(output.StdErr, AllowableFailurePercent)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByName(projectID, batchNameString, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 failed
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByName(projectID, batchNameString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(batchNameString, *batch.FriendlyName)
	ts.Equal(batchID, *batch.BatchID)
	ts.Equal(metricsBuildID, *batch.MetricsBuildID)
	// Validate that it succeeded:
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Get the batch by ID:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(batchNameString, *batch.FriendlyName)
	ts.Equal(batchID, *batch.BatchID)
	ts.Equal(AssociatedAccount, batch.AssociatedAccount)
	// Validate that it succeeded:
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)

	// List the logs for the succeeded batch
	output = s.runCommand(ts, listBatchLogs(projectID, "", batchNameString), ExpectNoError)
	// Marshal into a struct:
	var batchLogs []api.BatchLog
	err = json.Unmarshal([]byte(output.StdOut), &batchLogs)
	ts.NoError(err)
	// Validate that one or more logs are returned
	ts.Greater(len(batchLogs), 0)
	for _, batchLog := range batchLogs {
		uuid.MustParse(batchLog.LogID.String())
	}

	// Pass blank name / id to batches get:
	output = s.runCommand(ts, getBatchByName(projectID, "", ExitStatusFalse), ExpectError)
	ts.Contains(output.StdErr, RequireBatchName)
	output = s.runCommand(ts, getBatchByID(projectID, "", ExitStatusFalse), ExpectError)
	ts.Contains(output.StdErr, RequireBatchName)

	// Pass unknown name / id to batches tests:
	output = s.runCommand(ts, getBatchByName(projectID, "does not exist", ExitStatusFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchName)
	output = s.runCommand(ts, getBatchByID(projectID, "0000-0000-0000-0000-000000000000", ExitStatusFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchID)

	// Now grab the tests from the batch:
	output = s.runCommand(ts, getBatchJobsByName(projectID, batchNameString), ExpectNoError)
	// Marshal into a struct:
	var tests []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &tests)
	ts.NoError(err)
	ts.Equal(2, len(tests))
	for _, test := range tests {
		ts.Contains([]uuid.UUID{experienceID1, experienceID2}, *test.ExperienceID)
		ts.Equal(buildID, *test.BuildID)
	}
	output = s.runCommand(ts, getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &tests)
	ts.NoError(err)
	ts.Equal(2, len(tests))
	for _, test := range tests {
		ts.Contains([]uuid.UUID{experienceID1, experienceID2}, *test.ExperienceID)
		ts.Equal(buildID, *test.BuildID)
	}

	testID2 := *tests[1].JobID
	// Pass blank name / id to batches tests:
	output = s.runCommand(ts, getBatchJobsByName(projectID, ""), ExpectError)
	ts.Contains(output.StdErr, RequireBatchName)
	output = s.runCommand(ts, getBatchJobsByID(projectID, ""), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchID)

	// Pass unknown name / id to batches tests:
	output = s.runCommand(ts, getBatchJobsByName(projectID, "does not exist"), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchName)

	// List test logs:
	output = s.runCommand(ts, listLogs(projectID, batchIDString, testID2.String()), ExpectNoError)
	// Marshal into a struct:
	var logs []api.JobLog
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	ts.NoError(err)
	ts.Len(logs, 8)
	for _, log := range logs {
		ts.Equal(testID2, *log.JobID)
		ts.Contains([]string{"experience-worker.log", "metrics-worker.log", "experience-container.log", "metrics-container.log", "resource_metrics.binproto", "logs.zip", "file.name", "test_length_metric.binproto"}, *log.FileName)
	}

	// Download a single test log
	tempDir, err := os.MkdirTemp("", "test-logs")
	ts.NoError(err)
	output = s.runCommand(ts, downloadLogs(projectID, batchIDString, testID2.String(), tempDir, []string{"file.name"}), ExpectNoError)
	ts.Contains(output.StdOut, fmt.Sprintf("Downloaded 1 log(s) to %s", tempDir))

	// Download all test logs:
	output = s.runCommand(ts, downloadLogs(projectID, batchIDString, testID2.String(), tempDir, []string{}), ExpectNoError)
	ts.Contains(output.StdOut, fmt.Sprintf("Downloaded 8 log(s) to %s", tempDir))

	// Check that the logs were downloaded and unzipped:
	files, err := os.ReadDir(tempDir)
	ts.NoError(err)
	ts.Len(files, 8)
	for _, file := range files {
		ts.Contains([]string{"experience-worker.log", "metrics-worker.log", "experience-container.log", "metrics-container.log", "resource_metrics.binproto", "logs", "file.name", "test_length_metric.binproto"}, file.Name())
	}

	// Pass blank name / id to logs:
	output = s.runCommand(ts, listLogs(projectID, "not-a-uuid", testID2.String()), ExpectError)
	ts.Contains(output.StdErr, InvalidBatchID)
	output = s.runCommand(ts, listLogs(projectID, batchIDString, "not-a-uuid"), ExpectError)
	ts.Contains(output.StdErr, InvalidTestID)

	// Ensure the rest of the batches complete:
	ts.Eventually(func() bool {
		allComplete := true
		for _, batchIDString := range []string{batchIDStringGH1, batchIDStringGH2, batchIDStringGH3, batchIDStringGH4} {
			cmd := s.buildCommand(getBatchByID(projectID, batchIDString, ExitStatusTrue))
			var stdout, stderr bytes.Buffer
			fmt.Println("About to run command: ", cmd.String())
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			exitCode := 0
			if err := cmd.Run(); err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				}
			}
			ts.Contains(AcceptableBatchStatusCodes, exitCode)
			ts.Empty(stderr.String())
			ts.Empty(stdout.String())
			// Check if the status is 0, complete, 5 cancelled, 2 failed
			complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
			if !complete {
				fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
			} else {
				fmt.Println("Batch completed, with exitCode:", exitCode)
			}
			allComplete = allComplete && complete
		}
		return allComplete
	}, 10*time.Minute, 10*time.Second)
	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}
func TestRerunBatch(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// This test does not use parameters, so we create an empty parameter map:
	emptyParameterMap := map[string]string{}
	// First create two experiences:
	experienceName1 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/test-object/", s.Config.E2EBucket)
	output = s.runCommand(ts, createExperience(projectID, experienceName1, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]

	experienceName2 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experienceName2, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/resim/open-builds/log-ingest:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]

	// Create a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]

	// Create a batch with an ID in the --experiences flag and a tag name in the --experience-tags flag (experience 1 is in the tag)
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceIDString2, experienceIDString1}, []string{}, metricsBuildIDString, GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)

	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByID(projectID, batchIDString, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 failed
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(batchIDString, batch.BatchID.String())
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Get the job IDs:
	output = s.runCommand(ts, getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	var tests []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &tests)
	ts.NoError(err)
	ts.Len(tests, 2)
	// Now rerun the batch:
	// Sleep for 60 seconds to ensure the batch is cleaned up:
	time.Sleep(60 * time.Second)
	output = s.runCommand(ts, rerunBatch(projectID, batchIDString, []string{}), ExpectNoError)
	ts.Contains(output.StdOut, "Batch rerun successfully!")

	// Await it finishing:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByID(projectID, batchIDString, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 failed
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)

	// Assert the status is SUCCEEDED:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)

	// Now rerun the batch:
	// Sleep for 60 seconds to ensure the batch is cleaned up:
	time.Sleep(60 * time.Second)
	output = s.runCommand(ts, rerunBatch(projectID, batchIDString, []string{tests[0].JobID.String()}), ExpectNoError)
	ts.Contains(output.StdOut, "Batch rerun successfully!")

	// Await it finishing:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByID(projectID, batchIDString, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 failed
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)

	// Assert the status is SUCCEEDED:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}
func TestParameterizedBatch(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	expectedParameterMap := map[string]string{
		"param1": "value1",
		"param2": "value2",
	}
	// First create an experience:
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString1)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create a batch with (only) experience names using the --experiences flag with some parameters
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName}, []string{}, metricsBuildIDString, GithubTrue, expectedParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDStringGH)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByID(projectID, batchIDStringGH, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 failed
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status by ID:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDStringGH, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(batchID, *batch.BatchID)
	// Validate that it succeeded:
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	ts.Equal(api.BatchParameters(expectedParameterMap), *batch.Parameters)
	ts.Equal(buildID, *batch.BuildID)
	ts.Equal(metricsBuildID, *batch.MetricsBuildID)

	// Archive the project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}
func TestCreateSweepParameterNameAndValues(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// create two experiences:
	experienceName1 := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(ts, createExperience(projectID, experienceName1, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experienceName2, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Archive the experiences when possible

	// Now create the branch:
	branchName := fmt.Sprintf("sweep-test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Create an experience tag:
	tagName := fmt.Sprintf("test-tag-%s", uuid.New().String())
	output = s.runCommand(ts, createExperienceTag(projectID, tagName, "testing tag"), ExpectNoError)

	// List experience tags and check the list contains the tag we just created
	output = s.runCommand(ts, listExperienceTags(projectID), ExpectNoError)
	ts.Contains(output.StdOut, tagName)

	// Tag one of the experiences:
	output = s.runCommand(ts, tagExperience(projectID, tagName, experienceID1), ExpectNoError)

	// Adding the same tag again should error:
	output = s.runCommand(ts, tagExperience(projectID, tagName, experienceID1), ExpectError)

	// List experiences for the tag
	output = s.runCommand(ts, listExperiencesWithTag(projectID, tagName), ExpectNoError)
	var tagExperiences []api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	ts.NoError(err)
	ts.Equal(1, len(tagExperiences))

	// Tag 2nd, check list contains 2 experiences
	output = s.runCommand(ts, tagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(ts, listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	ts.NoError(err)
	ts.Equal(2, len(tagExperiences))

	// Define the parameters:
	parameterName := "test-parameter"
	parameterValues := []string{"value1", "value2", "value3"}
	// Create a sweep with (only) experience names using the --experiences flag and specific parameter name and values (and "" for no config file location)
	output = s.runCommand(ts, createSweep(projectID, buildIDString, []string{experienceName1, experienceName2}, []string{}, metricsBuildIDString, parameterName, parameterValues, "", GithubTrue, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSweep)
	sweepIDStringGH := output.StdOut[len(GithubCreatedSweep) : len(output.StdOut)-1]
	uuid.MustParse(sweepIDStringGH)

	// Create a sweep with (only) experience tag names using the --experience-tags flag and a config file location
	// The config file location is in a subdirectory of the testing directory called 'data' and is called valid_sweep_config.json:
	// Find the current working directory:
	cwd, err := os.Getwd()
	ts.NoError(err)
	// Create the config location:
	configLocation := fmt.Sprintf("%s/data/valid_sweep_config.json", cwd)
	output = s.runCommand(ts, createSweep(projectID, buildIDString, []string{}, []string{tagName}, "", "", []string{}, configLocation, GithubFalse, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, CreatedSweep)
	ts.Empty(output.StdErr)

	// Extract from "Sweep ID:" to the next newline:
	re := regexp.MustCompile(`Sweep ID: (.+?)\n`)
	matches := re.FindStringSubmatch(output.StdOut)
	ts.Equal(2, len(matches))
	secondSweepIDString := strings.TrimSpace(matches[1])
	secondSweepID := uuid.MustParse(secondSweepIDString)
	// Extract the sweep name:
	re = regexp.MustCompile(`Sweep name: (.+?)\n`)
	matches = re.FindStringSubmatch(output.StdOut)
	ts.Equal(2, len(matches))
	sweepNameString := strings.TrimSpace(matches[1])
	// RePun:
	sweepNameParts := strings.Split(sweepNameString, "-")
	ts.Equal(3, len(sweepNameParts))
	// Try a sweep without any experiences:
	output = s.runCommand(ts, createSweep(projectID, buildIDString, []string{}, []string{}, "", parameterName, parameterValues, "", GithubFalse, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, FailedToCreateSweep)
	// Try a sweep without a build id:
	output = s.runCommand(ts, createSweep(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, "", parameterName, parameterValues, "", GithubFalse, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, InvalidBuildID)

	// Try a sweep with both parameter name and config (even if fake):
	output = s.runCommand(ts, createSweep(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, "", parameterName, parameterValues, "config location", GithubFalse, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, ConfigParamsMutuallyExclusive)

	// Try a sweep with an invalid config
	invalidConfigLocation := fmt.Sprintf("%s/data/invalid_sweep_config.json", cwd)
	output = s.runCommand(ts, createSweep(projectID, buildIDString, []string{}, []string{tagName}, "", "", []string{}, invalidConfigLocation, GithubFalse, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, InvalidGridSearchFile)

	// Get sweep passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the sweep, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getSweepByName(projectID, sweepNameString, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableSweepStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 2 failed
		complete := (exitCode == 0 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for sweep completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Sweep completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the sweep and validate the status, first by name then by ID:
	output = s.runCommand(ts, getSweepByName(projectID, sweepNameString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var sweep api.ParameterSweep
	err = json.Unmarshal([]byte(output.StdOut), &sweep)
	ts.NoError(err)
	ts.Equal(sweepNameString, *sweep.Name)
	ts.Equal(secondSweepID, *sweep.ParameterSweepID)
	// Validate that it succeeded:
	ts.Equal(api.ParameterSweepStatusSUCCEEDED, *sweep.Status)
	// Get the sweep by ID:
	output = s.runCommand(ts, getSweepByID(projectID, secondSweepIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &sweep)
	ts.NoError(err)
	ts.Equal(sweepNameString, *sweep.Name)
	ts.Equal(secondSweepID, *sweep.ParameterSweepID)
	// Validate that it succeeded:
	ts.Equal(api.ParameterSweepStatusSUCCEEDED, *sweep.Status)

	// Validate that the sweep has the correct parameters:
	passedParameters := []api.SweepParameter{}
	// Read from the valid config file:
	configFile, err := os.Open(configLocation)
	ts.NoError(err)
	defer configFile.Close()
	byteValue, err := io.ReadAll(configFile)
	ts.NoError(err)
	err = json.Unmarshal(byteValue, &passedParameters)
	ts.NoError(err)
	ts.Equal(passedParameters, *sweep.Parameters)
	// Figure out how many batches to expect:
	numBatches := 1
	for _, param := range passedParameters {
		numBatches *= len(*param.Values)
	}
	ts.Len(*sweep.Batches, numBatches)

	// Pass blank name / id to batches get:
	output = s.runCommand(ts, getSweepByName(projectID, "", ExitStatusFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidSweepNameOrID)
	output = s.runCommand(ts, getSweepByID(projectID, "", ExitStatusFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidSweepNameOrID)

	// Check we can list the sweeps, and our new sweep is in it:
	output = s.runCommand(ts, listSweeps(projectID), ExpectNoError)
	ts.Contains(output.StdOut, sweepNameString)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

// Test Cancel Sweep::
func TestCancelSweep(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// create an experience:
	experienceName := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
	// Now create the branch:
	branchName := fmt.Sprintf("sweep-test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Define the parameters:
	parameterName := "test-parameter"
	parameterValues := []string{"value1", "value2", "value3"}
	// Create a sweep with (only) experience names using the --experiences flag and specific parameter name and values (and "" for no config file location)
	output = s.runCommand(ts, createSweep(projectID, buildIDString, []string{experienceName}, []string{}, metricsBuildIDString, parameterName, parameterValues, "", GithubTrue, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSweep)
	sweepIDStringGH := output.StdOut[len(GithubCreatedSweep) : len(output.StdOut)-1]
	uuid.MustParse(sweepIDStringGH)

	time.Sleep(30 * time.Second) // arbitrary sleep to make sure the scheduler gets the batch and triggers it

	// Cancel the sweep:
	output = s.runCommand(ts, cancelSweep(projectID, sweepIDStringGH), ExpectNoError)
	ts.Contains(output.StdOut, CancelledSweep)

	// Get sweep passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the sweep, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getSweepByID(projectID, sweepIDStringGH, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		fmt.Println("stdout:")
		fmt.Println(stdout.String())
		fmt.Println("stderr:")
		fmt.Println(stderr.String())
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableSweepStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 2 failed
		complete := (exitCode == 0 || exitCode == 2 || exitCode == 5)
		if !complete {
			fmt.Println("Waiting for sweep completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Sweep completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the sweep and validate the status, first by name then by ID:
	output = s.runCommand(ts, getSweepByID(projectID, sweepIDStringGH, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var sweep api.ParameterSweep
	err := json.Unmarshal([]byte(output.StdOut), &sweep)
	ts.NoError(err)
	ts.Equal(sweepIDStringGH, sweep.ParameterSweepID.String())
	// Validate that it was cancelled:
	ts.Equal(api.ParameterSweepStatusCANCELLED, *sweep.Status)
}

// Test the metrics builds:
func TestCreateMetricsBuild(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing metrics build creation")
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create two systems to add as part of the experience creation:
	systemName1 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName1, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString1 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString1)
	systemName2 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName2, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString2 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString2)

	systemNames := []string{systemName1, systemName2}
	metricsBuildName := fmt.Sprintf("metrics-build-%s", uuid.New().String())
	output = s.runCommand(ts, createMetricsBuild(projectID, metricsBuildName, "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", systemNames, GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, CreatedMetricsBuild)
	ts.Empty(output.StdErr)
	// Validate that the metrics build is available for each system:
	for _, systemName := range systemNames {
		output = s.runCommand(ts, systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
		ts.Contains(output.StdOut, metricsBuildName)
	}
	// Verify that each of the required flags are required:
	output = s.runCommand(ts, createMetricsBuild(projectID, "", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyMetricsBuildName)
	output = s.runCommand(ts, createMetricsBuild(projectID, "name", "", "1.0.0", EmptySlice, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyMetricsBuildImage)
	output = s.runCommand(ts, createMetricsBuild(projectID, "name", "public.ecr.aws/docker/library/hello-world:latest", "", EmptySlice, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, EmptyMetricsBuildVersion)
	output = s.runCommand(ts, createMetricsBuild(projectID, "name", "public.ecr.aws/docker/library/hello-world", "1.1.1", EmptySlice, GithubFalse), ExpectError)
	ts.Contains(output.StdErr, InvalidMetricsBuildImage)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestMetricsBuildGithub(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing metrics build creation, with --github flag")
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	output = s.runCommand(ts, createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Check we can list the metrics builds, and our new metrics build is in it:
	output = s.runCommand(ts, listMetricsBuilds(projectID), ExpectNoError)
	ts.Contains(output.StdOut, metricsBuildIDString)

	// Archive the project:
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}
func TestAliases(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing project and branch aliases")
	// First create a project, manually:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now get the project using the old aliased commands:
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "project",
	}
	getByNameCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
		},
	}
	getByIDCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID.String(),
			},
		},
	}
	output = s.runCommand(ts, []CommandBuilder{projectCommand, getByNameCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	// Marshal into a struct:
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	ts.NoError(err)
	ts.Equal(projectName, project.Name)
	ts.Equal(projectID, project.ProjectID)
	// Try with the ID:
	output = s.runCommand(ts, []CommandBuilder{projectCommand, getByIDCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &project)
	ts.NoError(err)
	ts.Equal(projectName, project.Name)
	ts.Equal(projectID, project.ProjectID)

	// Now create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Empty(output.StdErr)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// We list branches by project-id and project-name to test the aliasing:
	branchCommand := CommandBuilder{
		Command: "branch",
	}
	listBranchesByNameCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project-name",
				Value: projectName,
			},
		},
	}
	listBranchesByIDCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID.String(),
			},
		},
	}
	output = s.runCommand(ts, []CommandBuilder{branchCommand, listBranchesByNameCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	// Marshal into a struct:
	var branches []api.Branch
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	ts.NoError(err)
	ts.Equal(1, len(branches))
	ts.Equal(branchName, branches[0].Name)
	ts.Equal(branchID, branches[0].BranchID)
	ts.Equal(projectID, branches[0].ProjectID)
	// Now try by ID:
	output = s.runCommand(ts, []CommandBuilder{branchCommand, listBranchesByIDCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	ts.NoError(err)
	ts.Equal(1, len(branches))
	ts.Equal(branchName, branches[0].Name)
	ts.Equal(branchID, branches[0].BranchID)
	ts.Equal(projectID, branches[0].ProjectID)

	// Now create a build:
	buildCommand := CommandBuilder{
		Command: "builds",
	}
	createBuildWithNamesCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project-name",
				Value: projectName,
			},
			{
				Name:  "--branch-name",
				Value: branchName,
			},
			{
				Name:  "--system",
				Value: systemName,
			},
			{
				Name:  "--description",
				Value: "description",
			},
			{
				Name:  "--image",
				Value: "public.ecr.aws/docker/library/hello-world:latest",
			},
			{
				Name:  "--version",
				Value: "version",
			},
		},
	}
	createBuildWithIDCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID.String(),
			},
			{
				Name:  "--branch-name",
				Value: branchName,
			},
			{
				Name:  "--system",
				Value: systemName,
			},
			{
				Name:  "--name",
				Value: "build-name",
			},
			{
				Name:  "--description",
				Value: "description",
			},
			{
				Name:  "--image",
				Value: "public.ecr.aws/docker/library/hello-world:latest",
			},
			{
				Name:  "--version",
				Value: "version",
			},
		},
	}

	output = s.runCommand(ts, []CommandBuilder{buildCommand, createBuildWithNamesCommand}, ExpectNoError)
	ts.Contains(output.StdErr, "Warning: Using 'description' to set the build name is deprecated. In the future, 'description' will only set the build's description. Please use --name instead.")
	ts.Contains(output.StdOut, CreatedBuild)

	// Now try to create using the id for projects:
	output = s.runCommand(ts, []CommandBuilder{buildCommand, createBuildWithIDCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	ts.Contains(output.StdOut, CreatedBuild)

	// Now, list build with ID and name
	listBuildByNameCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project-name",
				Value: projectName,
			},
			{
				Name:  "--branch-name",
				Value: branchName,
			},
		},
	}
	listBuildByIDCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID.String(),
			},
			{
				Name:  "--branch-name",
				Value: branchName,
			},
		},
	}
	// List by name
	output = s.runCommand(ts, []CommandBuilder{buildCommand, listBuildByNameCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	// Marshal into a struct:
	var builds []api.Build
	err = json.Unmarshal([]byte(output.StdOut), &builds)
	ts.NoError(err)
	ts.Equal(2, len(builds))
	// List by id
	output = s.runCommand(ts, []CommandBuilder{buildCommand, listBuildByIDCommand}, ExpectNoError)
	ts.Empty(output.StdErr)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &builds)
	ts.NoError(err)
	ts.Equal(2, len(builds))

	// Archive the project, using the aliased command:
	archiveProjectCommand := CommandBuilder{
		Command: "project",
	}
	archiveProjectByIDCommand := CommandBuilder{
		Command: "archive",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID.String(),
			},
		},
	}
	output = s.runCommand(ts, []CommandBuilder{archiveProjectCommand, archiveProjectByIDCommand}, ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)

	// Finally, create a new project to verify deletion with the old 'name' flag:
	projectName = fmt.Sprintf("test-project-%s", uuid.New().String())
	output = s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Empty(output.StdErr)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString = output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	uuid.MustParse(projectIDString)
	// Archive the project, using the aliased command:
	archiveProjectByNameCommand := CommandBuilder{
		Command: "archive",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
		},
	}
	output = s.runCommand(ts, []CommandBuilder{archiveProjectCommand, archiveProjectByNameCommand}, ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestTestSuites(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing test suites")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	const systemDescription = "test system description"
	const buildVCPUs = 1
	const buildGPUs = 0
	const buildMemoryMiB = 1000
	const buildSharedMemoryMB = 64
	const metricsBuildVCPUs = 2
	const metricsBuildGPUs = 0
	const metricsBuildMemoryMiB = 900
	const metricsBuildSharedMemoryMB = 1024
	output = s.runCommand(ts, createSystem(projectIDString, systemName, systemDescription, Ptr(buildVCPUs), Ptr(buildGPUs), Ptr(buildMemoryMiB), Ptr(buildSharedMemoryMB), Ptr(metricsBuildVCPUs), Ptr(metricsBuildGPUs), Ptr(metricsBuildMemoryMiB), Ptr(metricsBuildSharedMemoryMB), nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)
	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)

	// Now create a few experiences:
	NUM_EXPERIENCES := 4
	experienceNames := make([]string, NUM_EXPERIENCES)
	experienceIDs := make([]uuid.UUID, NUM_EXPERIENCES)
	for i := 0; i < NUM_EXPERIENCES; i++ {
		experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
		experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
		output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, []string{systemName}, EmptySlice, nil, nil, []string{}, GithubTrue), ExpectNoError)
		ts.Contains(output.StdOut, GithubCreatedExperience)
		ts.Empty(output.StdErr)
		// We expect to be able to parse the experience ID as a UUID
		experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
		experienceID := uuid.MustParse(experienceIDString)
		experienceNames[i] = experienceName
		experienceIDs[i] = experienceID
	}

	// Finally, a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Now, create a test suite with all our experiences, the system, and a metrics build:
	firstTestSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	testSuiteDescription := "test suite description"
	output = s.runCommand(ts, createTestSuite(projectID, firstTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubFalse, nil), ExpectNoError)
	ts.Contains(output.StdOut, CreatedTestSuite)
	// Try with the github flag:
	secondTestSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	output = s.runCommand(ts, createTestSuite(projectID, secondTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubTrue, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedTestSuite)
	// Parse the output
	testSuiteIDRevisionString := output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	// Split into the UUID and revision:
	testSuiteIDRevision := strings.Split(testSuiteIDRevisionString, "/")
	ts.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision := testSuiteIDRevision[1]
	ts.Equal("0", revision)

	// Failure possibilities:
	// Try to create a test suite with an empty system:
	output = s.runCommand(ts, createTestSuite(projectID, "test-suite", "description", "", experienceNames, metricsBuildIDString, GithubFalse, nil), ExpectError)
	ts.Contains(output.StdErr, EmptyTestSuiteSystemName)
	output = s.runCommand(ts, createTestSuite(projectID, "", "description", systemName, experienceNames, metricsBuildIDString, GithubFalse, nil), ExpectError)
	ts.Contains(output.StdErr, EmptyTestSuiteName)
	output = s.runCommand(ts, createTestSuite(projectID, "test-suite", "", systemName, experienceNames, metricsBuildIDString, GithubFalse, nil), ExpectError)
	ts.Contains(output.StdErr, EmptyTestSuiteDescription)
	output = s.runCommand(ts, createTestSuite(projectID, "test-suite", "description", systemName, []string{}, metricsBuildIDString, GithubFalse, nil), ExpectError)
	ts.Contains(output.StdErr, EmptyTestSuiteExperiences)
	output = s.runCommand(ts, createTestSuite(projectID, "test-suite", "description", systemName, experienceNames, "not-a-uuid", GithubFalse, nil), ExpectError)
	ts.Contains(output.StdErr, EmptyTestSuiteMetricsBuild)

	// Revise the test suite:
	// Now, create a test suite with all our experiences, the system, and a metrics build:
	output = s.runCommand(ts, reviseTestSuite(projectID, firstTestSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, nil, nil, GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, RevisedTestSuite)
	// Revise w/ github flag
	output = s.runCommand(ts, reviseTestSuite(projectID, firstTestSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedTestSuite)
	testSuiteIDRevisionString = output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	testSuiteIDRevision = strings.Split(testSuiteIDRevisionString, "/")
	ts.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision = testSuiteIDRevision[1]
	ts.Equal("2", revision)

	// Now list the test suites
	output = s.runCommand(ts, listTestSuites(projectID), ExpectNoError)
	// Parse the output into a list of test suites:
	var testSuites []api.TestSuite
	err := json.Unmarshal([]byte(output.StdOut), &testSuites)
	ts.NoError(err)
	ts.Len(testSuites, 2)
	ts.Equal(firstTestSuiteName, testSuites[0].Name)
	ts.Contains(secondTestSuiteName, testSuites[1].Name)
	// Then get a specific revision etc
	zerothRevision := int32(0)
	output = s.runCommand(ts, getTestSuite(projectID, firstTestSuiteName, Ptr(zerothRevision), false), ExpectNoError)
	// Parse the output into a test suite:
	var testSuite api.TestSuite
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.Equal(firstTestSuiteName, testSuite.Name)
	ts.Equal(zerothRevision, testSuite.TestSuiteRevision)
	ts.ElementsMatch(experienceIDs, testSuite.Experiences)
	secondRevision := int32(2)
	output = s.runCommand(ts, getTestSuite(projectID, firstTestSuiteName, Ptr(secondRevision), false), ExpectNoError)
	// Parse the output into a test suite:
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.Equal(firstTestSuiteName, testSuite.Name)
	ts.Equal(secondRevision, testSuite.TestSuiteRevision)
	ts.Len(testSuite.Experiences, 1)
	ts.ElementsMatch(experienceIDs[0], testSuite.Experiences[0])
	// Then run.
	output = s.runCommand(ts, runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubFalse, AssociatedAccount, nil, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, CreatedTestSuiteBatch)
	// Then list the test suite batches
	output = s.runCommand(ts, getTestSuiteBatches(projectID, firstTestSuiteName, nil), ExpectNoError)
	// Parse the output into a list of test suite batches:
	var testSuiteBatches []api.Batch
	err = json.Unmarshal([]byte(output.StdOut), &testSuiteBatches)
	ts.NoError(err)
	ts.Len(testSuiteBatches, 1)
	// Then get the test suite batch
	batch := testSuiteBatches[0]
	ts.Equal(buildIDString, batch.BuildID.String())

	// Create a new run using github and with a specific batch name:
	batchName := fmt.Sprintf("test-batch-%s", uuid.New().String())
	output = s.runCommand(ts, runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubTrue, AssociatedAccount, &batchName, Ptr(100), nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	// Parse the output to get a batch id:
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)
	// Get the batch:
	output = s.runCommand(ts, getBatchByName(projectID, batchName, ExitStatusFalse), ExpectNoError)
	ts.Contains(output.StdOut, batchName)
	ts.Contains(output.StdOut, batchIDString)
	// Then list the test suite batches
	output = s.runCommand(ts, getTestSuiteBatches(projectID, firstTestSuiteName, nil), ExpectNoError)
	// Parse the output into a list of test suite batches:
	err = json.Unmarshal([]byte(output.StdOut), &testSuiteBatches)
	ts.NoError(err)
	ts.Len(testSuiteBatches, 2)
	found := false
	for _, batch := range testSuiteBatches {
		ts.Equal(buildIDString, batch.BuildID.String())
		if batch.BatchID.String() == batchIDString {
			found = true
		}
	}
	ts.True(found)

	// Try running a test suite with a non-percentage allowable failure percent:
	output = s.runCommand(ts, runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubFalse, AssociatedAccount, nil, Ptr(101), nil), ExpectError)
	ts.Contains(output.StdErr, AllowableFailurePercent)
	output = s.runCommand(ts, runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubFalse, AssociatedAccount, nil, Ptr(-1), nil), ExpectError)
	ts.Contains(output.StdErr, AllowableFailurePercent)

	// Try running a test suite with a metrics build override:
	output = s.runCommand(ts, runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubTrue, AssociatedAccount, nil, nil, Ptr(metricsBuildIDString)), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	// Parse the output to get a batch id:
	batchIDString = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)
	// Get the batch:
	output = s.runCommand(ts, getBatchByName(projectID, batchName, ExitStatusFalse), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(metricsBuildIDString, batch.MetricsBuildID.String())
	ts.Equal(buildIDString, batch.BuildID.String())
	// Get the jobs for the batch:
	output = s.runCommand(ts, getBatchJobsByName(projectID, batchName), ExpectNoError)
	jobs := []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	ts.NoError(err)
	ts.Len(jobs, 1)
	// The job should have the correct experience ID:
	ts.Equal(experienceIDs[0], *jobs[0].ExperienceID)

	// Archive the test suite
	output = s.runCommand(ts, archiveTestSuite(projectID, firstTestSuiteName), false)
	ts.Contains(output.StdOut, "Archived test suite "+firstTestSuiteName+" successfully!")

	// Restore the test suite
	output = s.runCommand(ts, restoreTestSuite(projectID, firstTestSuiteName), false)
	ts.Contains(output.StdOut, "Restored archived test suite "+firstTestSuiteName+" successfully!")

	// Get the test suite again to verify it's restored
	output = s.runCommand(ts, getTestSuite(projectID, firstTestSuiteName, nil, false), false)
	ts.Contains(output.StdOut, firstTestSuiteName)
	ts.Contains(output.StdOut, testSuiteDescription)

	// Create and test a test suite using the metrics set:
	metricsSetName := fmt.Sprintf("metrics-set-%s", uuid.New().String())
	metricsSetTestSuiteName := fmt.Sprintf("metrics-set-test-suite-%s", uuid.New().String())
	output = s.runCommand(ts, createTestSuite(projectID, metricsSetTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubFalse, &metricsSetName), ExpectNoError)
	ts.Contains(output.StdOut, CreatedTestSuite)
	// Verify metrics set name is stored
	output = s.runCommand(ts, getTestSuite(projectID, metricsSetTestSuiteName, nil, false), false)
	ts.Contains(output.StdOut, metricsSetTestSuiteName)
	ts.Contains(output.StdOut, testSuiteDescription)
	// Parse the output into a test suite:
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.NotNil(testSuite.MetricsSetName)
	ts.Equal(metricsSetName, *testSuite.MetricsSetName)

	// Now revise the test suite to clear the metrics set (set to nil), then set a new one
	// First clear
	emptyMetricsSet := ""
	output = s.runCommand(ts, reviseTestSuite(projectID, metricsSetTestSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, nil, &emptyMetricsSet, GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, RevisedTestSuite)
	output = s.runCommand(ts, getTestSuite(projectID, metricsSetTestSuiteName, nil, false), false)
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.NotNil(testSuite.MetricsSetName)
	ts.Equal(emptyMetricsSet, *testSuite.MetricsSetName)

	// Then set a new metrics set name
	newMetricsSetName := fmt.Sprintf("metrics-set-%s", uuid.New().String())
	output = s.runCommand(ts, reviseTestSuite(projectID, metricsSetTestSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, nil, &newMetricsSetName, GithubFalse), ExpectNoError)
	ts.Contains(output.StdOut, RevisedTestSuite)
	output = s.runCommand(ts, getTestSuite(projectID, metricsSetTestSuiteName, nil, false), false)
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.NotNil(testSuite.MetricsSetName)
	ts.Equal(newMetricsSetName, *testSuite.MetricsSetName)
}
func TestReports(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	fmt.Println("Testing reports")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	const systemDescription = "test system description"
	const buildVCPUs = 1
	const buildGPUs = 0
	const buildMemoryMiB = 1000
	const buildSharedMemoryMB = 64
	const metricsBuildVCPUs = 2
	const metricsBuildGPUs = 0
	const metricsBuildMemoryMiB = 900
	const metricsBuildSharedMemoryMB = 1024
	output = s.runCommand(ts, createSystem(projectIDString, systemName, systemDescription, Ptr(buildVCPUs), Ptr(buildGPUs), Ptr(buildMemoryMiB), Ptr(buildSharedMemoryMB), Ptr(metricsBuildVCPUs), Ptr(metricsBuildGPUs), Ptr(metricsBuildMemoryMiB), Ptr(metricsBuildSharedMemoryMB), nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)
	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)
	// Now create a few experiences:
	NUM_EXPERIENCES := 2
	experienceNames := make([]string, NUM_EXPERIENCES)
	experienceIDs := make([]uuid.UUID, NUM_EXPERIENCES)
	for i := 0; i < NUM_EXPERIENCES; i++ {
		experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
		experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
		output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, []string{systemName}, EmptySlice, nil, nil, []string{}, GithubTrue), ExpectNoError)
		ts.Contains(output.StdOut, GithubCreatedExperience)
		ts.Empty(output.StdErr)
		// We expect to be able to parse the experience ID as a UUID
		experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
		experienceID := uuid.MustParse(experienceIDString)
		experienceNames[i] = experienceName
		experienceIDs[i] = experienceID
	}

	// Finally, a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Now, create a test suite with all our experiences, the system, and a metrics build:
	testSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	testSuiteDescription := "test suite description"
	output = s.runCommand(ts, createTestSuite(projectID, testSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubFalse, nil), ExpectNoError)
	ts.Contains(output.StdOut, CreatedTestSuite)
	// Try with the github flag:
	secondTestSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	output = s.runCommand(ts, createTestSuite(projectID, secondTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubTrue, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedTestSuite)
	// Parse the output
	testSuiteIDRevisionString := output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	// Split into the UUID and revision:
	testSuiteIDRevision := strings.Split(testSuiteIDRevisionString, "/")
	ts.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision := testSuiteIDRevision[1]
	ts.Equal("0", revision)

	// Get the test suite and validate the show on summary flag is false:
	output = s.runCommand(ts, getTestSuite(projectID, testSuiteName, Ptr(int32(0)), false), ExpectNoError)
	// Parse the output into a test suite:
	var testSuite api.TestSuite
	err := json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.False(testSuite.ShowOnSummary)

	// Revise w/ github flag
	output = s.runCommand(ts, reviseTestSuite(projectID, testSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, Ptr(true), nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedTestSuite)
	testSuiteIDRevisionString = output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	testSuiteIDRevision = strings.Split(testSuiteIDRevisionString, "/")
	ts.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision = testSuiteIDRevision[1]
	ts.Equal("1", revision)
	// Get the test suite and validate the show on summary flag is true:
	output = s.runCommand(ts, getTestSuite(projectID, testSuiteName, Ptr(int32(1)), false), ExpectNoError)
	// Parse the output into a test suite:
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	ts.NoError(err)
	ts.True(testSuite.ShowOnSummary)

	// 1. Create a report passing in length and have it correct [with name, with respect revision and explicit revision 0]
	reportName := fmt.Sprintf("test-report-%s", uuid.New().String())
	output = s.runCommand(ts, createReport(projectID, testSuiteName, Ptr(int32(0)), branchName, metricsBuildIDString, Ptr(28), nil, nil, Ptr(true), Ptr(reportName), GithubFalse, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, CreatedReport)
	ts.Contains(output.StdOut, EndTimestamp)
	ts.Contains(output.StdOut, StartTimestamp)
	// Create a second with the github flag and validate the timestamps are correct:
	reportName = fmt.Sprintf("test-report-%s", uuid.New().String())
	output = s.runCommand(ts, createReport(projectID, testSuiteName, Ptr(int32(0)), branchName, metricsBuildIDString, Ptr(28), nil, nil, Ptr(true), Ptr(reportName), GithubTrue, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedReport)
	// Extract the report id:
	reportIDString := output.StdOut[len(GithubCreatedReport) : len(output.StdOut)-1]
	uuid.MustParse(reportIDString)
	// Get the most recent report, by name:
	output = s.runCommand(ts, getReportByName(projectID, reportName, false), ExpectNoError)
	// Parse the output into a report:
	var report api.Report
	err = json.Unmarshal([]byte(output.StdOut), &report)
	ts.NoError(err)
	ts.Equal(reportName, report.Name)
	// Validate that the start timestamp is 4 weeks before today within a one minute duration
	ts.WithinDuration(time.Now().UTC().Add(-4*7*24*time.Hour), report.StartTimestamp, time.Minute)
	// Validate the end timestamp is pretty much now:
	ts.WithinDuration(time.Now().UTC(), report.EndTimestamp, time.Minute)
	ts.Equal(int32(0), report.TestSuiteRevision)
	ts.True(report.RespectRevisionBoundary)

	// 2. Create a report passing in start and end timestamps and have them correct [with name, with respect revision and implicit revision 1]
	reportName = fmt.Sprintf("test-report-%s", uuid.New().String())
	startTimestamp := time.Now().UTC().Add(-time.Hour)
	endTimestamp := time.Now().UTC().Add(-5 * time.Minute)
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, Ptr(startTimestamp.Format(time.RFC3339)), Ptr(endTimestamp.Format(time.RFC3339)), Ptr(true), Ptr(reportName), GithubTrue, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedReport)
	// Extract the report id:
	reportIDString = output.StdOut[len(GithubCreatedReport) : len(output.StdOut)-1]
	uuid.MustParse(reportIDString)
	// Get the most recent report, by name:
	output = s.runCommand(ts, getReportByName(projectID, reportName, false), ExpectNoError)
	// Parse the output into a report:
	err = json.Unmarshal([]byte(output.StdOut), &report)
	ts.NoError(err)
	ts.Equal(reportName, report.Name)
	// Validate that the start timestamp is correct within a second
	ts.WithinDuration(startTimestamp, report.StartTimestamp, time.Second)
	// Validate the end timestamp is correct within a second
	ts.WithinDuration(endTimestamp, report.EndTimestamp, time.Second)
	ts.Equal(int32(1), report.TestSuiteRevision)
	ts.True(report.RespectRevisionBoundary)

	// 3. Create a report passing in only start timestamp and have it correct [no name, no respect revision and implicit revision 1]
	newStartTimestamp := time.Now().UTC().Add(-time.Hour)
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, Ptr(newStartTimestamp.Format(time.RFC3339)), nil, nil, nil, GithubTrue, AssociatedAccount), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedReport)
	// Extract the report id:
	reportIDString = output.StdOut[len(GithubCreatedReport) : len(output.StdOut)-1]
	uuid.MustParse(reportIDString)
	// Get the most recent report, by name:
	output = s.runCommand(ts, getReportByID(projectID, reportIDString, false), ExpectNoError)
	// Parse the output into a report:
	err = json.Unmarshal([]byte(output.StdOut), &report)
	ts.NoError(err)
	ts.NotEqual("", report.Name)
	// Validate that the start timestamp is correct within a second
	ts.WithinDuration(newStartTimestamp, report.StartTimestamp, time.Second)
	// Validate the end timestamp is correct within a minute to now
	ts.WithinDuration(time.Now().UTC(), report.EndTimestamp, time.Minute)
	ts.Equal(int32(1), report.TestSuiteRevision)
	ts.False(report.RespectRevisionBoundary)

	// 4. Fail to create based on invalid timestamps
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, Ptr("invalid"), nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, FailedStartTimestamp)
	output = s.runCommand(ts, createReport(projectID, report.TestSuiteID.String(), nil, branchIDString, metricsBuildIDString, nil, Ptr(newStartTimestamp.Format(time.RFC3339)), Ptr("invalid"), nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, FailedEndTimestamp)
	// 6. Fail to create based on both end and length
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, Ptr(28), nil, Ptr(newStartTimestamp.Format(time.RFC3339)), nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, EndLengthMutuallyExclusive)
	// 7. Fail to create based on no start nor length
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, nil, Ptr(newStartTimestamp.Format(time.RFC3339)), nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, AtLeastOneReport)
	// 7. Fail to create, no test suite id, no branch id, no metrics build
	output = s.runCommand(ts, createReport(projectID, "", nil, "", "", Ptr(28), nil, nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, TestSuiteNameReport)
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, "", "", Ptr(28), nil, nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, BranchNotFoundReport)
	output = s.runCommand(ts, createReport(projectID, testSuiteName, nil, branchName, "", Ptr(28), nil, nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	ts.Contains(output.StdErr, FailedToParseMetricsBuildReport)

	//TODO(iain): check the wait and logs commands, once the rest has landed.
}

func TestWorkflows(t *testing.T) {
    ts := assert.New(t)
    t.Parallel()
    fmt.Println("Testing workflows and runs")

    // Create a project
    projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
    output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
    ts.Contains(output.StdOut, GithubCreatedProject)
    projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
    projectID := uuid.MustParse(projectIDString)

    // Create a system
    systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
    output = s.runCommand(ts, createSystem(projectIDString, systemName, "system for workflows", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectNoError)
    ts.Contains(output.StdOut, CreatedSystem)

    // Create a branch
    branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
    output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubFalse), ExpectNoError)
    ts.Contains(output.StdOut, CreatedBranch)

    // Create a build
    output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectNoError)
    ts.Contains(output.StdOut, CreatedBuild)
    buildIDString := output.StdOut[len(CreatedBuild)+len(" ") : len(output.StdOut)-1]
    // Above slice may not be reliable; instead, get by fetching the build list and parsing. Simpler: just list builds and pick latest.
    // Fallback to parsing via reports/test suites patterns when Github flag prints ID directly. For simplicity in E2E, create again with github to capture ID.
    output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.1", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
    ts.Contains(output.StdOut, GithubCreatedBuild)
    buildIDString = output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
    buildID := uuid.MustParse(buildIDString)

    // Create experiences and a metrics build to make a test suite
    expName := fmt.Sprintf("test-experience-%s", uuid.New().String())
    expLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
    output = s.runCommand(ts, createExperience(projectID, expName, "description", expLocation, []string{systemName}, EmptySlice, nil, nil, []string{}, GithubFalse), ExpectNoError)
    ts.Contains(output.StdOut, CreatedExperience)
    output = s.runCommand(ts, createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubFalse), ExpectNoError)
    ts.Contains(output.StdOut, CreatedMetricsBuild)
    ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
    metricsBuildID := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]

    // Create a test suite needed for workflow
    testSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
    output = s.runCommand(ts, createTestSuite(projectID, testSuiteName, "workflow test suite", systemName, []string{expName}, metricsBuildID, GithubFalse, nil), ExpectNoError)
    ts.Contains(output.StdOut, CreatedTestSuite)

    // Create workflow with one enabled suite
    suitesSpec := fmt.Sprintf("[{\"testSuite\":\"%s\",\"enabled\":true}]", testSuiteName)
    workflowName := fmt.Sprintf("test-workflow-%s", uuid.New().String())
    output = s.runCommand(ts, createWorkflow(projectID, workflowName, "desc", suitesSpec, ""), ExpectNoError)
    ts.Contains(output.StdOut, CreatedWorkflow)

    // List workflows and ensure present
    output = s.runCommand(ts, listWorkflowsCmd(projectID), ExpectNoError)
    ts.Contains(output.StdOut, workflowName)
    ts.Contains(output.StdOut, testSuiteName)

    // Get workflow
    output = s.runCommand(ts, getWorkflowCmd(projectID, workflowName), ExpectNoError)
    ts.Contains(output.StdOut, workflowName)
    ts.Contains(output.StdOut, testSuiteName)

    // Run workflow
    output = s.runCommand(ts, runWorkflowCmd(projectID, workflowName, buildID, map[string]string{"p1": "v1"}, []string{}, AssociatedAccount, Ptr(0)), ExpectNoError)
    ts.Contains(output.StdOut, CreatedWorkflowRun)
    // Parse workflow run ID from github path is not printed; we will list runs
    output = s.runCommand(ts, listWorkflowRunsCmd(projectID, workflowName), ExpectNoError)
    var runs []api.WorkflowRun
    err := json.Unmarshal([]byte(output.StdOut), &runs)
    ts.NoError(err)
    ts.NotEmpty(runs)
    // Get one run
    runID := runs[0].WorkflowRunID
    output = s.runCommand(ts, getWorkflowRunCmd(projectID, workflowName, runID), ExpectNoError)
    // Should print workflow run test suites JSON array
    ts.Contains(output.StdOut, testSuiteName)

    // Update workflow: change description and ensure success
    newDesc := "new description"
    output = s.runCommand(ts, updateWorkflowCmd(projectID, workflowName, nil, &newDesc, nil, nil, nil), ExpectNoError)
    ts.Contains(output.StdOut, UpdatedWorkflow)

    // Reconcile workflow suites: add, update enabled flags, and remove
    // Create two additional test suites to exercise add/remove/toggle
    secondSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
    output = s.runCommand(ts, createTestSuite(projectID, secondSuiteName, "second suite", systemName, []string{expName}, metricsBuildID, GithubFalse, nil), ExpectNoError)
    ts.Contains(output.StdOut, CreatedTestSuite)

    thirdSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
    output = s.runCommand(ts, createTestSuite(projectID, thirdSuiteName, "third suite", systemName, []string{expName}, metricsBuildID, GithubFalse, nil), ExpectNoError)
    ts.Contains(output.StdOut, CreatedTestSuite)

    // 1) Add thirdSuite to workflow (retain original suite enabled)
    suitesJSON := fmt.Sprintf("[{\"testSuite\":\"%s\",\"enabled\":true},{\"testSuite\":\"%s\",\"enabled\":true}]", testSuiteName, thirdSuiteName)
    output = s.runCommand(ts, updateWorkflowCmd(projectID, workflowName, nil, nil, nil, &suitesJSON, nil), ExpectNoError)
    ts.Contains(output.StdOut, UpdatedWorkflow)

    // Verify both suites are present and enabled
    output = s.runCommand(ts, getWorkflowCmd(projectID, workflowName), ExpectNoError)
    type wfGet struct {
        Suites []struct {
            Name    string `json:"name"`
            Enabled bool   `json:"enabled"`
        } `json:"suites"`
    }
    var wfOut wfGet
    err = json.Unmarshal([]byte(output.StdOut), &wfOut)
    ts.NoError(err)
    suitesState := map[string]bool{}
    for _, s := range wfOut.Suites {
        suitesState[s.Name] = s.Enabled
    }
    ts.Equal(true, suitesState[testSuiteName])
    ts.Equal(true, suitesState[thirdSuiteName])

    // 2) Toggle original suite to disabled, add secondSuite enabled, and remove thirdSuite
    suitesJSON = fmt.Sprintf("[{\"testSuite\":\"%s\",\"enabled\":false},{\"testSuite\":\"%s\",\"enabled\":true}]", testSuiteName, secondSuiteName)
    output = s.runCommand(ts, updateWorkflowCmd(projectID, workflowName, nil, nil, nil, &suitesJSON, nil), ExpectNoError)
    ts.Contains(output.StdOut, UpdatedWorkflow)

    // Verify: original suite disabled, secondSuite enabled, thirdSuite removed
    output = s.runCommand(ts, getWorkflowCmd(projectID, workflowName), ExpectNoError)
    wfOut = wfGet{}
    err = json.Unmarshal([]byte(output.StdOut), &wfOut)
    ts.NoError(err)
    suitesState = map[string]bool{}
    for _, s := range wfOut.Suites {
        suitesState[s.Name] = s.Enabled
    }
    // Present with expected enabled states
    ts.Contains(suitesState, testSuiteName)
    ts.Equal(false, suitesState[testSuiteName])
    ts.Contains(suitesState, secondSuiteName)
    ts.Equal(true, suitesState[secondSuiteName])
    // Removed
    _, exists := suitesState[thirdSuiteName]
    ts.False(exists)
}

func TestBatchWithZeroTimeout(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// Skip this test for now, as it's not working.
	t.Skip("Skipping batch creation with a single experience and 0s timeout")
	fmt.Println("Testing batch creation with a single experience and 0s timeout")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create an experience
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)

	// Now create the branch
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)

	// Attempt to create a batch with a 0s timeout
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{experienceIDString}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, map[string]string{}, AssociatedAccount, nil, Ptr(0)), ExpectNoError)
	// Expect the batch to be created successfully
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)

	// Now, wait for the batch to complete and validate that it has an error:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByID(projectID, batchIDString, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 errored
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	ts.Contains(output.StdOut, "ERROR")

	// Archive the project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}
func TestLogIngest(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	const ReIngestTrue = true
	const ReIngestFalse = false

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// Create the system
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]

	// A metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// There are no branches and there are no builds; we use the ingest command to create:
	firstBranchName := "test-branch"
	firstVersion := "first-version"
	logName := fmt.Sprintf("test-log-%s", uuid.New().String())

	logLocation := fmt.Sprintf("s3://%v/test-object/", s.Config.E2EBucket)

	experienceTags := []string{"test-tag"}
	ingestCommand := createIngestedLog(projectID, &systemIDString, &firstBranchName, &firstVersion, metricsBuildID, Ptr(logName), Ptr(logLocation), []string{}, nil, experienceTags, nil, nil, GithubTrue, ReIngestFalse)
	output = s.runCommand(ts, ingestCommand, ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDString)

	// Await the batch to complete:
	ts.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(ts, projectID, batchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByID(projectID, batchID.String(), ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	// Get the build and check version:
	output = s.runCommand(ts, getBuild(projectIDString, *batch.BuildID, false), ExpectNoError)
	var build api.Build
	err = json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(firstVersion, build.Version)
	// Get the branch and check the name:
	output = s.runCommand(ts, listBranches(projectID), ExpectNoError)
	var branches []api.Branch
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	ts.NoError(err)
	ts.Equal(1, len(branches))
	ts.Equal(firstBranchName, branches[0].Name)

	// Get the job ID:
	output = s.runCommand(ts, getBatchJobsByID(projectID, batchID.String()), ExpectNoError)
	var jobs []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	ts.NoError(err)
	ts.Equal(1, len(jobs))
	jobID := jobs[0].JobID

	// Check the logs and ensure the `file.name` file exists:
	output = s.runCommand(ts, listLogs(projectID, batchID.String(), jobID.String()), ExpectNoError)
	logs := []api.Log{}
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	ts.NoError(err)
	found := false
	for _, log := range logs {
		if log.FileName != nil && *log.FileName == "file.name" {
			found = true
			break
		}
	}
	ts.True(found)

	// Get the experience:
	output = s.runCommand(ts, getExperience(projectID, jobs[0].ExperienceID.String()), ExpectNoError)
	var experience api.Experience
	err = json.Unmarshal([]byte(output.StdOut), &experience)
	ts.NoError(err)
	ts.Equal(logName, experience.Name)
	ts.Equal(logLocation, experience.Location)
	// Finally, validate the tags:
	output = s.runCommand(ts, listExperiencesWithTag(projectID, "ingested-via-resim"), ExpectNoError)
	var experiencesWithTag []api.Experience
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	ts.NoError(err)
	ts.Equal(1, len(experiencesWithTag))
	ts.Equal(logName, experiencesWithTag[0].Name)
	// And the specificed tag
	output = s.runCommand(ts, listExperiencesWithTag(projectID, experienceTags[0]), ExpectNoError)
	experiencesWithTag = []api.Experience{}
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	ts.NoError(err)
	ts.Equal(1, len(experiencesWithTag))
	ts.Equal(logName, experiencesWithTag[0].Name)

	// Now, defaults:
	secondLogName := fmt.Sprintf("test-log-%v", uuid.New())
	secondLogTags := []string{"test-tag-2"}
	defaultBranchName := "log-ingest-branch"
	defaultVersion := "latest"
	secondLogCommand := createIngestedLog(projectID, &systemIDString, nil, nil, metricsBuildID, Ptr(secondLogName), Ptr(logLocation), []string{}, nil, secondLogTags, nil, nil, GithubTrue, ReIngestFalse)
	output = s.runCommand(ts, secondLogCommand, ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	secondBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	secondBatchID := uuid.MustParse(secondBatchIDString)

	// Await the batch to complete:
	ts.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(ts, projectID, secondBatchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)

	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByID(projectID, secondBatchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)

	// Get the build and check version:
	output = s.runCommand(ts, getBuild(projectIDString, *batch.BuildID, false), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(defaultVersion, build.Version)
	defaultBuildID := build.BuildID
	// Get the branch and check the name:
	output = s.runCommand(ts, listBranches(projectID), ExpectNoError)
	branches = []api.Branch{}
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	ts.NoError(err)
	ts.Equal(2, len(branches))
	ts.Equal(firstBranchName, branches[1].Name)
	ts.Equal(defaultBranchName, branches[0].Name)
	// Get the job ID:
	output = s.runCommand(ts, getBatchJobsByID(projectID, secondBatchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	ts.NoError(err)
	ts.Equal(1, len(jobs))
	jobID = jobs[0].JobID

	// Check the logs and ensure the `file.name` file exists:
	output = s.runCommand(ts, listLogs(projectID, secondBatchIDString, jobID.String()), ExpectNoError)
	logs = []api.Log{}
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	ts.NoError(err)
	found = false
	for _, log := range logs {
		if log.FileName != nil && *log.FileName == "file.name" {
			found = true
			break
		}
	}
	ts.True(found)

	// Get the experience:
	output = s.runCommand(ts, getExperience(projectID, jobs[0].ExperienceID.String()), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &experience)
	ts.NoError(err)
	ts.Equal(secondLogName, experience.Name)
	ts.Equal(logLocation, experience.Location)
	// Finally, validate the tags:
	output = s.runCommand(ts, listExperiencesWithTag(projectID, "ingested-via-resim"), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	ts.NoError(err)
	ts.Equal(2, len(experiencesWithTag))
	// And the specificed tag
	output = s.runCommand(ts, listExperiencesWithTag(projectID, secondLogTags[0]), ExpectNoError)
	experiencesWithTag = []api.Experience{}
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	ts.NoError(err)
	ts.Equal(1, len(experiencesWithTag))
	ts.Equal(secondLogName, experiencesWithTag[0].Name)
	// And that there is no overflow
	output = s.runCommand(ts, listExperiencesWithTag(projectID, experienceTags[0]), ExpectNoError)
	experiencesWithTag = []api.Experience{}
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	ts.NoError(err)
	ts.Equal(1, len(experiencesWithTag))
	ts.Equal(logName, experiencesWithTag[0].Name)

	// Validate that things are not recreated:
	thirdLogName := fmt.Sprintf("test-log-%v", uuid.New())
	specialBatchName := "my-batch-name"
	output = s.runCommand(ts, createIngestedLog(projectID, &systemIDString, nil, nil, metricsBuildID, Ptr(thirdLogName), Ptr(logLocation), []string{}, nil, secondLogTags, nil, &specialBatchName, GithubTrue, ReIngestFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	thirdBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByID(projectID, thirdBatchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(specialBatchName, *batch.FriendlyName)

	// Get the build and check version:
	output = s.runCommand(ts, getBuild(projectIDString, *batch.BuildID, false), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	ts.NoError(err)
	ts.Equal(defaultVersion, build.Version)
	ts.Equal(defaultBuildID, build.BuildID)
	// Get the branch and check the name and that no new branches were created:
	output = s.runCommand(ts, listBranches(projectID), ExpectNoError)
	branches = []api.Branch{}
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	ts.NoError(err)
	ts.Equal(2, len(branches))
	ts.Equal(firstBranchName, branches[1].Name)
	ts.Equal(defaultBranchName, branches[0].Name)

	//Create a build:
	output = s.runCommand(ts, createBuild(projectName, firstBranchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	existingBuildID := uuid.MustParse(buildIDString)
	// Finally, use the existing build ID:
	fourthLogName := fmt.Sprintf("test-log-%v", uuid.New())
	output = s.runCommand(ts, createIngestedLog(projectID, nil, nil, nil, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue, ReIngestFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	fourthBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(ts, getBatchByID(projectID, fourthBatchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)

	// Check the MuTex parameters:
	output = s.runCommand(ts, createIngestedLog(projectID, &systemIDString, nil, nil, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue, ReIngestFalse), ExpectError)
	ts.Contains(output.StdErr, "build-id")
	ts.Contains(output.StdErr, "system")
	output = s.runCommand(ts, createIngestedLog(projectID, nil, &firstBranchName, nil, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue, ReIngestFalse), ExpectError)
	ts.Contains(output.StdErr, "build-id")
	ts.Contains(output.StdErr, "branch")
	output = s.runCommand(ts, createIngestedLog(projectID, nil, nil, &firstVersion, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue, ReIngestFalse), ExpectError)
	ts.Contains(output.StdErr, "build-id")
	ts.Contains(output.StdErr, "version")

	// Test the `--log` flag:
	log1Name := fmt.Sprintf("test-log-%v", uuid.New())
	log1 := fmt.Sprintf("%s=%s", log1Name, logLocation)
	log2Name := fmt.Sprintf("test-log-%v", uuid.New())
	log2 := fmt.Sprintf("%s=%s", log2Name, logLocation)
	output = s.runCommand(ts, createIngestedLog(projectID, nil, nil, nil, metricsBuildID, nil, nil, []string{log1, log2}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue, ReIngestFalse), ExpectNoError)
	fmt.Println("Output: ", output.StdOut)
	fmt.Println("Output: ", output.StdErr)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID = uuid.MustParse(batchIDString)
	ts.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(ts, projectID, batchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Check there are two jobs:
	// Get the job ID:
	output = s.runCommand(ts, getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	ts.NoError(err)
	ts.Equal(2, len(jobs))

	// Finally, the config file:
	configFile := commands.LogsFile{
		Logs: []commands.LogConfig{
			{
				Name:     "log-1",
				Location: logLocation,
			},
			{
				Name:     "log-2",
				Location: logLocation,
			},
		},
	}
	// serialize this:
	configFileBytes, err := yaml.Marshal(configFile)
	ts.NoError(err)
	configFileString := string(configFileBytes)

	// Create the config file in the current directory:
	configFileLocation := filepath.Join(os.TempDir(), "valid_log_file.yaml")
	err = os.WriteFile(configFileLocation, []byte(configFileString), 0644)
	ts.NoError(err)

	// Run the ingest command with the config file:
	output = s.runCommand(ts, createIngestedLog(projectID, &systemIDString, &firstBranchName, &firstVersion, metricsBuildID, nil, nil, []string{}, Ptr(configFileLocation), nil, nil, nil, GithubTrue, ReIngestFalse), ExpectNoError)
	fmt.Println("Output: ", output.StdOut)
	fmt.Println("Output: ", output.StdErr)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID = uuid.MustParse(batchIDString)
	ts.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(ts, projectID, batchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	output = s.runCommand(ts, getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Check there are two jobs:
	// Get the job ID:
	output = s.runCommand(ts, getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	// collect the experience IDs
	configFileExperienceIDs := []uuid.UUID{}
	for _, job := range jobs {
		configFileExperienceIDs = append(configFileExperienceIDs, *job.ExperienceID)
	}
	ts.NoError(err)
	ts.Equal(2, len(jobs))

	// Re-ingest the config file with the --reingest flag; should not create duplicate experiences
	output = s.runCommand(ts, createIngestedLog(projectID, &systemIDString, &firstBranchName, &firstVersion, metricsBuildID, nil, nil, []string{}, Ptr(configFileLocation), nil, nil, nil, GithubTrue, ReIngestTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)

	reIngestBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	// Do not wait for batch to complete; just cancel it
	output = s.runCommand(ts, cancelBatchByID(projectID, reIngestBatchIDString), ExpectNoError)
	ts.Contains(output.StdOut, CancelledBatch)

	// Check that batch used the same experiences as the config file batch
	output = s.runCommand(ts, getBatchJobsByID(projectID, reIngestBatchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	// collect the experience IDs
	for _, job := range jobs {
		ts.Contains(configFileExperienceIDs, *job.ExperienceID)
	}
	ts.NoError(err)
	ts.Equal(2, len(jobs))
}
func TestMetricsSync(t *testing.T) {
	ts := assert.New(t)
	req := require.New(t)
	t.Parallel()

	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	username := os.Getenv(username)
	password := os.Getenv(password)
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)

	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID, err := uuid.Parse(projectIDString)
	req.NoError(err)

	// create the main branch
	output = s.runCommand(ts, createBranch(projectID, "main", "RELEASE", GithubTrue), ExpectNoError)

	t.Run("NoConfigFiles", func(t *testing.T) {
		output := s.runCommand(ts, syncMetrics(projectIDString, true, username, password), true)

		ts.Contains(output.StdErr, "failed to find ReSim metrics config")
	})

	t.Run("SyncsMetricsConfig", func(t *testing.T) {
		ts := assert.New(t)
		os.RemoveAll(".resim") // Cleanup old folder just in case

		err := os.Mkdir(".resim", 0755)
		ts.NoError(err)
		defer os.RemoveAll(".resim")
		err = os.Mkdir(".resim/metrics", 0755)
		ts.NoError(err)
		err = os.Mkdir(".resim/metrics/templates", 0755)
		ts.NoError(err)

		// strings.Join is ugly but using backticks `` is a mess with getting indentation correct
		metricsFile := strings.Join([]string{
			"version: 1",
			"topics:",
			"  ok:",
			"    schema:",
			"      speed: float",
			"metrics:",
			"  Average Speed:",
			"    type: test",
			"    query_string: SELECT AVG(speed) FROM speed WHERE job_id=$job_id",
			"    template_type: system",
			"    template: line",
			"metrics sets:",
			"  woot:",
			"    metrics:",
			"      - Average Speed",
		}, "\n")
		err = os.WriteFile(".resim/metrics/config.yml", []byte(metricsFile), 0644)
		ts.NoError(err)
		err = os.WriteFile(".resim/metrics/templates/bar.liquid", []byte("{}"), 0644)
		ts.NoError(err)

		// Standard behavior is exit 0 with no output
		output := s.runCommand(ts, syncMetrics(projectIDString, false, username, password), false)
		ts.Equal("", output.StdOut)
		ts.Equal("", output.StdErr)

		// Verbose logs a lot of info about what it is doing
		output = s.runCommand(ts, syncMetrics(projectIDString, true, username, password), false)
		ts.Equal("", output.StdErr)
		ts.Contains(output.StdOut, "Looking for metrics config at .resim/metrics/config.yml")
		ts.Contains(output.StdOut, "Found template bar.liquid")
		ts.Contains(output.StdOut, "Successfully synced metrics config, and the following templates:")
	})
}

func checkBatchComplete(ts *assert.Assertions, projectID uuid.UUID, batchID uuid.UUID) (bool, int) {
	cmd := s.buildCommand(getBatchByID(projectID, batchID.String(), ExitStatusTrue))
	var stdout, stderr bytes.Buffer
	fmt.Println("About to run command: ", cmd.String())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}
	ts.Contains(AcceptableBatchStatusCodes, exitCode)
	ts.Empty(stderr.String())
	ts.Empty(stdout.String())
	// Check if the status is 0, complete, 5 cancelled, 2 failed
	complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
	return complete, exitCode
}

func TestCancelBatch(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// First create an experience:
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString1)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(ts, createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create a batch with (only) experience names using the --experiences flag with no parameters
	output = s.runCommand(ts, createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName}, []string{}, metricsBuildIDString, GithubTrue, map[string]string{}, AssociatedAccount, nil, nil), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDStringGH)

	time.Sleep(30 * time.Second) // arbitrary sleep to make sure the scheduler gets the batch and triggers it

	// Cancel the batch
	output = s.runCommand(ts, cancelBatchByID(projectID, batchIDStringGH), ExpectNoError)
	ts.Contains(output.StdOut, CancelledBatch)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	ts.Eventually(func() bool {
		cmd := s.buildCommand(getBatchByID(projectID, batchIDStringGH, ExitStatusTrue))
		var stdout, stderr bytes.Buffer
		fmt.Println("About to run command: ", cmd.String())
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}
		ts.Contains(AcceptableBatchStatusCodes, exitCode)
		ts.Empty(stderr.String())
		ts.Empty(stdout.String())
		// Check if the status is 0, complete, 5 cancelled, 2 failed
		complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status by ID:
	output = s.runCommand(ts, getBatchByID(projectID, batchIDStringGH, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	ts.NoError(err)
	ts.Equal(batchID, *batch.BatchID)
	// Validate that it was cancelled
	ts.Equal(api.BatchStatusCANCELLED, *batch.Status)
	ts.Equal(buildID, *batch.BuildID)
	ts.Equal(metricsBuildID, *batch.MetricsBuildID)

	// Archive the project
	output = s.runCommand(ts, archiveProject(projectIDString), ExpectNoError)
	ts.Contains(output.StdOut, ArchivedProject)
	ts.Empty(output.StdErr)
}

func TestDebug(t *testing.T) {
	ts := assert.New(t)
	t.Parallel()
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(ts, createProject(projectName, "description", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// create a system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(ts, createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(ts, createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// create a build:
	output = s.runCommand(ts, createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/ubuntu/ubuntu:24.04_stable", []string{}, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedBuild, output.StdErr)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	// buildID := uuid.MustParse(buildIDString)

	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())

	// create an experience:
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(ts, createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, EmptySlice, nil, nil, nil, GithubTrue), ExpectNoError)
	ts.Contains(output.StdOut, GithubCreatedExperience)
	ts.Empty(output.StdErr)

	expectedOutput := "Waiting for debug environment to be ready...\n"

	// run a debug batch:
	output = s.runCommand(ts, debugCommand(projectID, buildIDString, experienceName), ExpectNoError)
	ts.Contains(output.StdOut, expectedOutput)
	ts.Empty(output.StdErr)
	ts.NotContains(output.StdOut, "error")

	fmt.Println("Output: ", output.StdOut)
}

func TestMain(m *testing.M) {
	viper.AutomaticEnv()
	viper.SetDefault(Config, Dev)
	// Get a default value for the associated account:
	maybeCIAccount := commands.GetCIEnvironmentVariableAccount()
	if maybeCIAccount != "" {
		AssociatedAccount = maybeCIAccount
	}
	fmt.Printf("Running the end to end test with %s account", AssociatedAccount)
	s.SetupHelper()
	os.Exit(m.Run())
}
