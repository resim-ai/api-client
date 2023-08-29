//go:build end_to_end
// +build end_to_end

// The end-to-end test is meant to exercise every potential command of the CLI in its built form
// It's meant as a sanity check on deployed code.
// It is not intended to catch edge cases or weird interactions; that's the realm of unit testing.

// To run:
//    go test -v -tags end_to_end ./testing

package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
}

var DevConfig = CliConfig{
	AuthKeyProviderDomain: "https://resim-dev.us.auth0.com/",
	ApiEndpoint:           "https://$DEPLOYMENT.api.dev.resim.io/v1/",
}

var StagingConfig = CliConfig{
	AuthKeyProviderDomain: "https://resim-dev.us.auth0.com/",
	ApiEndpoint:           "https://api.resim.io/v1/",
}

var ProdConfig = CliConfig{
	AuthKeyProviderDomain: "https://resim.us.auth0.com/",
	ApiEndpoint:           "https://api.resim.ai/v1/",
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
	InvalidProjectID        string = "unable to parse project ID"
	FailedToFindProject     string = "failed to find project"
	DeletedProject          string = "Deleted project"
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
)

func (s *EndToEndTestSuite) TearDownSuite() {
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
	case Staging:
		s.Config = StagingConfig
	case Prod:
		s.Config = ProdConfig
	default:
		fmt.Fprintf(os.Stderr, "error: invalid value for %s: %s", Config, viper.GetString(Config))
		os.Exit(1)
	}

	// Validate the client credential environment variables are set:
	if !viper.IsSet(ClientID) {
		fmt.Fprintf(os.Stderr, "error: %v environment variable must be set", ClientID)
		os.Exit(1)
	}
	if !viper.IsSet(ClientSecret) {
		fmt.Fprintf(os.Stderr, "error: %v environment variable must be set", ClientSecret)
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
	return outputPath
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
	return exec.Command(s.CliPath, allCommands...)
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
		Command: "project",
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

func (s *EndToEndTestSuite) getProjectByName(projectName string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "project",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--name",
				Value: projectName,
			},
		},
	}
	return []CommandBuilder{projectCommand, getCommand}
}

func (s *EndToEndTestSuite) deleteProjectByName(projectName string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "project",
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

func (s *EndToEndTestSuite) getProjectByID(projectID string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "project",
	}
	getCommand := CommandBuilder{
		Command: "get",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID,
			},
		},
	}
	return []CommandBuilder{projectCommand, getCommand}
}

func (s *EndToEndTestSuite) deleteProjectByID(projectID string) []CommandBuilder {
	// We build a get project command with the name flag
	projectCommand := CommandBuilder{
		Command: "project",
	}
	deleteCommand := CommandBuilder{
		Command: "delete",
		Flags: []Flag{
			{
				Name:  "--project-id",
				Value: projectID,
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
				Name:  "--project-id",
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

func (s *EndToEndTestSuite) createBuild(projectName string, branchName string, description string, image string, version string, github bool, autoCreateBranch bool) []CommandBuilder {
	// Now create the build:
	buildCommand := CommandBuilder{
		Command: "builds",
	}
	createCommand := CommandBuilder{
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

// As a first test, we expect the help command to run successfully
func (s *EndToEndTestSuite) TestHelp() {
	fmt.Println("Testing help command")
	runCommand := CommandBuilder{
		Command: "help",
	}
	output := s.runCommand([]CommandBuilder{runCommand}, ExpectNoError)
	s.Contains(output.StdOut, "Usage:")
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
	s.Contains(output.StdErr, FailedToCreateProject)
	// Validate that omitting the name leads to an error:
	output = s.runCommand(s.createProject("", "description", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyProjectName)
	// Validate that omitting the description leads to an error:
	output = s.runCommand(s.createProject(projectName, "", GithubFalse), ExpectError)
	s.Contains(output.StdErr, EmptyProjectDescription)

	// Now get, verify, and delete the project:
	fmt.Println("Testing project get command")
	output = s.runCommand(s.getProjectByName(projectName), ExpectNoError)
	var project api.Project
	json.Unmarshal([]byte(output.StdOut), &project)
	s.Equal(projectName, *project.Name)
	s.Empty(output.StdErr)

	// Attempt to get project by id:
	output = s.runCommand(s.getProjectByID((*project.ProjectID).String()), ExpectNoError)
	var project2 api.Project
	json.Unmarshal([]byte(output.StdOut), &project2)
	s.Equal(projectName, *project.Name)
	s.Empty(output.StdErr)
	// Attempt to get a project with empty name and id:
	output = s.runCommand(s.getProjectByID(""), ExpectError)
	s.Contains(output.StdErr, InvalidProjectID)
	// Non-existentt project:
	output = s.runCommand(s.getProjectByID(uuid.Nil.String()), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Blank name:
	output = s.runCommand(s.getProjectByName(""), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)

	fmt.Println("Testing project delete command")
	output = s.runCommand(s.deleteProjectByName(projectName), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
	// Verify that attempting to re-delete will fail:
	output = s.runCommand(s.deleteProjectByName(projectName), ExpectError)
	s.Contains(output.StdErr, FailedToFindProject)
	// Verify that a valid project ID is needed:
	output = s.runCommand(s.deleteProjectByID(""), ExpectError)
	s.Contains(output.StdErr, InvalidProjectID)
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
	output = s.runCommand(s.getProjectByID(projectIDString), ExpectNoError)
	var project api.Project
	json.Unmarshal([]byte(output.StdOut), &project)
	s.Equal(projectName, *project.Name)
	s.Equal(projectID, *project.ProjectID)
	s.Empty(output.StdErr)
	output = s.runCommand(s.deleteProjectByID(projectIDString), ExpectNoError)
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
	s.Contains(output.StdErr, EmptyProjectID)
	output = s.runCommand(s.createBranch(projectID, branchName, "INVALID", GithubFalse), ExpectError)
	s.Contains(output.StdErr, InvalidBranchType)

	// Delete the test project
	output = s.runCommand(s.deleteProjectByID(projectIDString), ExpectNoError)
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
	output = s.runCommand(s.deleteProjectByID(projectIDString), ExpectNoError)
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
	uuid.MustParse(branchIDString)

	// Now create the build:

	output = s.runCommand(s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", GithubFalse, AutoCreateBranchFalse), ExpectNoError)
	s.Contains(output.StdOut, CreatedBuild)
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
	output = s.runCommand(s.deleteProjectByID(projectIDString), ExpectNoError)
	s.Contains(output.StdOut, DeletedProject)
	s.Empty(output.StdErr)
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
}

func (s *EndToEndTestSuite) TestExperienceCreateGithub() {
	fmt.Println("Testing experience creation command, with --github flag")
	experienceName := fmt.Sprintf("test-experience-%s", uuid.New().String())
	output := s.runCommand(s.createExperience(experienceName, "description", "location", GithubTrue), ExpectNoError)
	s.Contains(output.StdOut, GithubCreatedExperience)
	// We expect to be able to parse the experience ID as a UUID
	experienceIDString := output.StdOut[len(GithubCreatedExperience) : len(output.StdOut)-1]
	uuid.MustParse(experienceIDString)
}

func TestEndToEndTestSuite(t *testing.T) {
	viper.AutomaticEnv()
	viper.SetDefault(Config, Dev)
	suite.Run(t, new(EndToEndTestSuite))
}
