package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	buildCmd = &cobra.Command{
		Use:     "builds",
		Short:   "builds contains commands for creating and managing builds",
		Long:    ``,
		Aliases: []string{"build"},
	}
	createBuildCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new build",
		Long:  ``,
		Run:   createBuild,
	}
	listBuildsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists existing builds",
		Long:  ``,
		Run:   listBuilds,
	}
)

const (
	buildDescriptionKey      = "description"
	buildImageURIKey         = "image"
	buildVersionKey          = "version"
	buildProjectKey          = "project"
	buildSystemKey           = "system"
	buildBranchKey           = "branch"
	buildAutoCreateBranchKey = "auto-create-branch"
	buildGithubKey           = "github"
)

func init() {
	createBuildCmd.Flags().String(buildDescriptionKey, "", "The description of the build, often a commit message")
	createBuildCmd.MarkFlagRequired(buildDescriptionKey)
	createBuildCmd.Flags().String(buildImageURIKey, "", "The URI of the docker image")
	createBuildCmd.MarkFlagRequired(buildImageURIKey)
	createBuildCmd.Flags().String(buildVersionKey, "", "The version of the build image, usually a commit ID")
	createBuildCmd.MarkFlagRequired(buildVersionKey)
	createBuildCmd.Flags().String(buildProjectKey, "", "The name or ID of the project to create the build in")
	createBuildCmd.MarkFlagRequired(buildProjectKey)
	createBuildCmd.Flags().String(buildSystemKey, "", "The name or ID of the system the build is an instance of")
	createBuildCmd.MarkFlagRequired(buildSystemKey)
	createBuildCmd.Flags().String(buildBranchKey, "", "The name or ID of the branch to nest the build in, usually the associated git branch")
	createBuildCmd.MarkFlagRequired(buildBranchKey)
	createBuildCmd.Flags().Bool(buildAutoCreateBranchKey, false, "Whether to automatically create branch if it doesn't exist")
	createBuildCmd.Flags().Bool(buildGithubKey, false, "Whether to output format in github action friendly format")
	createBuildCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	listBuildsCmd.Flags().String(buildProjectKey, "", "List builds associated with this project")
	listBuildsCmd.MarkFlagRequired(buildProjectKey)
	listBuildsCmd.Flags().String(buildBranchKey, "", "List builds associated with this branch")
	listBuildsCmd.Flags().String(buildSystemKey, "", "List builds associated with this system")
	listBuildsCmd.MarkFlagsMutuallyExclusive(buildBranchKey, buildSystemKey) // We currently only support filtering by one, the other, or none
	listBuildsCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	buildCmd.AddCommand(createBuildCmd)
	buildCmd.AddCommand(listBuildsCmd)
	rootCmd.AddCommand(buildCmd)
}

func listBuildsByBranch(projectID uuid.UUID, branchID uuid.UUID) []api.Build {
	var pageToken *string = nil

	var allBuilds []api.Build

	for {
		response, err := Client.ListBuildsForBranchesWithResponse(
			context.Background(), projectID, []api.BranchID{branchID}, &api.ListBuildsForBranchesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list builds for branch:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list builds for branch", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Builds == nil {
			log.Fatal("no builds")
		}
		allBuilds = append(allBuilds, response.JSON200.Builds...)
		if *pageToken == "" {
			break
		}
	}
	return allBuilds
}

func listBuildsBySystem(projectID uuid.UUID, systemID uuid.UUID) []api.Build {
	var pageToken *string = nil

	var allBuilds []api.Build

	for {
		response, err := Client.ListBuildsForSystemWithResponse(
			context.Background(), projectID, systemID, &api.ListBuildsForSystemParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list builds for system:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list builds for system", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Builds == nil {
			log.Fatal("no builds")
		}
		allBuilds = append(allBuilds, response.JSON200.Builds...)
		if *pageToken == "" {
			break
		}
	}
	return allBuilds
}

func listAllBuilds(projectID uuid.UUID) []api.Build {
	var pageToken *string = nil

	var allBuilds []api.Build

	for {
		response, err := Client.ListBuildsWithResponse(
			context.Background(), projectID, &api.ListBuildsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list builds:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list builds", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Builds == nil {
			log.Fatal("no builds")
		}
		allBuilds = append(allBuilds, response.JSON200.Builds...)
		if *pageToken == "" {
			break
		}
	}
	return allBuilds
}

func listBuilds(ccmd *cobra.Command, args []string) {
	// Check if the project exists, by listing projects:
	projectName := viper.GetString(buildProjectKey)
	projectID := getProjectID(Client, projectName)
	var allBuilds []api.Build
	if viper.IsSet(buildBranchKey) {
		// Check if the branch exists, by listing branches (and fail if branch not found):
		branchName := viper.GetString(buildBranchKey)
		branchID := getBranchID(Client, projectID, branchName, true)

		allBuilds = listBuildsByBranch(projectID, branchID)
	} else if viper.IsSet(buildSystemKey) {
		// Check if the system exists, by listing systems (and fail if system not found):
		systemName := viper.GetString(buildSystemKey)
		systemID := getSystemID(Client, projectID, systemName, true)

		allBuilds = listBuildsBySystem(projectID, systemID)
	} else { // no filtering
		allBuilds = listAllBuilds(projectID)
	}

	OutputJson(allBuilds)
}

func createBuild(ccmd *cobra.Command, args []string) {
	buildGithub := viper.GetBool(buildGithubKey)
	if !buildGithub {
		fmt.Println("Creating a build...")
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
	// Validate that the image URI is valid:
	_, err := name.ParseReference(buildImageURI, name.StrictValidation)
	if err != nil {
		log.Fatal("failed to parse the image URI - it must be a valid docker image URI, including tag or digest")
	}

	// Check if the project exists, by listing projects:
	projectID := getProjectID(Client, viper.GetString(buildProjectKey))

	// Check if the branch exists, by listing branches, returning uuid.Nil if branch not found:
	branchName := viper.GetString(buildBranchKey)
	branchID := getBranchID(Client, projectID, branchName, false) // we don't fail on error for branches, because we can autocreate
	systemID := getSystemID(Client, projectID, viper.GetString(buildSystemKey), true)
	if branchID == uuid.Nil {
		if viper.GetBool(buildAutoCreateBranchKey) {
			if !buildGithub {
				fmt.Printf("Branch with name %v doesn't currently exist. Creating... \n", branchName)
			}
			branchType := api.CHANGEREQUEST
			if branchName == "main" || branchName == "master" {
				branchType = api.MAIN
			}
			// Create the branch
			body := api.CreateBranchInput{
				Name:       branchName,
				BranchType: branchType,
			}

			response, err := Client.CreateBranchForProjectWithResponse(context.Background(), projectID, body)
			if err != nil {
				log.Fatal("failed to create branch:", err)
			}
			ValidateResponse(http.StatusCreated, fmt.Sprintf("failed to create a new branch with name %v", branchName),
				response.HTTPResponse, response.Body)
			branchID = response.JSON201.BranchID
			if !buildGithub {
				fmt.Printf("Created branch with ID %v\n", branchID)
			}
		} else {
			log.Fatal("Branch does not exist, and auto-create-branch is false, so not creating branch")
		}
	}

	body := api.CreateBuildForBranchInput{
		Description: &buildDescription,
		ImageUri:    buildImageURI,
		Version:     buildVersion,
		SystemID:    systemID,
	}

	response, err := Client.CreateBuildForBranchWithResponse(context.Background(), projectID, branchID, body)
	if err != nil {
		log.Fatal("unable to create build:", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create build", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	build := *response.JSON201
	if build.BuildID == uuid.Nil {
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
