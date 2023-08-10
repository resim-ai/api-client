package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/rerun/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
)

var (
	buildCmd = &cobra.Command{
		Use:   "build",
		Short: "build contains commands for creating and managing builds",
		Long:  ``,
	}
	createBuildCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new build",
		Long:  ``,
		Run:   createBuild,
	}

	buildDescription      string
	buildImageUri         string
	buildVersion          string
	buildProjectName      string
	buildBranchName       string
	buildAutoCreateBranch bool
	buildGithub           bool
)

func init() {
	createBuildCmd.Flags().StringVar(&buildDescription, "description", "", "The description of the build, often a commit message")
	createBuildCmd.Flags().StringVar(&buildImageUri, "image", "", "The URI of the docker image")
	createBuildCmd.Flags().StringVar(&buildVersion, "version", "", "The version of the build image, usually a commit ID")
	createBuildCmd.Flags().StringVar(&buildProjectName, "project_name", "", "The name of the project to create the build in")
	createBuildCmd.Flags().StringVar(&buildBranchName, "branch_name", "", "The name of the branch to nest the build in, usually the associated git branch")
	createBuildCmd.Flags().BoolVar(&buildAutoCreateBranch, "auto_create_branch", false, "Whether to automatically create branch if it doesn't exist")
	createBuildCmd.Flags().BoolVar(&buildGithub, "github", false, "Whether to output format in github action friendly format")
	buildCmd.AddCommand(createBuildCmd)
	rootCmd.AddCommand(buildCmd)
}

func createBuild(ccmd *cobra.Command, args []string) {
	if !buildGithub {
		fmt.Println("Creating a build...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	if buildDescription == "" {
		log.Fatal("Empty build description")
	}

	if buildVersion == "" {
		log.Fatal("Empty build version")
	}

	if buildImageUri == "" {
		log.Fatal("Empty build image uri")
	}

	// Check if the project exists, by listing projects:
	projectID := getProjectIDForName(client, buildProjectName)

	// Check if the branch exists, by listing branches:
	branchID := getBranchIDForName(client, projectID, buildProjectName)

	if branchID == uuid.Nil {
		if buildAutoCreateBranch {
			if !buildGithub {
				fmt.Printf("Branch with name %v doesn't currently exist. Creating...", buildBranchName)
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
		ImageUri:    &buildImageUri,
		Version:     &buildVersion,
	}

	response, err := client.CreateBuildForBranchWithResponse(context.Background(), projectID, branchID, body)

	if err != nil {
		log.Fatal(err)
	}

	// Report the results back to the user
	success := response.HTTPResponse.StatusCode == http.StatusCreated
	if success {
		if buildGithub {
			fmt.Printf("build_id=%s\n", response.JSON201.BuildID.String())
		} else {
			fmt.Println("Created build successfully!")
			fmt.Printf("Build ID: %s\n", response.JSON201.BuildID.String())
		}
	} else {
		log.Fatal("Failed to create build!\n", string(response.Body))
	}

}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func getProjectIDForName(client *api.ClientWithResponses, buildProjectName string) uuid.UUID {
	// Page through projects until we find the one we want:
	var projectID uuid.UUID = uuid.Nil
	var pageToken *string = nil
	found := false
	for {
		listResponse, err := client.ListProjectsWithResponse(
			context.Background(), &api.ListProjectsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("Failed to find project with error: ", err)
		}

		pageToken = listResponse.JSON200.NextPageToken
		projects := *listResponse.JSON200.Projects
		for _, project := range projects {
			if *project.Name == buildProjectName {
				projectID = *project.ProjectID
				found = true
				break
			}
		}
		if found || *pageToken == "" {
			break
		}
	}
	if !found {
		log.Fatal("Failed to find project with requested name: ", buildProjectName)
	}
	return projectID
}

func getBranchIDForName(client *api.ClientWithResponses, projectID uuid.UUID, buildBranchName string) uuid.UUID {
	// Page through branches until we find the one we want:
	var branchID uuid.UUID = uuid.Nil
	var pageToken *string = nil
	found := false
	for {
		listResponse, err := client.ListBranchesForProjectWithResponse(
			context.Background(), projectID, &api.ListBranchesForProjectParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("Failed to find branch with error: ", err)
		}

		pageToken = listResponse.JSON200.NextPageToken
		branches := *listResponse.JSON200.Branches
		for _, branch := range branches {
			if *branch.Name == buildBranchName {
				branchID = *branch.BranchID
				found = true
				break
			}
		}
		if found || *pageToken == "" {
			break
		}
	}

	// We return the branch ID whether or not it is found:
	return branchID
}
