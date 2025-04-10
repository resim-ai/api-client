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

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
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

type EndToEndTestSuite struct {
	suite.Suite
	CliPath string
	Config  CliConfig
}

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
	CreatedBuild          string = "Created build"
	GithubCreatedBuild    string = "build_id="
	EmptyBuildName        string = "empty build name"
	EmptyBuildDescription string = "empty build description"
	EmptyBuildImage       string = "empty build image URI"
	InvalidBuildImage     string = "failed to parse the image URI"
	EmptyBuildVersion     string = "empty build version"
	EmptySystem           string = "system not supplied"
	SystemDoesNotExist    string = "failed to find system"
	BranchNotExist        string = "Branch does not exist"
	UpdatedBuild          string = "Updated build"
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
	EmptyExperienceLocation    string = "empty experience location"
	DeprecatedLaunchProfile    string = "launch profiles are deprecated"
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
	// Log Ingest Messages
	LogIngested string = "Ingested log successfully!"
)

var AcceptableBatchStatusCodes = [...]int{0, 2, 3, 4, 5}
var AcceptableSweepStatusCodes = [...]int{0, 2, 3, 4, 5}

func (s *EndToEndTestSuite) TearDownSuite() {
	os.Remove(fmt.Sprintf("%s/%s", s.CliPath, CliName))
	os.Remove(s.CliPath)
}

