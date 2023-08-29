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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
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

func (s *EndToEndTestSuite) runCommand(commandBuilders []CommandBuilder) Output {
	var stdout, stderr bytes.Buffer
	cmd := s.buildCommand(commandBuilders)
	fmt.Println("About to run command: ", cmd.String())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	s.NoError(err)
	return Output{
		StdOut: stdout.String(),
		StdErr: stderr.String(),
	}
}

// As a first test, we expect the help command to run successfully
func (s *EndToEndTestSuite) TestHelp() {
	fmt.Println("Testing help command")
	runCommand := CommandBuilder{
		Command: "help",
	}
	output := s.runCommand([]CommandBuilder{runCommand})
	s.Contains(output.StdOut, "Usage:")
}

func (s *EndToEndTestSuite) TestProjectCreate() {
	fmt.Println("Testing project create command")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())

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
				Value: "description",
			},
		},
	}
	output := s.runCommand([]CommandBuilder{projectCommand, createCommand})
	s.Contains(output.StdOut, "Created project")
}

func (s *EndToEndTestSuite) TestProjectCreateGithub() {
	fmt.Println("Testing project create command, with --github flag")

	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	// We build a create project command with the name and description flags
	projectCommand := CommandBuilder{
		Command: "project",
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
	return s.runCommand([]CommandBuilder{branchCommand, createCommand})
}

func (s *EndToEndTestSuite) createBuild(projectName string, branchName string, description string, image string, version string, github bool) Output {
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
	return s.runCommand([]CommandBuilder{buildCommand, createCommand})
}

func (s *EndToEndTestSuite) createExperience(name string, description string, location string, github bool) Output {
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
	return s.runCommand([]CommandBuilder{experienceCommand, createCommand})
}

func (s *EndToEndTestSuite) TestProjectCreate() {
	fmt.Println("Testing project create command")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", false)
	s.Contains(output.StdOut, "Created project")
	s.Contains(output.StdOut, "Project ID: ")
	s.Empty(output.StdErr)
}

func (s *EndToEndTestSuite) TestProjectCreateGithub() {
	fmt.Println("Testing project create command, with --github flag")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", true)
	s.Contains(output.StdOut, "project_id=")
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len("project_id=") : len(output.StdOut)-1]
	uuid.MustParse(projectIDString)
}

// Test branch creation:
func (s *EndToEndTestSuite) TestBranchCreate() {
	fmt.Println("Testing branch creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", true)
	s.Contains(output.StdOut, "project_id=")
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len("project_id=") : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.createBranch(projectID, branchName, "RELEASE", false)
	s.Contains(output.StdOut, "Created branch")
	s.Contains(output.StdOut, "Branch ID: ")
}

func (s *EndToEndTestSuite) TestBranchCreateGithub() {
	fmt.Println("Testing branch creation, with github flag enabled")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", true)
	s.Contains(output.StdOut, "project_id=")
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len("project_id=") : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.createBranch(projectID, branchName, "RELEASE", true)
	s.Contains(output.StdOut, "branch_id=")
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len("branch_id=") : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)
}

// Test the build creation:
func (s *EndToEndTestSuite) TestBuildCreate() {
	fmt.Println("Testing build creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", true)
	s.Contains(output.StdOut, "project_id=")
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len("project_id=") : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.createBranch(projectID, branchName, "RELEASE", true)
	s.Contains(output.StdOut, "branch_id=")
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len("branch_id=") : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", false)
	s.Contains(output.StdOut, "Created build")
}

func (s *EndToEndTestSuite) TestBuildCreateGithub() {
	fmt.Println("Testing build creation")

	// First create a project
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", true)
	s.Contains(output.StdOut, "project_id=")
	// We expect to be able to parse the project ID as a UUID
	projectIDString := output.StdOut[len("project_id=") : len(output.StdOut)-1]
	projectID := uuid.MustParse(projectIDString)

	// Now create the branch:
	branchName := fmt.Sprintf("test-branch-%s", uuid.New().String())
	output = s.createBranch(projectID, branchName, "RELEASE", true)
	s.Contains(output.StdOut, "branch_id=")
	// We expect to be able to parse the branch ID as a UUID
	branchIDString := output.StdOut[len("branch_id=") : len(output.StdOut)-1]
	uuid.MustParse(branchIDString)

	// Now create the build:
	output = s.createBuild(projectName, branchName, "description", "public.ecr.aws/docker/library/hello-world", "1.0.0", true)
	s.Contains(output.StdOut, "build_id=")
	// We expect to be able to parse the build ID as a UUID
	buildIDString := output.StdOut[len("build_id=") : len(output.StdOut)-1]
	uuid.MustParse(buildIDString)
}

func (s *EndToEndTestSuite) TestExperienceCreate() {
	fmt.Println("Testing project create command")
	projectName := fmt.Sprintf("test-project-%s", uuid.New().String())
	output := s.createProject(projectName, "description", false)
	s.Contains(output.StdOut, "Created project")
	s.Contains(output.StdOut, "Project ID: ")
	s.Empty(output.StdErr)
}

func TestEndToEndTestSuite(t *testing.T) {
	viper.AutomaticEnv()
	viper.SetDefault(Config, Dev)
	suite.Run(t, new(EndToEndTestSuite))
}
