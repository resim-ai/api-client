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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
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
	DeletedProject          string = "Deleted project"
	ProjectNameCollision    string = "project name matches an existing"
	// Branch Messages
	CreatedBranch       string = "Created branch"
	GithubCreatedBranch string = "branch_id="
	EmptyBranchName     string = "empty branch name"
	EmptyProjectID      string = "empty project ID"
	InvalidBranchType   string = "invalid branch type"
	// Build Messages
	CreatedBuild          string = "Created build"
	GithubCreatedBuild    string = "build_id="
	EmptyBuildDescription string = "empty build description"
	EmptyBuildImage       string = "empty build image URI"
	InvalidBuildImage     string = "failed to parse the image URI"
	EmptyBuildVersion     string = "empty build version"
	BranchNotExist        string = "Branch does not exist"
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
	// Log Messages
	CreatedLog            string = "Created log"
	GithubCreatedLog      string = "log_location="
	EmptyLogFileName      string = "empty log file name"
	EmptyLogChecksum      string = "No checksum was provided"
	EmptyLogBatchID       string = "empty batch ID"
	EmptyLogJobID         string = "empty job ID"
	EmptyLogType          string = "invalid log type"
	EmptyLogExecutionStep string = "invalid execution step"
	InvalidJobID          string = "unable to parse job ID"

	// Sweep Messages
	CreatedSweep                  string = "Created sweep"
	GithubCreatedSweep            string = "sweep_id="
	FailedToCreateSweep           string = "failed to create sweep"
	ConfigParamsMutuallyExclusive string = "if any flags in the group"
	InvalidSweepName              string = "unable to find sweep"
	InvalidSweepID                string = "unable to parse sweep ID"
	InvalidGridSearchFile         string = "failed to parse grid search config file"
)

var AcceptableBatchStatusCodes = [...]int{0, 2, 3, 4, 5}
var AcceptableSweepStatusCodes = [...]int{0, 2, 3, 4} // we do not have cancelled for sweeps yet

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

func (s *EndToEndTestSuite) foldFlags(flags []Flag) []string {
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
		for _, flag := range s.foldFlags(commandBuilder.Flags) {
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

func (s *EndToEndTestSuite) createProject(projectName string, description string, github bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) listProjects() []CommandBuilder {
	projectCommand := CommandBuilder{
		Command: "project", // Implicitly testing alias to old singular noun
	}
	listCommand := CommandBuilder{
		Command: "list",
	}
	return []CommandBuilder{projectCommand, listCommand}
}

func (s *EndToEndTestSuite) getProject(projectName string) []CommandBuilder {
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

func (s *EndToEndTestSuite) deleteProject(projectName string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "projects",
	}
	deleteCommand := CommandBuilder{
		Command: "delete",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
		},
	}
	return []CommandBuilder{projectCommand, deleteCommand}
}

func (s *EndToEndTestSuite) createBranch(projectID uuid.UUID, name string, branchType string, github bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) listBranches(projectID uuid.UUID) []CommandBuilder {
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

func (s *EndToEndTestSuite) createBuild(projectName string, branchName string, description string, image string, version string, github bool, autoCreateBranch bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) listBuilds(projectID uuid.UUID, branchName string) []CommandBuilder {
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
			{
				Name:  "--branch",
				Value: branchName,
			},
		},
	}
	return []CommandBuilder{buildCommand, listCommand}
}

func (s *EndToEndTestSuite) createMetricsBuild(projectID uuid.UUID, name string, image string, version string, github bool) []CommandBuilder {
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
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{metricsBuildCommand, createCommand}
}