func (s *EndToEndTestSuite) SetupSuite() {
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

func (s *EndToEndTestSuite) buildCLI() string {
	fmt.Println("Building CLI")
	tmpDir, err := os.MkdirTemp("", TempDirSuffix)
	s.NoError(err)
	outputPath := filepath.Join(tmpDir, CliName)
	buildCmd := exec.Command("go", "build", "-o", outputPath, "../cmd/resim")
	err = buildCmd.Run()
	s.NoError(err)
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

func (s *EndToEndTestSuite) buildCommand(commandBuilders []CommandBuilder) *exec.Cmd {
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

func (s *EndToEndTestSuite) runCommand(commandBuilders []CommandBuilder, expectError bool) Output {
	var stdout, stderr bytes.Buffer
	cmd := s.buildCommand(commandBuilders)
	fmt.Println("About to run command: ", cmd.String())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if expectError {
		s.Error(err)
	} else {
		s.NoError(err)
	}
	return Output{
		StdOut: stdout.String(),
		StdErr: stderr.String(),
	}
}

func syncMetrics(verbose bool) []CommandBuilder {
	metricsCommand := CommandBuilder{Command: "metrics"}

	flags := []Flag{}
	if verbose {
		flags = append(flags, Flag{Name: "--verbose"})
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

func createSystem(projectName string, systemName string, systemDescription string, buildVCPUs *int, buildGPUs *int, buildMemoryMiB *int, buildSharedMemoryMB *int, metricsBuildVCPUs *int, metricsBuildGPUs *int, metricsBuildMemoryMiB *int, metricsBuildSharedMemoryMB *int, github bool) []CommandBuilder {
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

func createBuild(projectName string, branchName string, systemName string, description string, image string, version string, github bool, autoCreateBranch bool) []CommandBuilder {
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
				Name:  "--image",
				Value: image,
			},
			{
				Name:  "--version",
				Value: version,
			},
		},
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

func getBuild(projectName string, existingBuildID uuid.UUID) []CommandBuilder {
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

func createExperience(projectID uuid.UUID, name string, description string, location string, systems []string, timeout *time.Duration, github bool) []CommandBuilder {
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
				Name:  "--location",
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
	if timeout != nil {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--timeout",
			Value: timeout.String(),
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

func updateExperience(projectID uuid.UUID, experienceKey string, name *string, description *string, location *string, timeout *time.Duration) []CommandBuilder {
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
	if timeout != nil {
		updateCommand.Flags = append(updateCommand.Flags, Flag{
			Name:  "--timeout",
			Value: timeout.String(),
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

func createIngestedLog(projectID uuid.UUID, system *string, branchname *string, version *string, metricsBuildID uuid.UUID, logName *string, logLocation *string, logsList []string, configFileLocation *string, experienceTags []string, buildID *uuid.UUID, batchName *string, github bool) []CommandBuilder {
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

func createTestSuite(projectID uuid.UUID, name string, description string, systemID string, experiences []string, metricsBuildID string, github bool) []CommandBuilder {
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
	return []CommandBuilder{suitesCommand, createCommand}
}

func reviseTestSuite(projectID uuid.UUID, testSuite string, name *string, description *string, systemID *string, experiences *[]string, metricsBuildID *string, showOnSummary *bool, github bool) []CommandBuilder {
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

// As a first test, we expect the help command to run successfully
func (s *EndToEndTestSuite) TestHelp() {
	fmt.Println("Testing help command")
	runCommand := CommandBuilder{
		Command: "help",
	}
	output := s.runCommand([]CommandBuilder{runCommand}, ExpectNoError)
	s.Contains(output.StdOut, "USAGE")
}

func (s *EndToEndTestSuite) TestProjectCommands() {
	fmt.Println("Testing project create command")
	// Check we can successfully create a project with a unique name
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedProject)
	s.Empty(output.StdErr)
	// Validate that repeating that name leads to an error:
	output = s.runCommand(createProject(projectName, "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, ProjectNameCollision)

	// Validate that omitting the name leads to an error:
	output = s.runCommand(createProject("", "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyProjectName)
	// Validate that omitting the description leads to an error:
	output = s.runCommand(createProject(projectName, "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyProjectDescription)

	// Check we can list the projects, and our new project is in it:
	output = s.runCommand(listProjects(), ExpectNoError)
	s.Contains(output.StdOut, projectName)

	// Now get, verify, and archive the project:
	fmt.Println("Testing project get command")
	output = s.runCommand(getProject(projectName), ExpectNoError)
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, project.Name)
	s.Empty(output.StdErr)

	// Attempt to get project by id:
	output = s.runCommand(getProject((project.ProjectID).String()), ExpectNoError)
	var project2 api.Project
	err = json.Unmarshal([]byte(output.StdOut), &project2)
	s.NoError(err)
	s.Equal(projectName, project.Name)
	s.Empty(output.StdErr)
	// Attempt to get a project with empty name and id:
	output = s.runCommand(getProject(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Non-existent project:
	output = s.runCommand(getProject(uuid.New().String()), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Blank name:
	output = s.runCommand(getProject(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)

	// Validate that using the id as another project name throws an error.
	output = s.runCommand(createProject(project.ProjectID.String(), "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, ProjectNameCollision)

	fmt.Println("Testing project archive command")
	output = s.runCommand(archiveProject(projectName), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
	// Verify that attempting to re-archive will fail:
	output = s.runCommand(archiveProject(projectName), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Verify that a valid project ID is needed:
	output = s.runCommand(archiveProject(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
}

func (s *EndToEndTestSuite) TestProjectCreateGithub() {
	fmt.Println("Testing project create command, with --github flag")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// Now get, verify, and archive the project:
	output = s.runCommand(getProject(projectIDString), ExpectNoError)
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, project.Name)
	s.Equal(projectID, project.ProjectID)
	s.Empty(output.StdErr)
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

// Test branch creation:
func (s *EndToEndTestSuite) TestBranchCreate() {
	fmt.Println("Testing branch creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedBranch)
	// Validate that  missing name, project, or type returns errors:
	output = s.runCommand(createBranch(projectID, "", "RELEASE", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBranchName)
	output = s.runCommand(createBranch(uuid.Nil, branchName, "RELEASE", GithubFalse), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(createBranch(projectID, branchName, "INVALID", GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBranchType)

	// Check we can list the branches, and our new branch is in it:
	output = s.runCommand(listBranches(projectID), ExpectNoError)
	s.Contains(output.StdOut, branchName)

	// Archive the test project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestBranchCreateGithub() {
	fmt.Println("Testing branch creation, with --github flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Archive the test project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestSystems() {
	fmt.Println("Testing system creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
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
	output = s.runCommand(createSystem(projectIDString, systemName, systemDescription, Ptr(buildVCPUs), Ptr(buildGPUs), Ptr(buildMemoryMiB), Ptr(buildSharedMemoryMB), Ptr(metricsBuildVCPUs), Ptr(metricsBuildGPUs), Ptr(metricsBuildMemoryMiB), Ptr(metricsBuildSharedMemoryMB), GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedSystem)
	// Get the system:
	output = s.runCommand(getSystem(projectIDString, systemName), ExpectNoError)
	var system api.System
	err := json.Unmarshal([]byte(output.StdOut), &system)
	s.NoError(err)
	s.Equal(systemName, system.Name)
	s.Equal(systemDescription, system.Description)
	s.Equal(buildVCPUs, system.BuildVcpus)
	s.Equal(buildGPUs, system.BuildGpus)
	s.Equal(buildMemoryMiB, system.BuildMemoryMib)
	s.Equal(buildSharedMemoryMB, system.BuildSharedMemoryMb)
	s.Equal(metricsBuildVCPUs, system.MetricsBuildVcpus)
	s.Equal(metricsBuildGPUs, system.MetricsBuildGpus)
	s.Equal(metricsBuildMemoryMiB, system.MetricsBuildMemoryMib)
	s.Equal(metricsBuildSharedMemoryMB, system.MetricsBuildSharedMemoryMb)
	s.Empty(output.StdErr)
	systemID := system.SystemID

	// Validate that the defaults work:
	system2Name := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, system2Name, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedSystem)
	// Get the system:
	output = s.runCommand(getSystem(projectIDString, system2Name), ExpectNoError)
	var system2 api.System
	err = json.Unmarshal([]byte(output.StdOut), &system2)
	s.NoError(err)
	s.Equal(system2Name, system2.Name)
	s.Equal(systemDescription, system2.Description)
	s.Equal(commands.DefaultCPUs, system2.BuildVcpus)
	s.Equal(commands.DefaultGPUs, system2.BuildGpus)
	s.Equal(commands.DefaultMemoryMiB, system2.BuildMemoryMib)
	s.Equal(commands.DefaultSharedMemoryMB, system2.BuildSharedMemoryMb)
	s.Equal(commands.DefaultCPUs, system2.MetricsBuildVcpus)
	s.Equal(commands.DefaultGPUs, system2.MetricsBuildGpus)
	s.Equal(commands.DefaultMemoryMiB, system2.MetricsBuildMemoryMib)
	s.Equal(commands.DefaultSharedMemoryMB, system2.MetricsBuildSharedMemoryMb)
	s.Empty(output.StdErr)

	// Validate that missing name, project, or description returns errors:
	output = s.runCommand(createSystem(projectIDString, "", systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(createSystem(projectIDString, systemName, "", nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptySystemDescription)
	output = s.runCommand(createSystem(uuid.Nil.String(), systemName, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, GithubFalse), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)

	// Check we can list the systems, and our new system is in it:
	output = s.runCommand(listSystems(projectID), ExpectNoError)
	s.Contains(output.StdOut, systemName)

	// Now add a couple of builds to the system (and a branch by the autocreate):
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	build1Version := "0.0.1"
	output = s.runCommand(createBuild(projectIDString, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", build1Version, GithubTrue, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	build1IDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(build1IDString)
	build2Version := "0.0.2"
	output = s.runCommand(createBuild(projectIDString, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", build2Version, GithubTrue, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	build2IDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(build2IDString)
	// Check we can list the builds, and our new builds are in it:
	output = s.runCommand(systemBuilds(projectIDString, systemID.String()), ExpectNoError)
	s.Contains(output.StdOut, build1Version)
	s.Contains(output.StdOut, build2Version)

	// Create and tag a couple of experiences:
	experience1Name := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(createExperience(projectID, experience1Name, "description", "location", EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experience1IDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experience1IDString)
	experience2Name := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(createExperience(projectID, experience2Name, "description", "location", EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experience2IDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experience2IDString)
	// Check we can list the experiences for the systems, and our new experiences are not in it:
	output = s.runCommand(systemExperiences(projectIDString, systemName), ExpectNoError)
	s.NotContains(output.StdOut, experience1Name)
	s.NotContains(output.StdOut, experience2Name)
	s.NotContains(output.StdOut, experience1IDString)
	s.NotContains(output.StdOut, experience2IDString)

	// Add the experiences to the system:
	output = s.runCommand(addSystemToExperience(projectIDString, systemName, experience1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(addSystemToExperience(projectIDString, systemName, experience1IDString), ExpectError)
	s.Contains(output.StdErr, SystemAlreadyRegistered)
	output = s.runCommand(addSystemToExperience(projectName, systemName, experience2IDString), ExpectNoError)
	// Check we can list the experiences for the systems, and our new experiences are in it:
	output = s.runCommand(systemExperiences(projectIDString, systemName), ExpectNoError)
	s.Contains(output.StdOut, experience1Name)
	s.Contains(output.StdOut, experience2Name)
	s.Contains(output.StdOut, experience1IDString)
	s.Contains(output.StdOut, experience2IDString)

	// Remove one experience:
	output = s.runCommand(removeSystemFromExperience(projectIDString, systemName, experience1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(removeSystemFromExperience(projectIDString, systemName, experience1IDString), ExpectError)
	// Check we can list the experiences for the systems, and only one experience is in it:
	output = s.runCommand(systemExperiences(projectIDString, systemName), ExpectNoError)
	s.NotContains(output.StdOut, experience1Name)
	s.Contains(output.StdOut, experience2Name)
	// Remove the second experience:
	output = s.runCommand(removeSystemFromExperience(projectIDString, systemName, experience2IDString), ExpectNoError)
	// Check we can list the experiences for the systems, and no experiences are in it:
	output = s.runCommand(systemExperiences(projectIDString, systemName), ExpectNoError)
	s.NotContains(output.StdOut, experience1Name)
	s.NotContains(output.StdOut, experience2Name)

	// Edge cases:
	output = s.runCommand(addSystemToExperience("", systemName, experience1IDString), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(addSystemToExperience(projectIDString, "", experience1IDString), ExpectError)
	s.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(addSystemToExperience(projectIDString, systemName, ""), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceName)
	output = s.runCommand(removeSystemFromExperience("", systemName, experience1IDString), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(removeSystemFromExperience(projectIDString, "", experience1IDString), ExpectError)
	s.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(removeSystemFromExperience(projectIDString, systemName, ""), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceName)

	// Create and tag a couple of metrics builds:
	metricsBuild1Name := fmt.Sprintf("test-metrics-build-%s", uuid.New().String())
	output = s.runCommand(createMetricsBuild(projectID, metricsBuild1Name, "public.ecr.aws/docker/library/hello-world:latest", "0.0.1", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the metrics build ID as a UUID
	metricsBuild1IDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuild1IDString)
	metricsBuild2Name := fmt.Sprintf("test-metrics-build-%s", uuid.New().String())
	output = s.runCommand(createMetricsBuild(projectID, metricsBuild2Name, "public.ecr.aws/docker/library/hello-world:latest", "0.0.2", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the metrics build ID as a UUID
	metricsBuild2IDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuild2IDString)
	// Check we can list the metrics builds for the systems, and our new metrics builds are not in it:
	output = s.runCommand(systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	s.NotContains(output.StdOut, metricsBuild1Name)
	s.NotContains(output.StdOut, metricsBuild2Name)
	s.NotContains(output.StdOut, metricsBuild1IDString)
	s.NotContains(output.StdOut, metricsBuild2IDString)

	// Add the metrics builds to the system:
	output = s.runCommand(addSystemToMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(addSystemToMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectError)
	s.Contains(output.StdErr, SystemAlreadyRegistered)
	output = s.runCommand(addSystemToMetricsBuild(projectName, systemName, metricsBuild2IDString), ExpectNoError)
	// Check we can list the metrics builds for the systems, and our new metrics builds are in it:
	output = s.runCommand(systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	s.Contains(output.StdOut, metricsBuild1Name)
	s.Contains(output.StdOut, metricsBuild2Name)
	s.Contains(output.StdOut, metricsBuild1IDString)
	s.Contains(output.StdOut, metricsBuild2IDString)

	// Remove one metrics build:
	output = s.runCommand(removeSystemFromMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectNoError)
	// Check duplicates should error:
	output = s.runCommand(removeSystemFromMetricsBuild(projectIDString, systemName, metricsBuild1IDString), ExpectError)
	// Check we can list the metrics builds for the systems, and only one metrics build is in it:
	output = s.runCommand(systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	s.NotContains(output.StdOut, metricsBuild1Name)
	s.Contains(output.StdOut, metricsBuild2Name)
	// Remove the second metrics build:
	output = s.runCommand(removeSystemFromMetricsBuild(projectIDString, systemName, metricsBuild2IDString), ExpectNoError)
	// Check we can list the metrics builds for the systems, and no metrics builds are in it:
	output = s.runCommand(systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
	s.NotContains(output.StdOut, metricsBuild1Name)
	s.NotContains(output.StdOut, metricsBuild2Name)

	// Edge cases:
	output = s.runCommand(addSystemToMetricsBuild("", systemName, metricsBuild1IDString), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(addSystemToMetricsBuild(projectIDString, "", metricsBuild1IDString), ExpectError)
	s.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(addSystemToMetricsBuild(projectIDString, systemName, ""), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildName)
	output = s.runCommand(removeSystemFromMetricsBuild("", systemName, metricsBuild1IDString), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(removeSystemFromMetricsBuild(projectIDString, "", metricsBuild1IDString), ExpectError)
	s.Contains(output.StdErr, EmptySystemName)
	output = s.runCommand(removeSystemFromMetricsBuild(projectIDString, systemName, ""), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildName)

	// Update the system:
	const updatedSystemDescription = "updated system description"
	output = s.runCommand(
		updateSystem(projectIDString, systemName, nil, Ptr(updatedSystemDescription), nil, nil, nil, nil, nil, nil, nil, nil),
		ExpectNoError)
	fmt.Println(output.StdErr)
	fmt.Println(output.StdOut)
	s.Contains(output.StdOut, UpdatedSystem)
	// Get the system:
	output = s.runCommand(getSystem(projectIDString, systemName), ExpectNoError)
	var updatedSystem api.System
	err = json.Unmarshal([]byte(output.StdOut), &updatedSystem)
	s.NoError(err)
	s.Equal(systemName, updatedSystem.Name)
	s.Equal(updatedSystemDescription, updatedSystem.Description)
	s.Equal(buildVCPUs, updatedSystem.BuildVcpus)
	s.Equal(buildGPUs, updatedSystem.BuildGpus)
	s.Equal(buildMemoryMiB, updatedSystem.BuildMemoryMib)
	s.Equal(buildSharedMemoryMB, updatedSystem.BuildSharedMemoryMb)
	s.Equal(metricsBuildVCPUs, updatedSystem.MetricsBuildVcpus)
	s.Equal(metricsBuildGPUs, updatedSystem.MetricsBuildGpus)
	s.Equal(metricsBuildMemoryMiB, updatedSystem.MetricsBuildMemoryMib)
	s.Equal(metricsBuildSharedMemoryMB, updatedSystem.MetricsBuildSharedMemoryMb)
	s.Empty(output.StdErr)

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

	output = s.runCommand(
		updateSystem(projectIDString, systemName, Ptr(newName), nil, Ptr(newBuildCPUs), Ptr(newBuildGPUs), Ptr(newBuildMemory), Ptr(newBuildSharedMemory), Ptr(newMetricsBuildCPUs), Ptr(newMetricsBuildGPUs), Ptr(newMetricsBuildMemory), Ptr(newMetricsBuildSharedMemory)),
		ExpectNoError)
	fmt.Println(output.StdErr)
	fmt.Println(output.StdOut)
	s.Contains(output.StdOut, UpdatedSystem)
	// Get the system:
	output = s.runCommand(getSystem(projectIDString, newName), ExpectNoError)
	var newUpdatedSystem api.System
	err = json.Unmarshal([]byte(output.StdOut), &newUpdatedSystem)
	s.NoError(err)
	s.Equal(newName, newUpdatedSystem.Name)
	s.Equal(updatedSystemDescription, newUpdatedSystem.Description)
	s.Equal(newBuildCPUs, newUpdatedSystem.BuildVcpus)
	s.Equal(newBuildGPUs, newUpdatedSystem.BuildGpus)
	s.Equal(newBuildMemory, newUpdatedSystem.BuildMemoryMib)
	s.Equal(newBuildSharedMemory, newUpdatedSystem.BuildSharedMemoryMb)
	s.Equal(newMetricsBuildCPUs, newUpdatedSystem.MetricsBuildVcpus)
	s.Equal(newMetricsBuildGPUs, newUpdatedSystem.MetricsBuildGpus)
	s.Equal(newMetricsBuildMemory, newUpdatedSystem.MetricsBuildMemoryMib)
	s.Equal(newMetricsBuildSharedMemory, newUpdatedSystem.MetricsBuildSharedMemoryMb)
	s.Empty(output.StdErr)

	// Archive the system:
	output = s.runCommand(archiveSystem(projectIDString, newName), ExpectNoError)
	s.Contains(output.StdOut, ArchivedSystem)
	s.Empty(output.StdErr)

	// Archive the test project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestSystemCreateGithub() {
	fmt.Println("Testing system creation, with github flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	const systemDescription = "test system description"
	output = s.runCommand(createSystem(projectIDString, systemName, systemDescription, nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// Get the system:
	output = s.runCommand(getSystem(projectIDString, systemName), ExpectNoError)
	var system2 api.System
	err := json.Unmarshal([]byte(output.StdOut), &system2)
	s.NoError(err)
	s.Equal(systemName, system2.Name)
	s.Equal(systemDescription, system2.Description)
	s.Equal(commands.DefaultCPUs, system2.BuildVcpus)
	s.Equal(commands.DefaultGPUs, system2.BuildGpus)
	s.Equal(commands.DefaultMemoryMiB, system2.BuildMemoryMib)
	s.Equal(commands.DefaultSharedMemoryMB, system2.BuildSharedMemoryMb)
	s.Equal(commands.DefaultCPUs, system2.MetricsBuildVcpus)
	s.Equal(commands.DefaultGPUs, system2.MetricsBuildGpus)
	s.Equal(commands.DefaultMemoryMiB, system2.MetricsBuildMemoryMib)
	s.Equal(commands.DefaultSharedMemoryMB, system2.MetricsBuildSharedMemoryMb)
	s.Empty(output.StdErr)

	// Check we can list the systems, and our new system is in it:
	output = s.runCommand(listSystems(projectID), ExpectNoError)
	s.Contains(output.StdOut, systemName)

	// Archive the test project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

// Test the build creation:
func (s *EndToEndTestSuite) TestBuildCreateUpdate() {
	fmt.Println("Testing build creation")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the project ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	systemID := uuid.MustParse(systemIDString)
	// Now create the build:
	originalBuildDescription := "description"
	output = s.runCommand(createBuild(projectName, branchName, systemName, originalBuildDescription, "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)

	// Check we can list the builds by passing in the branch, and our new build is in it:
	output = s.runCommand(listBuilds(projectID, Ptr(branchName), nil), ExpectNoError) // with no system filter
	s.Contains(output.StdOut, systemIDString)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the system, and our new build is in it:
	output = s.runCommand(listBuilds(projectID, nil, Ptr(systemName)), ExpectNoError) // with no branch filter
	s.Contains(output.StdOut, systemIDString)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the branchID, and our new build is in it:
	output = s.runCommand(listBuilds(projectID, Ptr(branchID.String()), nil), ExpectNoError)
	s.Contains(output.StdOut, systemIDString)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the systemID, and our new build is in it:
	output = s.runCommand(listBuilds(projectID, nil, Ptr(systemID.String())), ExpectNoError)
	s.Contains(output.StdOut, systemIDString)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Check we can list the builds with no filters, and our new build is in it:
	output = s.runCommand(listBuilds(projectID, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, systemIDString)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Verify that each of the required flags are required:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildName)
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildImage)
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildVersion)
	output = s.runCommand(createBuild("", branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(createBuild(projectName, "", systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, BranchNotExist)
	output = s.runCommand(createBuild(projectName, branchName, "", "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, SystemDoesNotExist)
	// Validate the image URI is required to be valid and have a tag:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBuildImage)
	// Update the branch id:
	secondBranchName := fmt.Sprintf("updated-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, secondBranchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	updatedBranchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	updatedBranchID := uuid.MustParse(updatedBranchIDString)
	output = s.runCommand(updateBuild(projectIDString, buildID, Ptr(updatedBranchID), nil), ExpectNoError)
	s.Contains(output.StdOut, UpdatedBuild)
	// Get the build and check:
	output = s.runCommand(getBuild(projectIDString, buildID), ExpectNoError)
	var build api.Build
	err := json.Unmarshal([]byte(output.StdOut), &build)
	s.NoError(err)
	s.Equal(updatedBranchID, build.BranchID)
	s.Equal(originalBuildDescription, build.Description)
	s.Empty(output.StdErr)

	updatedBuildDescription := "updated description"
	output = s.runCommand(updateBuild(projectIDString, buildID, nil, Ptr(updatedBuildDescription)), ExpectNoError)
	s.Contains(output.StdOut, UpdatedBuild)
	// Get the build and check:
	output = s.runCommand(getBuild(projectIDString, buildID), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	s.NoError(err)
	s.Equal(updatedBranchID, build.BranchID)
	s.Equal(updatedBuildDescription, build.Description)
	s.Empty(output.StdErr)

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible
}

func (s *EndToEndTestSuite) TestBuildCreateGithub() {
	fmt.Println("Testing build creation, with --github flag")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the project ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestBuildCreateAutoCreateBranch() {
	fmt.Println("Testing build creation with the auto-create-branch flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build: (with auto-create-branch flag). We expect this to succeed without any additional information
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, CreatedBuild)
	s.NotContains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", branchName))

	// Now try to create a build with a new branch name:
	newBranchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBuild(projectName, newBranchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.1", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, CreatedBuild)
	s.Contains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", newBranchName))
	s.Contains(output.StdOut, CreatedBranch)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestExperienceCreate() {
	fmt.Println("Testing experience creation command")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create two systems to add as part of the experience creation:
	systemName1 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName1, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString1 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString1)
	systemName2 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName2, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString2 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString2)

	systemNames := []string{systemName1, systemName2}
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	timeoutSeconds := int32(200)
	timeout := time.Duration(timeoutSeconds) * time.Second
	output = s.runCommand(createExperience(projectID, experienceName, "description", "location", systemNames, &timeout, GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedExperience)
	s.Empty(output.StdErr)

	// Now get the experience by name:
	output = s.runCommand(getExperience(projectID, experienceName), ExpectNoError)
	s.Contains(output.StdOut, experienceName)
	s.Empty(output.StdErr)
	var experience api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &experience)
	s.NoError(err)
	s.Equal(experienceName, experience.Name)
	s.Equal("description", experience.Description)
	s.Equal("location", experience.Location)
	s.Equal(timeoutSeconds, experience.ContainerTimeoutSeconds)
	// Validate that the experience is available for each system:
	for _, systemName := range systemNames {
		output = s.runCommand(systemExperiences(projectIDString, systemName), ExpectNoError)
		s.Contains(output.StdOut, experienceName)
	}

	// Validate we cannot create experiences without values for the required flags:
	output = s.runCommand(createExperience(projectID, "", "description", "location", EmptySlice, nil, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceName)
	output = s.runCommand(createExperience(projectID, experienceName, "", "location", EmptySlice, nil, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceDescription)
	output = s.runCommand(createExperience(projectID, experienceName, "description", "", EmptySlice, nil, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceLocation)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Archive the experiences when possible

	// Test creating an experience with the launch profile flag:
	launchProfileID := uuid.New().String()
	experienceCommand := createExperience(projectID, experienceName, "description", "location", EmptySlice, nil, GithubFalse)
	experienceCommand[1].Flags = append(experienceCommand[1].Flags, Flag{
		Name:  "--launch-profile",
		Value: launchProfileID,
	})
	output = s.runCommand(experienceCommand, ExpectError)
	s.Contains(output.StdErr, DeprecatedLaunchProfile)

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestExperienceCreateGithub() {
	fmt.Println("Testing experience creation command, with --github flag")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(createExperience(projectID, experienceName, "description", "location", EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
	// Now get the experience by id:
	output = s.runCommand(getExperience(projectID, experienceIDString), ExpectNoError)
	s.Contains(output.StdOut, experienceName)
	s.Empty(output.StdErr)
	var experience api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &experience)
	s.NoError(err)
	s.Equal(experienceName, experience.Name)
	s.Equal("description", experience.Description)
	s.Equal("location", experience.Location)
	s.Equal(int32(3600), experience.ContainerTimeoutSeconds) // default timeout

	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Archive the experiences when possible

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) verifyExperienceUpdate(projectID uuid.UUID, experienceID, expectedName, expectedDescription, expectedLocation string, expectedTimeout int32) {
	output := s.runCommand(getExperience(projectID, experienceID), ExpectNoError)
	s.Contains(output.StdOut, expectedName)
	s.Contains(output.StdOut, expectedDescription)
	s.Contains(output.StdOut, expectedLocation)
	s.Contains(output.StdOut, fmt.Sprintf("%d", expectedTimeout))
}

func (s *EndToEndTestSuite) TestExperienceUpdate() {
	fmt.Println("Testing experience update command")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create an experience:
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	originalDescription := "original description"
	originalLocation := "original location"
	originalTimeoutSeconds := int32(200)
	originalTimeout := time.Duration(originalTimeoutSeconds) * time.Second
	output = s.runCommand(createExperience(projectID, experienceName, originalDescription, originalLocation, EmptySlice, &originalTimeout, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]

	// 1. Update the experience name alone and verify
	newName := "updated-experience-name"
	output = s.runCommand(updateExperience(projectID, experienceIDString, Ptr(newName), nil, nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, originalDescription, originalLocation, originalTimeoutSeconds)

	// 2. Update the description alone and verify
	newDescription := "updated description"
	output = s.runCommand(updateExperience(projectID, experienceIDString, nil, Ptr(newDescription), nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, newDescription, originalLocation, originalTimeoutSeconds)

	// 3. Update the location alone and verify
	newLocation := "updated location"
	output = s.runCommand(updateExperience(projectID, experienceIDString, nil, nil, Ptr(newLocation), nil), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, newDescription, newLocation, originalTimeoutSeconds)

	// 4. Update the timeout alone and verify
	newTimeoutSeconds := int32(300)
	newTimeout := time.Duration(newTimeoutSeconds) * time.Second
	output = s.runCommand(updateExperience(projectID, experienceIDString, nil, nil, nil, &newTimeout), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds)

	// 5. Update the name and description and verify
	newName = "final-experience-name"
	newDescription = "final description"
	output = s.runCommand(updateExperience(projectID, experienceIDString, Ptr(newName), Ptr(newDescription), nil, nil), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds)

	// 6. Update the name, description, and location and verify
	newLocation = "final location"
	output = s.runCommand(updateExperience(projectID, experienceIDString, Ptr(newName), Ptr(newDescription), Ptr(newLocation), nil), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds)

	// 7. Update the name, description, location, and timeout and verify
	newTimeoutSeconds = int32(400)
	newTimeout = time.Duration(newTimeoutSeconds) * time.Second
	output = s.runCommand(updateExperience(projectID, experienceIDString, Ptr(newName), Ptr(newDescription), Ptr(newLocation), Ptr(newTimeout)), ExpectNoError)
	s.verifyExperienceUpdate(projectID, experienceIDString, newName, newDescription, newLocation, newTimeoutSeconds)

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestBatchAndLogs() {
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// This test does not use parameters, so we create an empty parameter map:
	emptyParameterMap := map[string]string{}
	// First create two experiences:
	experienceName1 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/test-object/", s.Config.E2EBucket)
	output = s.runCommand(createExperience(projectID, experienceName1, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(createExperience(projectID, experienceName2, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Archive the experiences when possible

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/resim/open-builds/log-ingest:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create an experience tag:
	tagName := fmt.Sprintf("test-tag-%s", uuid.New().String())
	output = s.runCommand(createExperienceTag(projectID, tagName, "testing tag"), ExpectNoError)

	// List experience tags and check the list contains the tag we just created
	output = s.runCommand(listExperienceTags(projectID), ExpectNoError)
	s.Contains(output.StdOut, tagName)

	// Tag one of the experiences:
	output = s.runCommand(tagExperience(projectID, tagName, experienceID1), ExpectNoError)

	// Adding the same tag again should error:
	output = s.runCommand(tagExperience(projectID, tagName, experienceID1), ExpectError)

	// List experiences for the tag
	output = s.runCommand(listExperiencesWithTag(projectID, tagName), ExpectNoError)
	var tagExperiences []api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(1, len(tagExperiences))

	// Tag 2nd, check list contains 2 experiences
	output = s.runCommand(tagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(2, len(tagExperiences))

	// Untag and check list again
	output = s.runCommand(untagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(1, len(tagExperiences))

	// Fail to list batch logs without a batch specifier
	output = s.runCommand(listBatchLogs(projectID, "", ""), ExpectError)
	s.Contains(output.StdErr, SelectOneRequired)

	// Fail to list batch logs with a bad UUID
	output = s.runCommand(listBatchLogs(projectID, "not-a-uuid", ""), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Fail to list batch logs with a made up batch name
	output = s.runCommand(listBatchLogs(projectID, "", "not-a-valid-batch-name"), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)

	// Fail to create a batch without any experience ids, tags, or names
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	s.Contains(output.StdErr, SelectOneRequired)

	// Create a batch with (only) experience names using the --experiences flag
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName1, experienceName2}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH1 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH1)

	// Create a batch with mixed experience names and IDs in the --experiences flag
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName1, experienceIDString2}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH2 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH2)

	// Create a batch with an ID in the --experiences flag and a tag name in the --experience-tags flag (experience 1 is in the tag)
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceIDString2}, []string{tagName}, "", GithubTrue, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH3 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH3)

	// Create a batch without metrics with the github flag set, with a specified batch name and check the output
	batchName := fmt.Sprintf("test-batch-%s", uuid.New().String())
	output = s.runCommand(createBatch(projectID, buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, emptyParameterMap, AssociatedAccount, &batchName, Ptr(100)), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH4 := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH4)

	// Get the batch by name:
	output = s.runCommand(getBatchByName(projectID, batchName, ExitStatusFalse), ExpectNoError)
	s.Contains(output.StdOut, batchName)
	s.Contains(output.StdOut, batchIDStringGH4)

	// Now create a batch without the github flag, but with metrics
	output = s.runCommand(createBatch(projectID, buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, metricsBuildIDString, GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, CreatedBatch)
	s.Empty(output.StdErr)

	// Extract from "Batch ID:" to the next newline:
	re := regexp.MustCompile(`Batch ID: (.+?)\n`)
	matches := re.FindStringSubmatch(output.StdOut)
	s.Equal(2, len(matches))
	batchIDString := strings.TrimSpace(matches[1])
	batchID := uuid.MustParse(batchIDString)
	// Extract the batch name:
	re = regexp.MustCompile(`Batch name: (.+?)\n`)
	matches = re.FindStringSubmatch(output.StdOut)
	s.Equal(2, len(matches))
	batchNameString := strings.TrimSpace(matches[1])
	// RePun:
	batchNameParts := strings.Split(batchNameString, "-")
	s.Equal(3, len(batchNameParts))
	// Try a batch without any experiences:
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	s.Contains(output.StdErr, SelectOneRequired)
	// Try a batch without a build id:
	output = s.runCommand(createBatch(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	s.Contains(output.StdErr, InvalidBuildID)
	// Try a batch with both experience tag ids and experience tag names (even if fake):
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{"tag-id"}, []string{"tag-name"}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, nil), ExpectError)
	s.Contains(output.StdErr, BranchTagMutuallyExclusive)

	// Try a batch with non-percentage allowable failure percent:
	output = s.runCommand(createBatch(projectID, buildIDString, []string{experienceIDString1}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap, AssociatedAccount, nil, Ptr(101)), ExpectError)
	s.Contains(output.StdErr, AllowableFailurePercent)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
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
		s.Contains(AcceptableBatchStatusCodes, exitCode)
		s.Empty(stderr.String())
		s.Empty(stdout.String())
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
	output = s.runCommand(getBatchByName(projectID, batchNameString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchNameString, *batch.FriendlyName)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(metricsBuildID, *batch.MetricsBuildID)
	// Validate that it succeeded:
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Get the batch by ID:
	output = s.runCommand(getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchNameString, *batch.FriendlyName)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(AssociatedAccount, batch.AssociatedAccount)
	// Validate that it succeeded:
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)

	// List the logs for the succeeded batch
	output = s.runCommand(listBatchLogs(projectID, "", batchNameString), ExpectNoError)
	// Marshal into a struct:
	var batchLogs []api.BatchLog
	err = json.Unmarshal([]byte(output.StdOut), &batchLogs)
	s.NoError(err)
	// Validate that one or more logs are returned
	s.Greater(len(batchLogs), 0)
	for _, batchLog := range batchLogs {
		uuid.MustParse(batchLog.LogID.String())
	}

	// Pass blank name / id to batches get:
	output = s.runCommand(getBatchByName(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, RequireBatchName)
	output = s.runCommand(getBatchByID(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, RequireBatchName)

	// Pass unknown name / id to batches tests:
	output = s.runCommand(getBatchByName(projectID, "does not exist", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)
	output = s.runCommand(getBatchByID(projectID, "0000-0000-0000-0000-000000000000", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Now grab the tests from the batch:
	output = s.runCommand(getBatchJobsByName(projectID, batchNameString), ExpectNoError)
	// Marshal into a struct:
	var tests []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &tests)
	s.NoError(err)
	s.Equal(2, len(tests))
	for _, test := range tests {
		s.Contains([]uuid.UUID{experienceID1, experienceID2}, *test.ExperienceID)
		s.Equal(buildID, *test.BuildID)
	}
	output = s.runCommand(getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &tests)
	s.NoError(err)
	s.Equal(2, len(tests))
	for _, test := range tests {
		s.Contains([]uuid.UUID{experienceID1, experienceID2}, *test.ExperienceID)
		s.Equal(buildID, *test.BuildID)
	}

	testID2 := *tests[1].JobID
	// Pass blank name / id to batches tests:
	output = s.runCommand(getBatchJobsByName(projectID, ""), ExpectError)
	s.Contains(output.StdErr, RequireBatchName)
	output = s.runCommand(getBatchJobsByID(projectID, ""), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Pass unknown name / id to batches tests:
	output = s.runCommand(getBatchJobsByName(projectID, "does not exist"), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)

	// List test logs:
	output = s.runCommand(listLogs(projectID, batchIDString, testID2.String()), ExpectNoError)
	// Marshal into a struct:
	var logs []api.JobLog
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	s.NoError(err)
	s.Len(logs, 7)
	for _, log := range logs {
		s.Equal(testID2, *log.JobID)
		s.Contains([]string{"experience-worker.log", "metrics-worker.log", "experience-container.log", "metrics-container.log", "resource_metrics.binproto", "logs.zip", "file.name"}, *log.FileName)
	}

	// Download a single test log
	tempDir, err := os.MkdirTemp("", "test-logs")
	s.NoError(err)
	output = s.runCommand(downloadLogs(projectID, batchIDString, testID2.String(), tempDir, []string{"file.name"}), ExpectNoError)
	s.Contains(output.StdOut, fmt.Sprintf("Downloaded 1 log(s) to %s", tempDir))

	// Download all test logs:
	output = s.runCommand(downloadLogs(projectID, batchIDString, testID2.String(), tempDir, []string{}), ExpectNoError)
	s.Contains(output.StdOut, fmt.Sprintf("Downloaded 7 log(s) to %s", tempDir))

	// Check that the logs were downloaded and unzipped:
	files, err := os.ReadDir(tempDir)
	s.NoError(err)
	s.Len(files, 7)
	for _, file := range files {
		s.Contains([]string{"experience-worker.log", "metrics-worker.log", "experience-container.log", "metrics-container.log", "resource_metrics.binproto", "logs", "file.name"}, file.Name())
	}

	// Pass blank name / id to logs:
	output = s.runCommand(listLogs(projectID, "not-a-uuid", testID2.String()), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)
	output = s.runCommand(listLogs(projectID, batchIDString, "not-a-uuid"), ExpectError)
	s.Contains(output.StdErr, InvalidTestID)

	// Ensure the rest of the batches complete:
	s.Eventually(func() bool {
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
			s.Contains(AcceptableBatchStatusCodes, exitCode)
			s.Empty(stderr.String())
			s.Empty(stdout.String())
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
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestParameterizedBatch() {
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
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
	output = s.runCommand(createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString1)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create a batch with (only) experience names using the --experiences flag with some parameters
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName}, []string{}, metricsBuildIDString, GithubTrue, expectedParameterMap, AssociatedAccount, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDStringGH)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
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
		s.Contains(AcceptableBatchStatusCodes, exitCode)
		s.Empty(stderr.String())
		s.Empty(stdout.String())
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
	output = s.runCommand(getBatchByID(projectID, batchIDStringGH, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchID, *batch.BatchID)
	// Validate that it succeeded:
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	s.Equal(api.BatchParameters(expectedParameterMap), *batch.Parameters)
	s.Equal(buildID, *batch.BuildID)
	s.Equal(metricsBuildID, *batch.MetricsBuildID)

	// Archive the project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}
func (s *EndToEndTestSuite) TestCreateSweepParameterNameAndValues() {
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// create two experiences:
	experienceName1 := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(createExperience(projectID, experienceName1, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	output = s.runCommand(createExperience(projectID, experienceName2, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Archive the experiences when possible

	// Now create the branch:
	branchName := fmt.Sprintf("sweep-test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Create an experience tag:
	tagName := fmt.Sprintf("test-tag-%s", uuid.New().String())
	output = s.runCommand(createExperienceTag(projectID, tagName, "testing tag"), ExpectNoError)

	// List experience tags and check the list contains the tag we just created
	output = s.runCommand(listExperienceTags(projectID), ExpectNoError)
	s.Contains(output.StdOut, tagName)

	// Tag one of the experiences:
	output = s.runCommand(tagExperience(projectID, tagName, experienceID1), ExpectNoError)

	// Adding the same tag again should error:
	output = s.runCommand(tagExperience(projectID, tagName, experienceID1), ExpectError)

	// List experiences for the tag
	output = s.runCommand(listExperiencesWithTag(projectID, tagName), ExpectNoError)
	var tagExperiences []api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(1, len(tagExperiences))

	// Tag 2nd, check list contains 2 experiences
	output = s.runCommand(tagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(2, len(tagExperiences))

	// Define the parameters:
	parameterName := "test-parameter"
	parameterValues := []string{"value1", "value2", "value3"}
	// Create a sweep with (only) experience names using the --experiences flag and specific parameter name and values (and "" for no config file location)
	output = s.runCommand(createSweep(projectID, buildIDString, []string{experienceName1, experienceName2}, []string{}, metricsBuildIDString, parameterName, parameterValues, "", GithubTrue, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSweep)
	sweepIDStringGH := output.StdOut[len(GithubCreatedSweep) : len(output.StdOut)-1]
	uuid.MustParse(sweepIDStringGH)

	// Create a sweep with (only) experience tag names using the --experience-tags flag and a config file location
	// The config file location is in a subdirectory of the testing directory called 'data' and is called valid_sweep_config.json:
	// Find the current working directory:
	cwd, err := os.Getwd()
	s.NoError(err)
	// Create the config location:
	configLocation := fmt.Sprintf("%s/data/valid_sweep_config.json", cwd)
	output = s.runCommand(createSweep(projectID, buildIDString, []string{}, []string{tagName}, "", "", []string{}, configLocation, GithubFalse, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, CreatedSweep)
	s.Empty(output.StdErr)

	// Extract from "Sweep ID:" to the next newline:
	re := regexp.MustCompile(`Sweep ID: (.+?)\n`)
	matches := re.FindStringSubmatch(output.StdOut)
	s.Equal(2, len(matches))
	secondSweepIDString := strings.TrimSpace(matches[1])
	secondSweepID := uuid.MustParse(secondSweepIDString)
	// Extract the sweep name:
	re = regexp.MustCompile(`Sweep name: (.+?)\n`)
	matches = re.FindStringSubmatch(output.StdOut)
	s.Equal(2, len(matches))
	sweepNameString := strings.TrimSpace(matches[1])
	// RePun:
	sweepNameParts := strings.Split(sweepNameString, "-")
	s.Equal(3, len(sweepNameParts))
	// Try a sweep without any experiences:
	output = s.runCommand(createSweep(projectID, buildIDString, []string{}, []string{}, "", parameterName, parameterValues, "", GithubFalse, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, FailedToCreateSweep)
	// Try a sweep without a build id:
	output = s.runCommand(createSweep(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, "", parameterName, parameterValues, "", GithubFalse, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, InvalidBuildID)

	// Try a sweep with both parameter name and config (even if fake):
	output = s.runCommand(createSweep(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, "", parameterName, parameterValues, "config location", GithubFalse, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, ConfigParamsMutuallyExclusive)

	// Try a sweep with an invalid config
	invalidConfigLocation := fmt.Sprintf("%s/data/invalid_sweep_config.json", cwd)
	output = s.runCommand(createSweep(projectID, buildIDString, []string{}, []string{tagName}, "", "", []string{}, invalidConfigLocation, GithubFalse, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, InvalidGridSearchFile)

	// Get sweep passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the sweep, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
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
		s.Contains(AcceptableSweepStatusCodes, exitCode)
		s.Empty(stderr.String())
		s.Empty(stdout.String())
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
	output = s.runCommand(getSweepByName(projectID, sweepNameString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var sweep api.ParameterSweep
	err = json.Unmarshal([]byte(output.StdOut), &sweep)
	s.NoError(err)
	s.Equal(sweepNameString, *sweep.Name)
	s.Equal(secondSweepID, *sweep.ParameterSweepID)
	// Validate that it succeeded:
	s.Equal(api.ParameterSweepStatusSUCCEEDED, *sweep.Status)
	// Get the sweep by ID:
	output = s.runCommand(getSweepByID(projectID, secondSweepIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &sweep)
	s.NoError(err)
	s.Equal(sweepNameString, *sweep.Name)
	s.Equal(secondSweepID, *sweep.ParameterSweepID)
	// Validate that it succeeded:
	s.Equal(api.ParameterSweepStatusSUCCEEDED, *sweep.Status)

	// Validate that the sweep has the correct parameters:
	passedParameters := []api.SweepParameter{}
	// Read from the valid config file:
	configFile, err := os.Open(configLocation)
	s.NoError(err)
	defer configFile.Close()
	byteValue, err := io.ReadAll(configFile)
	s.NoError(err)
	err = json.Unmarshal(byteValue, &passedParameters)
	s.NoError(err)
	s.Equal(passedParameters, *sweep.Parameters)
	// Figure out how many batches to expect:
	numBatches := 1
	for _, param := range passedParameters {
		numBatches *= len(*param.Values)
	}
	s.Len(*sweep.Batches, numBatches)

	// Pass blank name / id to batches get:
	output = s.runCommand(getSweepByName(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidSweepNameOrID)
	output = s.runCommand(getSweepByID(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidSweepNameOrID)

	// Check we can list the sweeps, and our new sweep is in it:
	output = s.runCommand(listSweeps(projectID), ExpectNoError)
	s.Contains(output.StdOut, sweepNameString)

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

// Test Cancel Sweep::
func (s *EndToEndTestSuite) TestCancelSweep() {
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// create an experience:
	experienceName := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
	// Now create the branch:
	branchName := fmt.Sprintf("sweep-test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Define the parameters:
	parameterName := "test-parameter"
	parameterValues := []string{"value1", "value2", "value3"}
	// Create a sweep with (only) experience names using the --experiences flag and specific parameter name and values (and "" for no config file location)
	output = s.runCommand(createSweep(projectID, buildIDString, []string{experienceName}, []string{}, metricsBuildIDString, parameterName, parameterValues, "", GithubTrue, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSweep)
	sweepIDStringGH := output.StdOut[len(GithubCreatedSweep) : len(output.StdOut)-1]
	uuid.MustParse(sweepIDStringGH)

	time.Sleep(30 * time.Second) // arbitrary sleep to make sure the scheduler gets the batch and triggers it

	// Cancel the sweep:
	output = s.runCommand(cancelSweep(projectID, sweepIDStringGH), ExpectNoError)
	s.Contains(output.StdOut, CancelledSweep)

	// Get sweep passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the sweep, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
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
		s.Contains(AcceptableSweepStatusCodes, exitCode)
		s.Empty(stderr.String())
		s.Empty(stdout.String())
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
	output = s.runCommand(getSweepByID(projectID, sweepIDStringGH, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var sweep api.ParameterSweep
	err := json.Unmarshal([]byte(output.StdOut), &sweep)
	s.NoError(err)
	s.Equal(sweepIDStringGH, sweep.ParameterSweepID.String())
	// Validate that it was cancelled:
	s.Equal(api.ParameterSweepStatusCANCELLED, *sweep.Status)
}

// Test the metrics builds:
func (s *EndToEndTestSuite) TestCreateMetricsBuild() {
	fmt.Println("Testing metrics build creation")
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create two systems to add as part of the experience creation:
	systemName1 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName1, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString1 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString1)
	systemName2 := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName2, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString2 := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString2)

	systemNames := []string{systemName1, systemName2}
	metricsBuildName := fmt.Sprintf("metrics-build-%s", uuid.New().String())
	output = s.runCommand(createMetricsBuild(projectID, metricsBuildName, "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", systemNames, GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedMetricsBuild)
	s.Empty(output.StdErr)
	// Validate that the metrics build is available for each system:
	for _, systemName := range systemNames {
		output = s.runCommand(systemMetricsBuilds(projectIDString, systemName), ExpectNoError)
		s.Contains(output.StdOut, metricsBuildName)
	}
	// Verify that each of the required flags are required:
	output = s.runCommand(createMetricsBuild(projectID, "", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildName)
	output = s.runCommand(createMetricsBuild(projectID, "name", "", "1.0.0", EmptySlice, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildImage)
	output = s.runCommand(createMetricsBuild(projectID, "name", "public.ecr.aws/docker/library/hello-world:latest", "", EmptySlice, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildVersion)
	output = s.runCommand(createMetricsBuild(projectID, "name", "public.ecr.aws/docker/library/hello-world", "1.1.1", EmptySlice, GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidMetricsBuildImage)

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestMetricsBuildGithub() {
	fmt.Println("Testing metrics build creation, with --github flag")
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	output = s.runCommand(createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Check we can list the metrics builds, and our new metrics build is in it:
	output = s.runCommand(listMetricsBuilds(projectID), ExpectNoError)
	s.Contains(output.StdOut, metricsBuildIDString)

	// Archive the project:
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestAliases() {
	fmt.Println("Testing project and branch aliases")
	// First create a project, manually:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
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
	output = s.runCommand([]CommandBuilder{projectCommand, getByNameCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	// Marshal into a struct:
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, project.Name)
	s.Equal(projectID, project.ProjectID)
	// Try with the ID:
	output = s.runCommand([]CommandBuilder{projectCommand, getByIDCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, project.Name)
	s.Equal(projectID, project.ProjectID)

	// Now create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Empty(output.StdErr)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
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
	output = s.runCommand([]CommandBuilder{branchCommand, listBranchesByNameCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	// Marshal into a struct:
	var branches []api.Branch
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	s.NoError(err)
	s.Equal(1, len(branches))
	s.Equal(branchName, branches[0].Name)
	s.Equal(branchID, branches[0].BranchID)
	s.Equal(projectID, branches[0].ProjectID)
	// Now try by ID:
	output = s.runCommand([]CommandBuilder{branchCommand, listBranchesByIDCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	s.NoError(err)
	s.Equal(1, len(branches))
	s.Equal(branchName, branches[0].Name)
	s.Equal(branchID, branches[0].BranchID)
	s.Equal(projectID, branches[0].ProjectID)

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

	output = s.runCommand([]CommandBuilder{buildCommand, createBuildWithNamesCommand}, ExpectNoError)
	s.Contains(output.StdErr, "Warning: Using 'description' to set the build name is deprecated. In the future, 'description' will only set the build's description. Please use --name instead.")
	s.Contains(output.StdOut, CreatedBuild)

	// Now try to create using the id for projects:
	output = s.runCommand([]CommandBuilder{buildCommand, createBuildWithIDCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	s.Contains(output.StdOut, CreatedBuild)

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
	output = s.runCommand([]CommandBuilder{buildCommand, listBuildByNameCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	// Marshal into a struct:
	var builds []api.Build
	err = json.Unmarshal([]byte(output.StdOut), &builds)
	s.NoError(err)
	s.Equal(2, len(builds))
	// List by id
	output = s.runCommand([]CommandBuilder{buildCommand, listBuildByIDCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &builds)
	s.NoError(err)
	s.Equal(2, len(builds))

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
	output = s.runCommand([]CommandBuilder{archiveProjectCommand, archiveProjectByIDCommand}, ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)

	// Finally, create a new project to verify deletion with the old 'name' flag:
	projectName = fmt.Sprintf("test-project-%s", uuid.New().String())
	output = s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Empty(output.StdErr)
	s.Contains(output.StdOut, GithubCreatedProject)
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
	output = s.runCommand([]CommandBuilder{archiveProjectCommand, archiveProjectByNameCommand}, ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestTestSuites() {
	fmt.Println("Testing test suites")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
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
	output = s.runCommand(createSystem(projectIDString, systemName, systemDescription, Ptr(buildVCPUs), Ptr(buildGPUs), Ptr(buildMemoryMiB), Ptr(buildSharedMemoryMB), Ptr(metricsBuildVCPUs), Ptr(metricsBuildGPUs), Ptr(metricsBuildMemoryMiB), Ptr(metricsBuildSharedMemoryMB), GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)
	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)

	// Now create a few experiences:
	NUM_EXPERIENCES := 4
	experienceNames := make([]string, NUM_EXPERIENCES)
	experienceIDs := make([]uuid.UUID, NUM_EXPERIENCES)
	for i := 0; i < NUM_EXPERIENCES; i++ {
		experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
		experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
		output = s.runCommand(createExperience(projectID, experienceName, "description", experienceLocation, []string{systemName}, nil, GithubTrue), ExpectNoError)
		s.Contains(output.StdOut, GithubCreatedExperience)
		s.Empty(output.StdErr)
		// We expect to be able to parse the experience ID as a UUID
		experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
		experienceID := uuid.MustParse(experienceIDString)
		experienceNames[i] = experienceName
		experienceIDs[i] = experienceID
	}

	// Finally, a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Now, create a test suite with all our experiences, the system, and a metrics build:
	firstTestSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	testSuiteDescription := "test suite description"
	output = s.runCommand(createTestSuite(projectID, firstTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedTestSuite)
	// Try with the github flag:
	secondTestSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	output = s.runCommand(createTestSuite(projectID, secondTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedTestSuite)
	// Parse the output
	testSuiteIDRevisionString := output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	// Split into the UUID and revision:
	testSuiteIDRevision := strings.Split(testSuiteIDRevisionString, "/")
	s.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision := testSuiteIDRevision[1]
	s.Equal("0", revision)

	// Failure possibilities:
	// Try to create a test suite with an empty system:
	output = s.runCommand(createTestSuite(projectID, "test-suite", "description", "", experienceNames, metricsBuildIDString, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyTestSuiteSystemName)
	output = s.runCommand(createTestSuite(projectID, "", "description", systemName, experienceNames, metricsBuildIDString, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyTestSuiteName)
	output = s.runCommand(createTestSuite(projectID, "test-suite", "", systemName, experienceNames, metricsBuildIDString, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyTestSuiteDescription)
	output = s.runCommand(createTestSuite(projectID, "test-suite", "description", systemName, []string{}, metricsBuildIDString, GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyTestSuiteExperiences)
	output = s.runCommand(createTestSuite(projectID, "test-suite", "description", systemName, experienceNames, "not-a-uuid", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyTestSuiteMetricsBuild)

	// Revise the test suite:
	// Now, create a test suite with all our experiences, the system, and a metrics build:
	output = s.runCommand(reviseTestSuite(projectID, firstTestSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, nil, GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, RevisedTestSuite)
	// Revise w/ github flag
	output = s.runCommand(reviseTestSuite(projectID, firstTestSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedTestSuite)
	testSuiteIDRevisionString = output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	testSuiteIDRevision = strings.Split(testSuiteIDRevisionString, "/")
	s.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision = testSuiteIDRevision[1]
	s.Equal("2", revision)

	// Now list the test suites
	output = s.runCommand(listTestSuites(projectID), ExpectNoError)
	// Parse the output into a list of test suites:
	var testSuites []api.TestSuite
	err := json.Unmarshal([]byte(output.StdOut), &testSuites)
	s.NoError(err)
	s.Len(testSuites, 2)
	s.Equal(firstTestSuiteName, testSuites[0].Name)
	s.Contains(secondTestSuiteName, testSuites[1].Name)
	// Then get a specific revision etc
	zerothRevision := int32(0)
	output = s.runCommand(getTestSuite(projectID, firstTestSuiteName, Ptr(zerothRevision), false), ExpectNoError)
	// Parse the output into a test suite:
	var testSuite api.TestSuite
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	s.NoError(err)
	s.Equal(firstTestSuiteName, testSuite.Name)
	s.Equal(zerothRevision, testSuite.TestSuiteRevision)
	s.ElementsMatch(experienceIDs, testSuite.Experiences)
	secondRevision := int32(2)
	output = s.runCommand(getTestSuite(projectID, firstTestSuiteName, Ptr(secondRevision), false), ExpectNoError)
	// Parse the output into a test suite:
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	s.NoError(err)
	s.Equal(firstTestSuiteName, testSuite.Name)
	s.Equal(secondRevision, testSuite.TestSuiteRevision)
	s.Len(testSuite.Experiences, 1)
	s.ElementsMatch(experienceIDs[0], testSuite.Experiences[0])
	// Then run.
	output = s.runCommand(runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubFalse, AssociatedAccount, nil, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, CreatedTestSuiteBatch)
	// Then list the test suite batches
	output = s.runCommand(getTestSuiteBatches(projectID, firstTestSuiteName, nil), ExpectNoError)
	// Parse the output into a list of test suite batches:
	var testSuiteBatches []api.Batch
	err = json.Unmarshal([]byte(output.StdOut), &testSuiteBatches)
	s.NoError(err)
	s.Len(testSuiteBatches, 1)
	// Then get the test suite batch
	batch := testSuiteBatches[0]
	s.Equal(buildIDString, batch.BuildID.String())

	// Create a new run using github and with a specific batch name:
	batchName := fmt.Sprintf("test-batch-%s", uuid.New().String())
	output = s.runCommand(runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubTrue, AssociatedAccount, &batchName, Ptr(100), nil), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	// Parse the output to get a batch id:
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)
	// Get the batch:
	output = s.runCommand(getBatchByName(projectID, batchName, ExitStatusFalse), ExpectNoError)
	s.Contains(output.StdOut, batchName)
	s.Contains(output.StdOut, batchIDString)
	// Then list the test suite batches
	output = s.runCommand(getTestSuiteBatches(projectID, firstTestSuiteName, nil), ExpectNoError)
	// Parse the output into a list of test suite batches:
	err = json.Unmarshal([]byte(output.StdOut), &testSuiteBatches)
	s.NoError(err)
	s.Len(testSuiteBatches, 2)
	found := false
	for _, batch := range testSuiteBatches {
		s.Equal(buildIDString, batch.BuildID.String())
		if batch.BatchID.String() == batchIDString {
			found = true
		}
	}
	s.True(found)

	// Try running a test suite with a non-percentage allowable failure percent:
	output = s.runCommand(runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubFalse, AssociatedAccount, nil, Ptr(101), nil), ExpectError)
	s.Contains(output.StdErr, AllowableFailurePercent)
	output = s.runCommand(runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubFalse, AssociatedAccount, nil, Ptr(-1), nil), ExpectError)
	s.Contains(output.StdErr, AllowableFailurePercent)

	// Try running a test suite with a metrics build override:
	output = s.runCommand(runTestSuite(projectID, firstTestSuiteName, nil, buildIDString, map[string]string{}, GithubTrue, AssociatedAccount, nil, nil, Ptr(metricsBuildIDString)), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	// Parse the output to get a batch id:
	batchIDString = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)
	// Get the batch:
	output = s.runCommand(getBatchByName(projectID, batchName, ExitStatusFalse), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(metricsBuildIDString, batch.MetricsBuildID.String())
	s.Equal(buildIDString, batch.BuildID.String())
	// Get the jobs for the batch:
	output = s.runCommand(getBatchJobsByName(projectID, batchName), ExpectNoError)
	jobs := []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Len(jobs, 1)
	// The job should have the correct experience ID:
	s.Equal(experienceIDs[0], *jobs[0].ExperienceID)
}

func (s *EndToEndTestSuite) TestReports() {
	fmt.Println("Testing reports")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
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
	output = s.runCommand(createSystem(projectIDString, systemName, systemDescription, Ptr(buildVCPUs), Ptr(buildGPUs), Ptr(buildMemoryMiB), Ptr(buildSharedMemoryMB), Ptr(metricsBuildVCPUs), Ptr(metricsBuildGPUs), Ptr(metricsBuildMemoryMiB), Ptr(metricsBuildSharedMemoryMB), GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)
	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
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
		output = s.runCommand(createExperience(projectID, experienceName, "description", experienceLocation, []string{systemName}, nil, GithubTrue), ExpectNoError)
		s.Contains(output.StdOut, GithubCreatedExperience)
		s.Empty(output.StdErr)
		// We expect to be able to parse the experience ID as a UUID
		experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
		experienceID := uuid.MustParse(experienceIDString)
		experienceNames[i] = experienceName
		experienceIDs[i] = experienceID
	}

	// Finally, a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Now, create a test suite with all our experiences, the system, and a metrics build:
	testSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	testSuiteDescription := "test suite description"
	output = s.runCommand(createTestSuite(projectID, testSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedTestSuite)
	// Try with the github flag:
	secondTestSuiteName := fmt.Sprintf("test-suite-%s", uuid.New().String())
	output = s.runCommand(createTestSuite(projectID, secondTestSuiteName, testSuiteDescription, systemName, experienceNames, metricsBuildIDString, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedTestSuite)
	// Parse the output
	testSuiteIDRevisionString := output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	// Split into the UUID and revision:
	testSuiteIDRevision := strings.Split(testSuiteIDRevisionString, "/")
	s.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision := testSuiteIDRevision[1]
	s.Equal("0", revision)

	// Get the test suite and validate the show on summary flag is false:
	output = s.runCommand(getTestSuite(projectID, testSuiteName, Ptr(int32(0)), false), ExpectNoError)
	// Parse the output into a test suite:
	var testSuite api.TestSuite
	err := json.Unmarshal([]byte(output.StdOut), &testSuite)
	s.NoError(err)
	s.False(testSuite.ShowOnSummary)

	// Revise w/ github flag
	output = s.runCommand(reviseTestSuite(projectID, testSuiteName, nil, nil, nil, Ptr([]string{experienceNames[0]}), nil, Ptr(true), GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedTestSuite)
	testSuiteIDRevisionString = output.StdOut[len(GithubCreatedTestSuite) : len(output.StdOut)-1]
	testSuiteIDRevision = strings.Split(testSuiteIDRevisionString, "/")
	s.Len(testSuiteIDRevision, 2)
	uuid.MustParse(testSuiteIDRevision[0])
	revision = testSuiteIDRevision[1]
	s.Equal("1", revision)
	// Get the test suite and validate the show on summary flag is true:
	output = s.runCommand(getTestSuite(projectID, testSuiteName, Ptr(int32(1)), false), ExpectNoError)
	// Parse the output into a test suite:
	err = json.Unmarshal([]byte(output.StdOut), &testSuite)
	s.NoError(err)
	s.True(testSuite.ShowOnSummary)

	// 1. Create a report passing in length and have it correct [with name, with respect revision and explicit revision 0]
	reportName := fmt.Sprintf("test-report-%s", uuid.New().String())
	output = s.runCommand(createReport(projectID, testSuiteName, Ptr(int32(0)), branchName, metricsBuildIDString, Ptr(28), nil, nil, Ptr(true), Ptr(reportName), GithubFalse, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, CreatedReport)
	s.Contains(output.StdOut, EndTimestamp)
	s.Contains(output.StdOut, StartTimestamp)
	// Create a second with the github flag and validate the timestamps are correct:
	reportName = fmt.Sprintf("test-report-%s", uuid.New().String())
	output = s.runCommand(createReport(projectID, testSuiteName, Ptr(int32(0)), branchName, metricsBuildIDString, Ptr(28), nil, nil, Ptr(true), Ptr(reportName), GithubTrue, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedReport)
	// Extract the report id:
	reportIDString := output.StdOut[len(GithubCreatedReport) : len(output.StdOut)-1]
	uuid.MustParse(reportIDString)
	// Get the most recent report, by name:
	output = s.runCommand(getReportByName(projectID, reportName, false), ExpectNoError)
	// Parse the output into a report:
	var report api.Report
	err = json.Unmarshal([]byte(output.StdOut), &report)
	s.NoError(err)
	s.Equal(reportName, report.Name)
	// Validate that the start timestamp is 4 weeks before today within a one minute duration
	s.WithinDuration(time.Now().UTC().Add(-4*7*24*time.Hour), report.StartTimestamp, time.Minute)
	// Validate the end timestamp is pretty much now:
	s.WithinDuration(time.Now().UTC(), report.EndTimestamp, time.Minute)
	s.Equal(int32(0), report.TestSuiteRevision)
	s.True(report.RespectRevisionBoundary)

	// 2. Create a report passing in start and end timestamps and have them correct [with name, with respect revision and implicit revision 1]
	reportName = fmt.Sprintf("test-report-%s", uuid.New().String())
	startTimestamp := time.Now().UTC().Add(-time.Hour)
	endTimestamp := time.Now().UTC().Add(-5 * time.Minute)
	output = s.runCommand(createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, Ptr(startTimestamp.Format(time.RFC3339)), Ptr(endTimestamp.Format(time.RFC3339)), Ptr(true), Ptr(reportName), GithubTrue, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedReport)
	// Extract the report id:
	reportIDString = output.StdOut[len(GithubCreatedReport) : len(output.StdOut)-1]
	uuid.MustParse(reportIDString)
	// Get the most recent report, by name:
	output = s.runCommand(getReportByName(projectID, reportName, false), ExpectNoError)
	// Parse the output into a report:
	err = json.Unmarshal([]byte(output.StdOut), &report)
	s.NoError(err)
	s.Equal(reportName, report.Name)
	// Validate that the start timestamp is correct within a second
	s.WithinDuration(startTimestamp, report.StartTimestamp, time.Second)
	// Validate the end timestamp is correct within a second
	s.WithinDuration(endTimestamp, report.EndTimestamp, time.Second)
	s.Equal(int32(1), report.TestSuiteRevision)
	s.True(report.RespectRevisionBoundary)

	// 3. Create a report passing in only start timestamp and have it correct [no name, no respect revision and implicit revision 1]
	newStartTimestamp := time.Now().UTC().Add(-time.Hour)
	output = s.runCommand(createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, Ptr(newStartTimestamp.Format(time.RFC3339)), nil, nil, nil, GithubTrue, AssociatedAccount), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedReport)
	// Extract the report id:
	reportIDString = output.StdOut[len(GithubCreatedReport) : len(output.StdOut)-1]
	uuid.MustParse(reportIDString)
	// Get the most recent report, by name:
	output = s.runCommand(getReportByID(projectID, reportIDString, false), ExpectNoError)
	// Parse the output into a report:
	err = json.Unmarshal([]byte(output.StdOut), &report)
	s.NoError(err)
	s.NotEqual("", report.Name)
	// Validate that the start timestamp is correct within a second
	s.WithinDuration(newStartTimestamp, report.StartTimestamp, time.Second)
	// Validate the end timestamp is correct within a minute to now
	s.WithinDuration(time.Now().UTC(), report.EndTimestamp, time.Minute)
	s.Equal(int32(1), report.TestSuiteRevision)
	s.False(report.RespectRevisionBoundary)

	// 4. Fail to create based on invalid timestamps
	output = s.runCommand(createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, Ptr("invalid"), nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, FailedStartTimestamp)
	output = s.runCommand(createReport(projectID, report.TestSuiteID.String(), nil, branchIDString, metricsBuildIDString, nil, Ptr(newStartTimestamp.Format(time.RFC3339)), Ptr("invalid"), nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, FailedEndTimestamp)
	// 6. Fail to create based on both end and length
	output = s.runCommand(createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, Ptr(28), nil, Ptr(newStartTimestamp.Format(time.RFC3339)), nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, EndLengthMutuallyExclusive)
	// 7. Fail to create based on no start nor length
	output = s.runCommand(createReport(projectID, testSuiteName, nil, branchName, metricsBuildIDString, nil, nil, Ptr(newStartTimestamp.Format(time.RFC3339)), nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, AtLeastOneReport)
	// 7. Fail to create, no test suite id, no branch id, no metrics build
	output = s.runCommand(createReport(projectID, "", nil, "", "", Ptr(28), nil, nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, TestSuiteNameReport)
	output = s.runCommand(createReport(projectID, testSuiteName, nil, "", "", Ptr(28), nil, nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, BranchNotFoundReport)
	output = s.runCommand(createReport(projectID, testSuiteName, nil, branchName, "", Ptr(28), nil, nil, nil, nil, GithubTrue, AssociatedAccount), ExpectError)
	s.Contains(output.StdErr, FailedToParseMetricsBuildReport)

	//TODO(iain): check the wait and logs commands, once the rest has landed.
}

func (s *EndToEndTestSuite) TestBatchWithZeroTimeout() {
	// Skip this test for now, as it's not working.
	s.T().Skip("Skipping batch creation with a single experience and 0s timeout")
	fmt.Println("Testing batch creation with a single experience and 0s timeout")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Create an experience
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, Ptr(0*time.Second), GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)

	// Now create the branch
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)

	// Attempt to create a batch with a 0s timeout
	output = s.runCommand(createBatch(projectID, buildIDString, []string{experienceIDString}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, map[string]string{}, AssociatedAccount, nil, Ptr(0)), ExpectNoError)
	// Expect the batch to be created successfully
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDString)

	// Now, wait for the batch to complete and validate that it has an error:
	s.Eventually(func() bool {
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
		s.Contains(AcceptableBatchStatusCodes, exitCode)
		s.Empty(stderr.String())
		s.Empty(stdout.String())
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
	output = s.runCommand(getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	s.Contains(output.StdOut, "ERROR")

	// Archive the project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestLogIngest() {
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// Create the system
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]

	// A metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// There are no branches and there are no builds; we use the ingest command to create:
	firstBranchName := "test-branch"
	firstVersion := "first-version"
	logName := fmt.Sprintf("test-log-%s", uuid.New().String())

	logLocation := fmt.Sprintf("s3://%v/test-object/", s.Config.E2EBucket)

	experienceTags := []string{"test-tag"}
	ingestCommand := createIngestedLog(projectID, &systemIDString, &firstBranchName, &firstVersion, metricsBuildID, Ptr(logName), Ptr(logLocation), []string{}, nil, experienceTags, nil, nil, GithubTrue)
	output = s.runCommand(ingestCommand, ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDString)

	// Await the batch to complete:
	s.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(s, projectID, batchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(getBatchByID(projectID, batchID.String(), ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	// Get the build and check version:
	output = s.runCommand(getBuild(projectIDString, *batch.BuildID), ExpectNoError)
	var build api.Build
	err = json.Unmarshal([]byte(output.StdOut), &build)
	s.NoError(err)
	s.Equal(firstVersion, build.Version)
	// Get the branch and check the name:
	output = s.runCommand(listBranches(projectID), ExpectNoError)
	var branches []api.Branch
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	s.NoError(err)
	s.Equal(1, len(branches))
	s.Equal(firstBranchName, branches[0].Name)

	// Get the job ID:
	output = s.runCommand(getBatchJobsByID(projectID, batchID.String()), ExpectNoError)
	var jobs []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(1, len(jobs))
	jobID := jobs[0].JobID

	// Check the logs and ensure the `file.name` file exists:
	output = s.runCommand(listLogs(projectID, batchID.String(), jobID.String()), ExpectNoError)
	logs := []api.Log{}
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	s.NoError(err)
	found := false
	for _, log := range logs {
		if log.FileName != nil && *log.FileName == "file.name" {
			found = true
			break
		}
	}
	s.True(found)

	// Get the experience:
	output = s.runCommand(getExperience(projectID, jobs[0].ExperienceID.String()), ExpectNoError)
	var experience api.Experience
	err = json.Unmarshal([]byte(output.StdOut), &experience)
	s.NoError(err)
	s.Equal(logName, experience.Name)
	s.Equal(logLocation, experience.Location)
	// Finally, validate the tags:
	output = s.runCommand(listExperiencesWithTag(projectID, "ingested-via-resim"), ExpectNoError)
	var experiencesWithTag []api.Experience
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	s.NoError(err)
	s.Equal(1, len(experiencesWithTag))
	s.Equal(logName, experiencesWithTag[0].Name)
	// And the specificed tag
	output = s.runCommand(listExperiencesWithTag(projectID, experienceTags[0]), ExpectNoError)
	experiencesWithTag = []api.Experience{}
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	s.NoError(err)
	s.Equal(1, len(experiencesWithTag))
	s.Equal(logName, experiencesWithTag[0].Name)

	// Now, defaults:
	secondLogName := fmt.Sprintf("test-log-%v", uuid.New())
	secondLogTags := []string{"test-tag-2"}
	defaultBranchName := "log-ingest-branch"
	defaultVersion := "latest"
	secondLogCommand := createIngestedLog(projectID, &systemIDString, nil, nil, metricsBuildID, Ptr(secondLogName), Ptr(logLocation), []string{}, nil, secondLogTags, nil, nil, GithubTrue)
	output = s.runCommand(secondLogCommand, ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	secondBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	secondBatchID := uuid.MustParse(secondBatchIDString)

	// Await the batch to complete:
	s.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(s, projectID, secondBatchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)

	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(getBatchByID(projectID, secondBatchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)

	// Get the build and check version:
	output = s.runCommand(getBuild(projectIDString, *batch.BuildID), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	s.NoError(err)
	s.Equal(defaultVersion, build.Version)
	defaultBuildID := build.BuildID
	// Get the branch and check the name:
	output = s.runCommand(listBranches(projectID), ExpectNoError)
	branches = []api.Branch{}
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	s.NoError(err)
	s.Equal(2, len(branches))
	s.Equal(firstBranchName, branches[1].Name)
	s.Equal(defaultBranchName, branches[0].Name)
	// Get the job ID:
	output = s.runCommand(getBatchJobsByID(projectID, secondBatchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(1, len(jobs))
	jobID = jobs[0].JobID

	// Check the logs and ensure the `file.name` file exists:
	output = s.runCommand(listLogs(projectID, secondBatchIDString, jobID.String()), ExpectNoError)
	logs = []api.Log{}
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	s.NoError(err)
	found = false
	for _, log := range logs {
		if log.FileName != nil && *log.FileName == "file.name" {
			found = true
			break
		}
	}
	s.True(found)

	// Get the experience:
	output = s.runCommand(getExperience(projectID, jobs[0].ExperienceID.String()), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &experience)
	s.NoError(err)
	s.Equal(secondLogName, experience.Name)
	s.Equal(logLocation, experience.Location)
	// Finally, validate the tags:
	output = s.runCommand(listExperiencesWithTag(projectID, "ingested-via-resim"), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	s.NoError(err)
	s.Equal(2, len(experiencesWithTag))
	// And the specificed tag
	output = s.runCommand(listExperiencesWithTag(projectID, secondLogTags[0]), ExpectNoError)
	experiencesWithTag = []api.Experience{}
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	s.NoError(err)
	s.Equal(1, len(experiencesWithTag))
	s.Equal(secondLogName, experiencesWithTag[0].Name)
	// And that there is no overflow
	output = s.runCommand(listExperiencesWithTag(projectID, experienceTags[0]), ExpectNoError)
	experiencesWithTag = []api.Experience{}
	err = json.Unmarshal([]byte(output.StdOut), &experiencesWithTag)
	s.NoError(err)
	s.Equal(1, len(experiencesWithTag))
	s.Equal(logName, experiencesWithTag[0].Name)

	// Validate that things are not recreated:
	thirdLogName := fmt.Sprintf("test-log-%v", uuid.New())
	specialBatchName := "my-batch-name"
	output = s.runCommand(createIngestedLog(projectID, &systemIDString, nil, nil, metricsBuildID, Ptr(thirdLogName), Ptr(logLocation), []string{}, nil, secondLogTags, nil, &specialBatchName, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	thirdBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(getBatchByID(projectID, thirdBatchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(specialBatchName, *batch.FriendlyName)

	// Get the build and check version:
	output = s.runCommand(getBuild(projectIDString, *batch.BuildID), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &build)
	s.NoError(err)
	s.Equal(defaultVersion, build.Version)
	s.Equal(defaultBuildID, build.BuildID)
	// Get the branch and check the name and that no new branches were created:
	output = s.runCommand(listBranches(projectID), ExpectNoError)
	branches = []api.Branch{}
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	s.NoError(err)
	s.Equal(2, len(branches))
	s.Equal(firstBranchName, branches[1].Name)
	s.Equal(defaultBranchName, branches[0].Name)

	//Create a build:
	output = s.runCommand(createBuild(projectName, firstBranchName, systemName, "description", commands.LogIngestURI, "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	existingBuildID := uuid.MustParse(buildIDString)
	// Finally, use the existing build ID:
	fourthLogName := fmt.Sprintf("test-log-%v", uuid.New())
	output = s.runCommand(createIngestedLog(projectID, nil, nil, nil, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	fourthBatchIDString := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(getBatchByID(projectID, fourthBatchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)

	// Check the MuTex parameters:
	output = s.runCommand(createIngestedLog(projectID, &systemIDString, nil, nil, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue), ExpectError)
	s.Contains(output.StdErr, "build-id")
	s.Contains(output.StdErr, "system")
	output = s.runCommand(createIngestedLog(projectID, nil, &firstBranchName, nil, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue), ExpectError)
	s.Contains(output.StdErr, "build-id")
	s.Contains(output.StdErr, "branch")
	output = s.runCommand(createIngestedLog(projectID, nil, nil, &firstVersion, metricsBuildID, Ptr(fourthLogName), Ptr(logLocation), []string{}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue), ExpectError)
	s.Contains(output.StdErr, "build-id")
	s.Contains(output.StdErr, "version")

	// Test the `--log` flag:
	log1Name := fmt.Sprintf("test-log-%v", uuid.New())
	log1 := fmt.Sprintf("%s=%s", log1Name, logLocation)
	log2Name := fmt.Sprintf("test-log-%v", uuid.New())
	log2 := fmt.Sprintf("%s=%s", log2Name, logLocation)
	output = s.runCommand(createIngestedLog(projectID, nil, nil, nil, metricsBuildID, nil, nil, []string{log1, log2}, nil, secondLogTags, Ptr(existingBuildID), nil, GithubTrue), ExpectNoError)
	fmt.Println("Output: ", output.StdOut)
	fmt.Println("Output: ", output.StdErr)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID = uuid.MustParse(batchIDString)
	s.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(s, projectID, batchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	output = s.runCommand(getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Check there are two jobs:
	// Get the job ID:
	output = s.runCommand(getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(2, len(jobs))

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
	s.NoError(err)
	configFileString := string(configFileBytes)

	// Create the config file in the current directory:
	configFileLocation := filepath.Join(os.TempDir(), "valid_log_file.yaml")
	err = os.WriteFile(configFileLocation, []byte(configFileString), 0644)
	s.NoError(err)

	// Run the ingest command with the config file:
	output = s.runCommand(createIngestedLog(projectID, &systemIDString, &firstBranchName, &firstVersion, metricsBuildID, nil, nil, []string{}, Ptr(configFileLocation), nil, nil, nil, GithubTrue), ExpectNoError)
	fmt.Println("Output: ", output.StdOut)
	fmt.Println("Output: ", output.StdErr)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDString = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID = uuid.MustParse(batchIDString)
	s.Eventually(func() bool {
		complete, exitCode := checkBatchComplete(s, projectID, batchID)
		if !complete {
			fmt.Println("Waiting for batch completion, current exitCode:", exitCode)
		} else {
			fmt.Println("Batch completed, with exitCode:", exitCode)
		}
		return complete
	}, 10*time.Minute, 10*time.Second)
	output = s.runCommand(getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Check there are two jobs:
	// Get the job ID:
	output = s.runCommand(getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	jobs = []api.Job{}
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(2, len(jobs))
}

func (s *EndToEndTestSuite) TestMetricsSync() {
	s.Run("NoConfigFiles", func() {
		output := s.runCommand(syncMetrics(true), true)

		s.Contains(output.StdErr, "failed to find ReSim metrics config")
	})

	s.Run("SyncsMetricsConfig", func() {
		os.RemoveAll(".resim") // Cleanup old folder just in case

		err := os.Mkdir(".resim", 0755)
		s.Require().NoError(err)
		defer os.RemoveAll(".resim")
		err = os.Mkdir(".resim/metrics", 0755)
		s.Require().NoError(err)
		err = os.Mkdir(".resim/metrics/templates", 0755)
		s.Require().NoError(err)

		// strings.Join is ugly but using backticks `` is a mess with getting indentation correct
		metricsFile := strings.Join([]string{
			"version: 1",
			"topics:",
			"  ok:",
			"    schema:",
			"      speed: float",
			"metrics:",
			"  Average Speed:",
			"    query_string: SELECT AVG(speed) FROM speed WHERE job_id=$job_id",
			"    template_type: system",
			"    template: line",
			"metrics sets:",
			"  woot:",
			"    metrics:",
			"      - Average Speed",
		}, "\n")
		err = os.WriteFile(".resim/metrics/config.yml", []byte(metricsFile), 0644)
		s.Require().NoError(err)
		err = os.WriteFile(".resim/metrics/templates/bar.json.heex", []byte("{}"), 0644)
		s.Require().NoError(err)

		// Standard behavior is exit 0 with no output
		output := s.runCommand(syncMetrics(false), false)
		s.Equal("", output.StdOut)
		s.Equal("", output.StdErr)

		// Verbose logs a lot of info about what it is doing
		output = s.runCommand(syncMetrics(true), false)
		s.Equal("", output.StdErr)
		s.Contains(output.StdOut, "Looking for metrics config at .resim/metrics/config.yml")
		s.Contains(output.StdOut, "Found template bar.json.heex")
		s.Contains(output.StdOut, "Successfully synced metrics config, and the following templates:")
	})
}

func checkBatchComplete(s *EndToEndTestSuite, projectID uuid.UUID, batchID uuid.UUID) (bool, int) {
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
	s.Contains(AcceptableBatchStatusCodes, exitCode)
	s.Empty(stderr.String())
	s.Empty(stdout.String())
	// Check if the status is 0, complete, 5 cancelled, 2 failed
	complete := (exitCode == 0 || exitCode == 5 || exitCode == 2)
	return complete, exitCode
}

func (s *EndToEndTestSuite) TestCancelBatch() {
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// First create an experience:
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(createExperience(projectID, experienceName, "description", experienceLocation, EmptySlice, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString1)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Create the system:
	systemName := fmt.Sprintf("test-system-%s", uuid.New().String())
	output = s.runCommand(createSystem(projectIDString, systemName, "description", nil, nil, nil, nil, nil, nil, nil, nil, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedSystem)
	// We expect to be able to parse the system ID as a UUID
	systemIDString := output.StdOut[len(GithubCreatedSystem) : len(output.StdOut)-1]
	uuid.MustParse(systemIDString)

	// Now create the build:
	output = s.runCommand(createBuild(projectName, branchName, systemName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Archive builds when possible

	// Create a metrics build:
	output = s.runCommand(createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", EmptySlice, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create a batch with (only) experience names using the --experiences flag with no parameters
	output = s.runCommand(createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName}, []string{}, metricsBuildIDString, GithubTrue, map[string]string{}, AssociatedAccount, nil, nil), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDStringGH)

	time.Sleep(30 * time.Second) // arbitrary sleep to make sure the scheduler gets the batch and triggers it

	// Cancel the batch
	output = s.runCommand(cancelBatchByID(projectID, batchIDStringGH), ExpectNoError)
	s.Contains(output.StdOut, CancelledBatch)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
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
		s.Contains(AcceptableBatchStatusCodes, exitCode)
		s.Empty(stderr.String())
		s.Empty(stdout.String())
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
	output = s.runCommand(getBatchByID(projectID, batchIDStringGH, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchID, *batch.BatchID)
	// Validate that it was cancelled
	s.Equal(api.BatchStatusCANCELLED, *batch.Status)
	s.Equal(buildID, *batch.BuildID)
	s.Equal(metricsBuildID, *batch.MetricsBuildID)

	// Archive the project
	output = s.runCommand(archiveProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, ArchivedProject)
	s.Empty(output.StdErr)
}

func TestEndToEndTestSuite(t *testing.T) {
	viper.AutomaticEnv()
	viper.SetDefault(Config, Dev)
	// Get a default value for the associated account:
	maybeCIAccount := commands.GetCIEnvironmentVariableAccount()
	if maybeCIAccount != "" {
		AssociatedAccount = maybeCIAccount
	}
	fmt.Printf("Running the end to end test with %s account", AssociatedAccount)
	suite.Run(t, new(EndToEndTestSuite))
}
