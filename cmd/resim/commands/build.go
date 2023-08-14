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
	buildCmd = &cobra.Command{
		Use:   "build",
		Short: "build contains commands for creating and managing builds",
		Long:  ``,
	}
	createBuildCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new build",
		Long:   ``,
		Run:    createBuild,
		PreRun: RegisterViperFlags,
	}
)

const (
	buildDescriptionKey      = "description"
	buildImageURIKey         = "image"
	buildVersionKey          = "version"
	buildProjectNameKey      = "project_name"
	buildBranchNameKey       = "branch_name"
	buildAutoCreateBranchKey = "auto_create_branch"
	buildGithubKey           = "github"
)

func init() {
	createBuildCmd.Flags().String(buildDescriptionKey, "", "The description of the build, often a commit message")
	createBuildCmd.MarkFlagRequired(buildDescriptionKey)
	createBuildCmd.Flags().String(buildImageURIKey, "", "The URI of the docker image")
	createBuildCmd.MarkFlagRequired(buildImageURIKey)
	createBuildCmd.Flags().String(buildVersionKey, "", "The version of the build image, usually a commit ID")
	createBuildCmd.MarkFlagRequired(buildVersionKey)
	createBuildCmd.Flags().String(buildProjectNameKey, "", "The name of the project to create the build in")
	createBuildCmd.MarkFlagRequired(buildProjectNameKey)
	createBuildCmd.Flags().String(buildBranchNameKey, "", "The name of the branch to nest the build in, usually the associated git branch")
	createBuildCmd.Flags().Bool(buildAutoCreateBranchKey, false, "Whether to automatically create branch if it doesn't exist")
	createBuildCmd.MarkFlagsMutuallyExclusive(buildBranchNameKey, buildAutoCreateBranchKey)
	createBuildCmd.Flags().Bool(buildGithubKey, false, "Whether to output format in github action friendly format")
	buildCmd.AddCommand(createBuildCmd)
	rootCmd.AddCommand(buildCmd)
}

func createBuild(ccmd *cobra.Command, args []string) {
	buildGithub := viper.GetBool(buildGithubKey)
	if !buildGithub {
		fmt.Println("Creating a build...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	buildDescription := viper.GetString(buildDescriptionKey)
	if buildDescription == "" {
		log.Fatal("empty build description")
	}

	buildVersion := viper.GetString(buildVersionKey)
	if buildVersion == "" {
		log.Fatal("empty build version")
	}

	buildImageURI := viper.GetString(buildImageURIKey)
	if buildImageURI == "" {
		log.Fatal("empty build image URI")
	}

	// Check if the project exists, by listing projects:
	projectName := viper.GetString(buildProjectNameKey)
	projectID := getProjectIDForName(client, projectName)

	// Check if the branch exists, by listing branches:
	branchName := viper.GetString(buildBranchNameKey)
	branchID := getBranchIDForName(client, projectID, branchName)

	if branchID == uuid.Nil {
		if viper.GetBool(buildAutoCreateBranchKey) {
			if !buildGithub {
				fmt.Printf("Branch with name %v doesn't currently exist. Creating... \n", branchName)
			}
			// Create the branch
			body := api.CreateBranchForProjectJSONRequestBody{
				Name:       &branchName,
				BranchType: Ptr(api.CHANGEREQUEST),
			}

			response, err := client.CreateBranchForProjectWithResponse(context.Background(), projectID, body)
			if err != nil {
				log.Fatal(fmt.Sprintf("Failed to create a new branch with name %v ", branchName), err)
			}
			branchID = *response.JSON201.BranchID
			if !buildGithub {
				fmt.Printf("Created branch with ID %v\n", branchID)
			}
		} else {
			log.Fatal("Branch does not exist, and auto_create_branch is false, so not creating branch")
		}
	}

	body := api.CreateBuildForBranchJSONRequestBody{
		Description: &buildDescription,
		ImageUri:    &buildImageURI,
		Version:     &buildVersion,
	}

	response, err := client.CreateBuildForBranchWithResponse(context.Background(), projectID, branchID, body)
	if err != nil || response.StatusCode() != http.StatusCreated {
		var message string
		if response != nil && response.Body != nil {
			message = string(response.Body)
		}
		log.Fatal("unable to create build ", err, message)
	}
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	build := *response.JSON201
	if build.BuildID == nil {
		log.Fatal("no build ID")
	}

	// Report the results back to the user
	if buildGithub {
		fmt.Printf("build_id=%s\n", build.BuildID.String())
	} else {
		fmt.Println("Created build successfully!")
		fmt.Printf("Build ID: %s\n", build.BuildID.String())
	}
}