func (s *EndToEndTestSuite) listMetricsBuilds(projectID uuid.UUID) []CommandBuilder {
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

func (s *EndToEndTestSuite) createExperience(projectID uuid.UUID, name string, description string, location string, github bool) []CommandBuilder {
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
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{experienceCommand, createCommand}
}

func (s *EndToEndTestSuite) createExperienceTag(projectID uuid.UUID, name string, description string) []CommandBuilder {
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

func (s *EndToEndTestSuite) listExperienceTags(projectID uuid.UUID) []CommandBuilder {
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

func (s *EndToEndTestSuite) tagExperience(projectID uuid.UUID, tag string, experienceID uuid.UUID) []CommandBuilder {
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

func (s *EndToEndTestSuite) untagExperience(projectID uuid.UUID, tag string, experienceID uuid.UUID) []CommandBuilder {
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

func (s *EndToEndTestSuite) listExperiencesWithTag(projectID uuid.UUID, tag string) []CommandBuilder {
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

func (s *EndToEndTestSuite) createBatch(projectID uuid.UUID, buildID string, experienceIDs []string, experienceTagIDs []string, experienceTagNames []string, experiences []string, experienceTags []string, metricsBuildID string, github bool, parameters map[string]string) []CommandBuilder {
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
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{batchCommand, createCommand}
}

func (s *EndToEndTestSuite) getBatchByName(projectID uuid.UUID, batchName string, exitStatus bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) getBatchByID(projectID uuid.UUID, batchID string, exitStatus bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) getBatchJobsByName(projectID uuid.UUID, batchName string) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "jobs",
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

func (s *EndToEndTestSuite) getBatchJobsByID(projectID uuid.UUID, batchID string) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "jobs",
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

func (s *EndToEndTestSuite) listBatchLogs(projectID uuid.UUID, batchID, batchName string) []CommandBuilder {
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

func (s *EndToEndTestSuite) createLog(projectID uuid.UUID, batchID uuid.UUID, jobID uuid.UUID, name string, fileSize string, checksum string, logType string, executionStep string, github bool) []CommandBuilder {
	logCommand := CommandBuilder{
		Command: "logs",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
			{
				Name:  "--project",
				Value: projectID.String(),
			},
			{
				Name:  "--batch-id",
				Value: batchID.String(),
			},
			{
				Name:  "--job-id",
				Value: jobID.String(),
			},
			{
				Name:  "--name",
				Value: name,
			},
			{
				Name:  "--file-size",
				Value: fileSize, //passed as string
			},
			{
				Name:  "--checksum",
				Value: checksum,
			},
			{
				Name:  "--type",
				Value: logType,
			},
			{
				Name:  "--execution-step",
				Value: executionStep,
			},
		},
	}
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{logCommand, createCommand}
}

func (s *EndToEndTestSuite) listLogs(projectID uuid.UUID, batchID string, jobID string) []CommandBuilder {
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
				Name:  "--job-id",
				Value: jobID,
			},
		},
	}
	return []CommandBuilder{logCommand, listCommand}
}

func (s *EndToEndTestSuite) createSweep(projectID uuid.UUID, buildID string, experiences []string, experienceTags []string, metricsBuildID string, parameterName string, parameterValues []string, configFileLocation string, github bool) []CommandBuilder {
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
	return []CommandBuilder{sweepCommand, createCommand}
}

func (s *EndToEndTestSuite) getSweepByName(projectID uuid.UUID, sweepName string, exitStatus bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) getSweepByID(projectID uuid.UUID, sweepID string, exitStatus bool) []CommandBuilder {
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

func (s *EndToEndTestSuite) listSweeps(projectID uuid.UUID) []CommandBuilder {
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
	output := s.runCommand(s.createProject(projectName, "description", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedProject)
	s.Empty(output.StdErr)
	// Validate that repeating that name leads to an error:
	output = s.runCommand(s.createProject(projectName, "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, ProjectNameCollision)

	// Validate that omitting the name leads to an error:
	output = s.runCommand(s.createProject("", "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyProjectName)
	// Validate that omitting the description leads to an error:
	output = s.runCommand(s.createProject(projectName, "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyProjectDescription)

	// Check we can list the projects, and our new project is in it:
	output = s.runCommand(s.listProjects(), ExpectNoError)
	s.Contains(output.StdOut, projectName)

	// Now get, verify, and delete the project:
	fmt.Println("Testing project get command")
	output = s.runCommand(s.getProject(projectName), ExpectNoError)
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, *project.Name)
	s.Empty(output.StdErr)

	// Attempt to get project by id:
	output = s.runCommand(s.getProject((*project.ProjectID).String()), ExpectNoError)
	var project2 api.Project
	err = json.Unmarshal([]byte(output.StdOut), &project2)
	s.NoError(err)
	s.Equal(projectName, *project.Name)
	s.Empty(output.StdErr)
	// Attempt to get a project with empty name and id:
	output = s.runCommand(s.getProject(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Non-existent project:
	output = s.runCommand(s.getProject(uuid.Nil.String()), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Blank name:
	output = s.runCommand(s.getProject(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)

	// Validate that using the id as another project name throws an error.
	output = s.runCommand(s.createProject(project.ProjectID.String(), "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, ProjectNameCollision)

	fmt.Println("Testing project delete command")
	output = s.runCommand(s.deleteProject(projectName), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
	// Verify that attempting to re-delete will fail:
	output = s.runCommand(s.deleteProject(projectName), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Verify that a valid project ID is needed:
	output = s.runCommand(s.deleteProject(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
}

func (s *EndToEndTestSuite) TestProjectCreateGithub() {
	fmt.Println("Testing project create command, with --github flag")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// Now get, verify, and delete the project:
	output = s.runCommand(s.getProject(projectIDString), ExpectNoError)
	var project api.Project
	err := json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, *project.Name)
	s.Equal(projectID, *project.ProjectID)
	s.Empty(output.StdErr)
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

// Test branch creation:
func (s *EndToEndTestSuite) TestBranchCreate() {
	fmt.Println("Testing branch creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(projectID, branchName, "RELEASE", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedBranch)
	// Validate that  missing name, project, or type returns errors:
	output = s.runCommand(s.createBranch(projectID, "", "RELEASE", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBranchName)
	output = s.runCommand(s.createBranch(uuid.Nil, branchName, "RELEASE", GithubFalse), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(s.createBranch(projectID, branchName, "INVALID", GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBranchType)

	// Check we can list the branches, and our new branch is in it:
	output = s.runCommand(s.listBranches(projectID), ExpectNoError)
	s.Contains(output.StdOut, branchName)

	// Delete the test project
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestBranchCreateGithub() {
	fmt.Println("Testing branch creation, with --github flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Delete the test project
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

// Test the build creation:
func (s *EndToEndTestSuite) TestBuildCreate() {
	fmt.Println("Testing build creation")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)

	// Check we can list the builds, and our new build is in it:
	output = s.runCommand(s.listBuilds(projectID, branchName), ExpectNoError)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Check we can list the builds by passing in the branchID, and our new build is in it:
	output = s.runCommand(s.listBuilds(projectID, branchID.String()), ExpectNoError)
	s.Contains(output.StdOut, branchIDString)
	s.Contains(output.StdOut, buildIDString)

	// Verify that each of the required flags are required:
	output = s.runCommand(s.createBuild(projectName, branchName, "", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildDescription)
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildImage)
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildVersion)
	output = s.runCommand(s.createBuild("", branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(s.createBuild(projectName, "", "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, BranchNotExist)
	// Validate the image URI is required to be valid and have a tag:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBuildImage)
	// Delete the project:
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible

}

func (s *EndToEndTestSuite) TestBuildCreateGithub() {
	fmt.Println("Testing build creation, with --github flag")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible

	// Delete the project:
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestBuildCreateAutoCreateBranch() {
	fmt.Println("Testing build creation with the auto-create-branch flag")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build: (with auto-create-branch flag). We expect this to succeed without any additional information
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, CreatedBuild)
	s.NotContains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", branchName))

	// Now try to create a build with a new branch name:
	newBranchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBuild(projectName, newBranchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.1", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, CreatedBuild)
	s.Contains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", newBranchName))
	s.Contains(output.StdOut, CreatedBranch)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible

	// Delete the project:
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestExperienceCreate() {
	fmt.Println("Testing experience creation command")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(s.createExperience(projectID, experienceName, "description", "location", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedExperience)
	s.Empty(output.StdErr)
	// Validate we cannot create experiences without values for the required flags:
	output = s.runCommand(s.createExperience(projectID, "", "description", "location", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceName)
	output = s.runCommand(s.createExperience(projectID, experienceName, "", "location", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceDescription)
	output = s.runCommand(s.createExperience(projectID, experienceName, "description", "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceLocation)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible
}

func (s *EndToEndTestSuite) TestExperienceCreateGithub() {
	fmt.Println("Testing experience creation command, with --github flag")
	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(s.createExperience(projectID, experienceName, "description", "location", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible

}

func (s *EndToEndTestSuite) TestBatchAndLogs() {
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// This test does not use parameters, so we create an empty parameter map:
	emptyParameterMap := map[string]string{}
	// First create two experiences:
	experienceName1 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(s.createExperience(projectID, experienceName1, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(s.createExperience(projectID, experienceName2, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible

	// Create a metrics build:
	output = s.runCommand(s.createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create an experience tag:
	tagName := fmt.Sprintf("test-tag-%s", uuid.New().String())
	output = s.runCommand(s.createExperienceTag(projectID, tagName, "testing tag"), ExpectNoError)

	// List experience tags and check the list contains the tag we just created
	output = s.runCommand(s.listExperienceTags(projectID), ExpectNoError)
	s.Contains(output.StdOut, tagName)

	// Tag one of the experiences:
	output = s.runCommand(s.tagExperience(projectID, tagName, experienceID1), ExpectNoError)

	// Adding the same tag again should error:
	output = s.runCommand(s.tagExperience(projectID, tagName, experienceID1), ExpectError)

	// List experiences for the tag
	output = s.runCommand(s.listExperiencesWithTag(projectID, tagName), ExpectNoError)
	var tagExperiences []api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(1, len(tagExperiences))

	// Tag 2nd, check list contains 2 experiences
	output = s.runCommand(s.tagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(s.listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(2, len(tagExperiences))

	// Untag and check list again
	output = s.runCommand(s.untagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(s.listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(1, len(tagExperiences))

	// Fail to list batch logs without a batch specifier
	output = s.runCommand(s.listBatchLogs(projectID, "", ""), ExpectError)
	s.Contains(output.StdErr, SelectOneRequired)

	// Fail to list batch logs with a bad UUID
	output = s.runCommand(s.listBatchLogs(projectID, "not-a-uuid", ""), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Fail to list batch logs with a made up batch name
	output = s.runCommand(s.listBatchLogs(projectID, "", "not-a-valid-batch-name"), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)

	// Fail to create a batch without any experience ids, tags, or names
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, emptyParameterMap), ExpectError)
	s.Contains(output.StdErr, SelectOneRequired)

	// Create a batch with (only) experience names using the --experiences flag
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName1, experienceName2}, []string{}, "", GithubTrue, emptyParameterMap), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH)

	// Create a batch with mixed experience names and IDs in the --experiences flag
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName1, experienceIDString2}, []string{}, "", GithubTrue, emptyParameterMap), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH)

	// Create a batch with an ID in the --experiences flag and a tag name in the --experience-tags flag (experience 1 is in the tag)
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceIDString2}, []string{tagName}, "", GithubTrue, emptyParameterMap), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH)

	// Create a batch without metrics with the github flag set and check the output
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, "", GithubTrue, emptyParameterMap), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH = output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH)

	// Now create a batch without the github flag, but with metrics
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, metricsBuildIDString, GithubFalse, emptyParameterMap), ExpectNoError)
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
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap), ExpectError)
	s.Contains(output.StdErr, SelectOneRequired)
	// Try a batch without a build id:
	output = s.runCommand(s.createBatch(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, []string{}, []string{}, "", GithubFalse, emptyParameterMap), ExpectError)
	s.Contains(output.StdErr, InvalidBuildID)
	// Try a batch with both experience tag ids and experience tag names (even if fake):
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{"tag-id"}, []string{"tag-name"}, []string{}, []string{}, "", GithubFalse, emptyParameterMap), ExpectError)
	s.Contains(output.StdErr, BranchTagMutuallyExclusive)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
		cmd := s.buildCommand(s.getBatchByName(projectID, batchNameString, ExitStatusTrue))
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
	}, 5*time.Minute, 10*time.Second)
	// Grab the batch and validate the status, first by name then by ID:
	output = s.runCommand(s.getBatchByName(projectID, batchNameString, ExitStatusFalse), ExpectNoError)
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
	output = s.runCommand(s.getBatchByID(projectID, batchIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchNameString, *batch.FriendlyName)
	s.Equal(batchID, *batch.BatchID)
	// Validate that it succeeded:
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)

	// List the logs for the succeeded batch
	output = s.runCommand(s.listBatchLogs(projectID, "", batchNameString), ExpectNoError)
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
	output = s.runCommand(s.getBatchByName(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, RequireBatchName)
	output = s.runCommand(s.getBatchByID(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, RequireBatchName)

	// Pass unknown name / id to batches jobs:
	output = s.runCommand(s.getBatchByName(projectID, "does not exist", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)
	output = s.runCommand(s.getBatchByID(projectID, "0000-0000-0000-0000-000000000000", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Now grab the jobs from the batch:
	output = s.runCommand(s.getBatchJobsByName(projectID, batchNameString), ExpectNoError)
	// Marshal into a struct:
	var jobs []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(2, len(jobs))
	for _, job := range jobs {
		s.Contains([]uuid.UUID{experienceID1, experienceID2}, *job.ExperienceID)
		s.Equal(buildID, *job.BuildID)
	}
	output = s.runCommand(s.getBatchJobsByID(projectID, batchIDString), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(2, len(jobs))
	for _, job := range jobs {
		s.Contains([]uuid.UUID{experienceID1, experienceID2}, *job.ExperienceID)
		s.Equal(buildID, *job.BuildID)
	}

	jobID1 := *jobs[0].JobID
	jobID2 := *jobs[1].JobID
	// Pass blank name / id to batches jobs:
	output = s.runCommand(s.getBatchJobsByName(projectID, ""), ExpectError)
	s.Contains(output.StdErr, RequireBatchName)
	output = s.runCommand(s.getBatchJobsByID(projectID, ""), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Pass unknown name / id to batches jobs:
	output = s.runCommand(s.getBatchJobsByName(projectID, "does not exist"), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)

	// Finally, create logs
	logName := fmt.Sprintf("test-log-%s", uuid.New().String())
	output = s.runCommand(s.createLog(projectID, batchID, jobID1, logName, "100", "checksum", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedLog)
	// Validate that all required flags are required:
	output = s.runCommand(s.createLog(projectID, uuid.Nil, jobID1, logName, "100", "checksum", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogBatchID)
	output = s.runCommand(s.createLog(projectID, batchID, uuid.Nil, logName, "100", "checksum", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogJobID)
	output = s.runCommand(s.createLog(projectID, batchID, jobID1, "", "100", "checksum", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogFileName)

	output = s.runCommand(s.createLog(projectID, batchID, jobID1, logName, "100", "checksum", "", string(api.EXPERIENCE), GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogType)
	output = s.runCommand(s.createLog(projectID, batchID, jobID1, logName, "100", "checksum", string(api.ARCHIVELOG), "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogExecutionStep)

	// TODO(iainjwhiteside): we can't check the empty file size easily in this framework

	// Checksum is actually optional, but warned about:
	output = s.runCommand(s.createLog(projectID, batchID, jobID1, logName, "100", "", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, EmptyLogChecksum)

	// Create w/ the github flag:
	logName = fmt.Sprintf("test-log-%s", uuid.New().String())
	output = s.runCommand(s.createLog(projectID, batchID, jobID2, logName, "100", "checksum", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedLog)
	log1Location := output.StdOut[len(GithubCreatedLog) : len(output.StdOut)-1]
	s.Contains(log1Location, "s3://")
	// Create a second log to test parsing:
	logName2 := fmt.Sprintf("test-log-%s", uuid.New().String())
	output = s.runCommand(s.createLog(projectID, batchID, jobID2, logName2, "100", "checksum", string(api.ARCHIVELOG), string(api.EXPERIENCE), GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedLog)
	log2Location := output.StdOut[len(GithubCreatedLog) : len(output.StdOut)-1]
	s.Contains(log2Location, "s3://")

	// List logs:
	output = s.runCommand(s.listLogs(projectID, batchIDString, jobID2.String()), ExpectNoError)
	// Marshal into a struct:
	var logs []api.JobLog
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	s.NoError(err)
	s.Len(logs, 6)
	for _, log := range logs {
		s.Equal(jobID2, *log.JobID)
		s.Contains([]string{logName, logName2, "experience-worker.log", "metrics-worker.log", "experience-container.log", "metrics-container.log"}, *log.FileName)
	}

	// Pass blank name / id to logs:
	output = s.runCommand(s.listLogs(projectID, "not-a-uuid", jobID2.String()), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)
	output = s.runCommand(s.listLogs(projectID, batchIDString, "not-a-uuid"), ExpectError)
	s.Contains(output.StdErr, InvalidJobID)

	// Delete the project:
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestParameterizedBatch() {
	// create a project:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
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
	output = s.runCommand(s.createExperience(projectID, experienceName, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible

	// Create a metrics build:
	output = s.runCommand(s.createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	metricsBuildID := uuid.MustParse(metricsBuildIDString)

	// Create a batch with (only) experience names using the --experiences flag with some parameters
	output = s.runCommand(s.createBatch(projectID, buildIDString, []string{}, []string{}, []string{}, []string{experienceName}, []string{}, metricsBuildIDString, GithubTrue, expectedParameterMap), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	batchID := uuid.MustParse(batchIDStringGH)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
		cmd := s.buildCommand(s.getBatchByID(projectID, batchIDStringGH, ExitStatusTrue))
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
	}, 5*time.Minute, 10*time.Second)
	// Grab the batch and validate the status by ID:
	output = s.runCommand(s.getBatchByID(projectID, batchIDStringGH, ExitStatusFalse), ExpectNoError)
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
	s.Equal([]uuid.UUID{experienceID1}, *batch.InstantiatedExperienceIDs)
	s.Empty(batch.InstantiatedExperienceTagIDs)

	// Delete the project
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}
func (s *EndToEndTestSuite) TestCreateSweepParameterNameAndValues() {
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)
	// create two experiences:
	experienceName1 := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output = s.runCommand(s.createExperience(projectID, experienceName1, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("sweep-test-experience-%s", uuid.New().String())
	output = s.runCommand(s.createExperience(projectID, experienceName2, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible

	// Now create the branch:
	branchName := fmt.Sprintf("sweep-test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible

	// Create a metrics build:
	output = s.runCommand(s.createMetricsBuild(projectID, "test-metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "version", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedMetricsBuild)
	// We expect to be able to parse the build ID as a UUID
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Create an experience tag:
	tagName := fmt.Sprintf("test-tag-%s", uuid.New().String())
	output = s.runCommand(s.createExperienceTag(projectID, tagName, "testing tag"), ExpectNoError)

	// List experience tags and check the list contains the tag we just created
	output = s.runCommand(s.listExperienceTags(projectID), ExpectNoError)
	s.Contains(output.StdOut, tagName)

	// Tag one of the experiences:
	output = s.runCommand(s.tagExperience(projectID, tagName, experienceID1), ExpectNoError)

	// Adding the same tag again should error:
	output = s.runCommand(s.tagExperience(projectID, tagName, experienceID1), ExpectError)

	// List experiences for the tag
	output = s.runCommand(s.listExperiencesWithTag(projectID, tagName), ExpectNoError)
	var tagExperiences []api.Experience
	err := json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(1, len(tagExperiences))

	// Tag 2nd, check list contains 2 experiences
	output = s.runCommand(s.tagExperience(projectID, tagName, experienceID2), ExpectNoError)
	output = s.runCommand(s.listExperiencesWithTag(projectID, tagName), ExpectNoError)
	err = json.Unmarshal([]byte(output.StdOut), &tagExperiences)
	s.NoError(err)
	s.Equal(2, len(tagExperiences))

	// Define the parameters:
	parameterName := "test-parameter"
	parameterValues := []string{"value1", "value2", "value3"}
	// Create a sweep with (only) experience names using the --experiences flag and specific parameter name and values (and "" for no config file location)
	output = s.runCommand(s.createSweep(projectID, buildIDString, []string{experienceName1, experienceName2}, []string{}, metricsBuildIDString, parameterName, parameterValues, "", GithubTrue), ExpectNoError)
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
	output = s.runCommand(s.createSweep(projectID, buildIDString, []string{}, []string{tagName}, "", "", []string{}, configLocation, GithubFalse), ExpectNoError)
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
	output = s.runCommand(s.createSweep(projectID, buildIDString, []string{}, []string{}, "", parameterName, parameterValues, "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, FailedToCreateSweep)
	// Try a sweep without a build id:
	output = s.runCommand(s.createSweep(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, "", parameterName, parameterValues, "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBuildID)

	// Try a sweep with both parameter name and config (even if fake):
	output = s.runCommand(s.createSweep(projectID, "", []string{experienceIDString1, experienceIDString2}, []string{}, "", parameterName, parameterValues, "config location", GithubFalse), ExpectError)
	s.Contains(output.StdErr, ConfigParamsMutuallyExclusive)

	// Try a sweep with an invalid config
	invalidConfigLocation := fmt.Sprintf("%s/data/invalid_sweep_config.json", cwd)
	output = s.runCommand(s.createSweep(projectID, buildIDString, []string{}, []string{tagName}, "", "", []string{}, invalidConfigLocation, GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidGridSearchFile)

	// Get sweep passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the sweep, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
		cmd := s.buildCommand(s.getSweepByName(projectID, sweepNameString, ExitStatusTrue))
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
	output = s.runCommand(s.getSweepByName(projectID, sweepNameString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var sweep api.ParameterSweep
	err = json.Unmarshal([]byte(output.StdOut), &sweep)
	s.NoError(err)
	s.Equal(sweepNameString, *sweep.Name)
	s.Equal(secondSweepID, *sweep.ParameterSweepID)
	// Validate that it succeeded:
	s.Equal(api.SUCCEEDED, *sweep.Status)
	// Get the sweep by ID:
	output = s.runCommand(s.getSweepByID(projectID, secondSweepIDString, ExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &sweep)
	s.NoError(err)
	s.Equal(sweepNameString, *sweep.Name)
	s.Equal(secondSweepID, *sweep.ParameterSweepID)
	// Validate that it succeeded:
	s.Equal(api.SUCCEEDED, *sweep.Status)

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
	output = s.runCommand(s.getSweepByName(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidSweepName)
	output = s.runCommand(s.getSweepByID(projectID, "", ExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidSweepID)

	// Check we can list the sweeps, and our new sweep is in it:
	output = s.runCommand(s.listSweeps(projectID), ExpectNoError)
	s.Contains(output.StdOut, sweepNameString)
}

// Test the metrics builds:
func (s *EndToEndTestSuite) TestCreateMetricsBuild() {
	fmt.Println("Testing metrics build creation")
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	output = s.runCommand(s.createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedMetricsBuild)
	s.Empty(output.StdErr)
	// Verify that each of the required flags are required:
	output = s.runCommand(s.createMetricsBuild(projectID, "", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildName)
	output = s.runCommand(s.createMetricsBuild(projectID, "name", "", "1.0.0", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildImage)
	output = s.runCommand(s.createMetricsBuild(projectID, "name", "public.ecr.aws/docker/library/hello-world:latest", "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyMetricsBuildVersion)
	output = s.runCommand(s.createMetricsBuild(projectID, "name", "public.ecr.aws/docker/library/hello-world", "1.1.1", GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidMetricsBuildImage)

}

func (s *EndToEndTestSuite) TestMetricsBuildGithub() {
	fmt.Println("Testing metrics build creation, with --github flag")
	// create a project:
	projectName := fmt.Sprintf("sweep-test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	output = s.runCommand(s.createMetricsBuild(projectID, "metrics-build", "public.ecr.aws/docker/library/hello-world:latest", "1.0.0", GithubTrue), ExpectNoError)
	metricsBuildIDString := output.StdOut[len(GithubCreatedMetricsBuild) : len(output.StdOut)-1]
	uuid.MustParse(metricsBuildIDString)

	// Check we can list the metrics builds, and our new metrics build is in it:
	output = s.runCommand(s.listMetricsBuilds(projectID), ExpectNoError)
	s.Contains(output.StdOut, metricsBuildIDString)
}

func (s *EndToEndTestSuite) TestAliases() {
	fmt.Println("Testing project and branch aliases")
	// First create a project, manually:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
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
	s.Equal(projectName, *project.Name)
	s.Equal(projectID, *project.ProjectID)
	// Try with the ID:
	output = s.runCommand([]CommandBuilder{projectCommand, getByIDCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &project)
	s.NoError(err)
	s.Equal(projectName, *project.Name)
	s.Equal(projectID, *project.ProjectID)

	// Now create a branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(projectID, branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Empty(output.StdErr)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	branchID := uuid.MustParse(branchIDString)

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
	s.Equal(branchName, *branches[0].Name)
	s.Equal(branchID, *branches[0].BranchID)
	s.Equal(projectID, *branches[0].ProjectID)
	// Now try by ID:
	output = s.runCommand([]CommandBuilder{branchCommand, listBranchesByIDCommand}, ExpectNoError)
	s.Empty(output.StdErr)
	err = json.Unmarshal([]byte(output.StdOut), &branches)
	s.NoError(err)
	s.Equal(1, len(branches))
	s.Equal(branchName, *branches[0].Name)
	s.Equal(branchID, *branches[0].BranchID)
	s.Equal(projectID, *branches[0].ProjectID)

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
	s.Empty(output.StdErr)
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

	// Delete the project, using the aliased command:
	deleteProjectCommand := CommandBuilder{
		Command: "project",
	}
	deleteProjectByIDCommand := CommandBuilder{
		Command: "delete",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID.String(),
			},
		},
	}
	output = s.runCommand([]CommandBuilder{deleteProjectCommand, deleteProjectByIDCommand}, ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)

	// Finally, create a new project to verify deletion with the old 'name' flag:
	projectName = fmt.Sprintf("test-project-%s", uuid.New().String())
	output = s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Empty(output.StdErr)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString = output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	uuid.MustParse(projectIDString)
	// Delete the project, using the aliased command:
	deleteProjectByNameCommand := CommandBuilder{
		Command: "delete",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
		},
	}
	output = s.runCommand([]CommandBuilder{deleteProjectCommand, deleteProjectByNameCommand}, ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
}

func TestEndToEndTestSuite(t *testing.T) {
	viper.AutomaticEnv()
	viper.SetDefault(Config, Dev)
	suite.Run(t, new(EndToEndTestSuite))
}
