package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
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

	buildDescription     string
	buildImageName       string
	buildVersion         string
	buildProjectIDString string
	buildBranchIDString  string
	buildGithub          bool
)

func init() {
	createBuildCmd.Flags().StringVar(&buildDescription, "description", "", "The description of the build, often a commit message")
	createBuildCmd.Flags().StringVar(&buildImageName, "image", "", "The URI of the docker image")
	createBuildCmd.Flags().StringVar(&buildVersion, "version", "", "The version of the build image, usually a commit ID")
	createBuildCmd.Flags().StringVar(&buildProjectIDString, "project_id", "", "The ID of the project to create the build in")
	createBuildCmd.Flags().StringVar(&buildBranchIDString, "branch_id", "", "The ID of the branch to nest the build in, usually the associated git branch")
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

	if buildImageName == "" {
		log.Fatal("Empty build image name")
	}

	projectID, err := uuid.Parse(buildProjectIDString)
	if err != nil || projectID == uuid.Nil {
		log.Fatal("Empty project id")
	}

	branchID, err := uuid.Parse(buildBranchIDString)
	if err != nil || branchID == uuid.Nil {
		log.Fatal("Empty branch id")
	}

	body := api.CreateBuildForBranchJSONRequestBody{
		Description: &buildDescription,
		ImageName:   &buildImageName,
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
