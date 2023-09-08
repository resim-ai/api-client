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
	BatchExitStatusTrue   bool = true
	BatchExitStatusFalse  bool = false
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
	EmptyBuildVersion     string = "empty build version"
	BranchNotExist        string = "Branch does not exist"
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
	// Log Messages
	CreatedLog       string = "Created log"
	GithubCreatedLog string = "log_location="
	EmptyLogFileName string = "empty log file name"
	EmptyLogChecksum string = "No checksum was provided"
	EmptyLogBatchID  string = "empty batch ID"
	EmptyLogJobID    string = "empty job ID"
	InvalidJobID     string = "unable to parse job ID"
)

var AcceptableBatchStatusCodes = [...]int{0, 2, 3, 4, 5}

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

func (s *EndToEndTestSuite) createExperience(name string, description string, location string, github bool) []CommandBuilder {
	// We build a create experience command with the name, description, location flags
	experienceCommand := CommandBuilder{
		Command: "experiences",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
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

func (s *EndToEndTestSuite) createBatch(buildID string, experienceIDs []string, experienceTagIDs []string, experienceTagNames []string, github bool) []CommandBuilder {
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
	if github {
		createCommand.Flags = append(createCommand.Flags, Flag{
			Name:  "--github",
			Value: "",
		})
	}
	return []CommandBuilder{batchCommand, createCommand}
}

func (s *EndToEndTestSuite) getBatchByName(batchName string, exitStatus bool) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
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

func (s *EndToEndTestSuite) getBatchByID(batchID string, exitStatus bool) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
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

func (s *EndToEndTestSuite) getBatchJobsByName(batchName string) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "jobs",
		Flags: []Flag{
			{
				Name:  "--batch-name",
				Value: batchName,
			},
		},
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func (s *EndToEndTestSuite) getBatchJobsByID(batchID string) []CommandBuilder {
	// We build a get batch command with the name flag
	batchCommand := CommandBuilder{
		Command: "batches",
	}
	getCommand := CommandBuilder{
		Command: "jobs",
		Flags: []Flag{
			{
				Name:  "--batch-id",
				Value: batchID,
			},
		},
	}
	return []CommandBuilder{batchCommand, getCommand}
}

func (s *EndToEndTestSuite) createLog(batchID uuid.UUID, jobID uuid.UUID, name string, fileSize string, checksum string, github bool) []CommandBuilder {
	logCommand := CommandBuilder{
		Command: "logs",
	}
	createCommand := CommandBuilder{
		Command: "create",
		Flags: []Flag{
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

func (s *EndToEndTestSuite) listLogs(batchID string, jobID string) []CommandBuilder {
	logCommand := CommandBuilder{
		Command: "log",
	}
	listCommand := CommandBuilder{
		Command: "list",
		Flags: []Flag{
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

	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
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
	output = s.runCommand(s.createBuild(projectName, branchName, "", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildDescription)
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildImage)
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, EmptyBuildVersion)
	output = s.runCommand(s.createBuild("", branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	output = s.runCommand(s.createBuild(projectName, "", "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectError)
	s.Contains(output.StdErr, BranchNotExist)
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
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
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
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
	s.Contains(output.StdOut, CreatedBuild)
	s.NotContains(output.StdOut, fmt.Sprintf("Branch with name %v doesn't currently exist.", branchName))

	// Now try to create a build with a new branch name:
	newBranchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBuild(projectName, newBranchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.1", GithubFalse, AutoCreateBranchTrue), ExpectNoError)
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
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output := s.runCommand(s.createExperience(experienceName, "description", "location", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedExperience)
	s.Empty(output.StdErr)
	// Validate we cannot create experiences without values for the required flags:
	output = s.runCommand(s.createExperience("", "description", "location", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceName)
	output = s.runCommand(s.createExperience(experienceName, "", "location", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceDescription)
	output = s.runCommand(s.createExperience(experienceName, "description", "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyExperienceLocation)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible
}

func (s *EndToEndTestSuite) TestExperienceCreateGithub() {
	fmt.Println("Testing experience creation command, with --github flag")
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output := s.runCommand(s.createExperience(experienceName, "description", "location", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible

}

func (s *EndToEndTestSuite) TestBatchAndLogs() {
	// First create two experiences:
	experienceName1 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	experienceLocation := fmt.Sprintf("s3://%s/experiences/%s/", s.Config.E2EBucket, uuid.New())
	output := s.runCommand(s.createExperience(experienceName1, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString1 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID1 := uuid.MustParse(experienceIDString1)

	experienceName2 := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output = s.runCommand(s.createExperience(experienceName2, "description", experienceLocation, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	s.Empty(output.StdErr)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString2 := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	experienceID2 := uuid.MustParse(experienceIDString2)
	//TODO(https://app.asana.com/0/1205272835002601/1205376807361744/f): Delete the experiences when possible

	// Then create a project, branch, build:
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output = s.runCommand(s.createProject(projectName, "description", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedProject)
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len(GithubCreatedProject) : len(output.StdOut)-1]
	uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.runCommand(s.createBranch(uuid.MustParse(projectIDString), branchName, "RELEASE", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBranch)
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len(GithubCreatedBranch) : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubTrue, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBuild)
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len(GithubCreatedBuild) : len(output.StdOut)-1]
	buildID := uuid.MustParse(buildIDString)
	// TODO(https://app.asana.com/0/1205272835002601/1205376807361747/f): Delete builds when possible
	// Create a batch with the github flag set and check the output
	output = s.runCommand(s.createBatch(buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedBatch)
	batchIDStringGH := output.StdOut[len(GithubCreatedBatch) : len(output.StdOut)-1]
	uuid.MustParse(batchIDStringGH)

	// Now create a batch:
	output = s.runCommand(s.createBatch(buildIDString, []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, GithubFalse), ExpectNoError)
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
	output = s.runCommand(s.createBatch(buildIDString, []string{}, []string{}, []string{}, GithubFalse), ExpectError)
	s.Contains(output.StdErr, FailedToCreateBatch)
	// Try a batch without a build id:
	output = s.runCommand(s.createBatch("", []string{experienceIDString1, experienceIDString2}, []string{}, []string{}, GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBuildID)
	// Try a batch with both experience tag ids and experience tag names (even if fake):
	output = s.runCommand(s.createBatch(buildIDString, []string{}, []string{"tag-id"}, []string{"tag-name"}, GithubFalse), ExpectError)
	s.Contains(output.StdErr, BranchTagMutuallyExclusive)

	// Get batch passing the status flag. We need to manually execute and grab the exit code:
	// Since we have just submitted the batch, we would expect it to be running or submitted
	// but we check that the exit code is in the acceptable range:
	s.Eventually(func() bool {
		cmd := s.buildCommand(s.getBatchByName(batchNameString, BatchExitStatusTrue))
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
	output = s.runCommand(s.getBatchByName(batchNameString, BatchExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	var batch api.Batch
	err := json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchNameString, *batch.FriendlyName)
	s.Equal(batchID, *batch.BatchID)
	// Validate that it succeeded:
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
	// Get the batch by ID:
	output = s.runCommand(s.getBatchByID(batchIDString, BatchExitStatusFalse), ExpectNoError)
	// Marshal into a struct:
	err = json.Unmarshal([]byte(output.StdOut), &batch)
	s.NoError(err)
	s.Equal(batchNameString, *batch.FriendlyName)
	s.Equal(batchID, *batch.BatchID)
	// Validate that it succeeded:
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)

	// Pass blank name / id to batches get:
	output = s.runCommand(s.getBatchByName("", BatchExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)
	output = s.runCommand(s.getBatchByID("", BatchExitStatusFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)
	// Now grab the jobs from the batch:
	output = s.runCommand(s.getBatchJobsByName(batchNameString), ExpectNoError)
	// Marshal into a struct:
	var jobs []api.Job
	err = json.Unmarshal([]byte(output.StdOut), &jobs)
	s.NoError(err)
	s.Equal(2, len(jobs))
	for _, job := range jobs {
		s.Contains([]uuid.UUID{experienceID1, experienceID2}, *job.ExperienceID)
		s.Equal(buildID, *job.BuildID)
	}
	output = s.runCommand(s.getBatchJobsByID(batchIDString), ExpectNoError)
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
	output = s.runCommand(s.getBatchJobsByName(""), ExpectError)
	s.Contains(output.StdErr, InvalidBatchName)
	output = s.runCommand(s.getBatchJobsByID(""), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)

	// Finally, create logs
	logName := fmt.Sprintf("test-log-%s", uuid.New().String())
	output = s.runCommand(s.createLog(batchID, jobID1, logName, "100", "checksum", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedLog)
	// Validate that all required flags are required:
	output = s.runCommand(s.createLog(uuid.Nil, jobID1, logName, "100", "checksum", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogBatchID)
	output = s.runCommand(s.createLog(batchID, uuid.Nil, logName, "100", "checksum", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogJobID)
	output = s.runCommand(s.createLog(batchID, jobID1, "", "100", "checksum", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyLogFileName)

	// TODO(iainjwhiteside): we can't check the empty file size easily in this framework

	// Checksum is actually optional, but warned about:
	output = s.runCommand(s.createLog(batchID, jobID1, logName, "100", "", GithubFalse), ExpectNoError)
	s.Contains(output.StdOut, EmptyLogChecksum)

	// Create w/ the github flag:
	logName = fmt.Sprintf("test-log-%s", uuid.New().String())
	output = s.runCommand(s.createLog(batchID, jobID2, logName, "100", "checksum", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedLog)
	log1Location := output.StdOut[len(GithubCreatedLog) : len(output.StdOut)-1]
	s.Contains(log1Location, "s3://")
	// Create a second log to test parsing:
	logName2 := fmt.Sprintf("test-log-%s", uuid.New().String())
	output = s.runCommand(s.createLog(batchID, jobID2, logName2, "100", "checksum", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedLog)
	log2Location := output.StdOut[len(GithubCreatedLog) : len(output.StdOut)-1]
	s.Contains(log2Location, "s3://")

	// List logs:
	output = s.runCommand(s.listLogs(batchIDString, jobID2.String()), ExpectNoError)
	// Marshal into a struct:
	var logs []api.Log
	err = json.Unmarshal([]byte(output.StdOut), &logs)
	s.NoError(err)
	s.Len(logs, 3)
	for _, log := range logs {
		s.Equal(jobID2, *log.JobID)
		s.Contains([]string{logName, logName2, "container.log"}, *log.FileName)
	}

	// Pass blank name / id to logs:
	output = s.runCommand(s.listLogs("not-a-uuid", jobID2.String()), ExpectError)
	s.Contains(output.StdErr, InvalidBatchID)
	output = s.runCommand(s.listLogs(batchIDString, "not-a-uuid"), ExpectError)
	s.Contains(output.StdErr, InvalidJobID)

	// Delete the project:
	output = s.runCommand(s.deleteProject(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
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
				Value: "image",
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
				Value: "image",
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
