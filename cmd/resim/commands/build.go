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

	updateBuildCmd = &cobra.Command{
		Use:   "update",
		Short: "update - Updates a build (either branch or description)",
		Long:  ``,
		Run:   updateBuild,
	}

	getBuildCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Get a build by ID",
		Long:  ``,
		Run:   getBuild,
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
	buildBranchIDKey         = "branch-id"
	buildBuildIDKey          = "build-id"
	buildAutoCreateBranchKey = "auto-create-branch"
	buildGithubKey           = "github"
	buildSpecKey             = "build-spec"
	buildSpecNameKey         = "build-spec-name"
	buildShowBuildSpecKey    = "show-build-spec-only"
)

func init() {
	createBuildCmd.Flags().String(buildDescriptionKey, "", "The description of the build, often a commit message")
	createBuildCmd.MarkFlagRequired(buildDescriptionKey)
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
	createBuildCmd.Flags().String(buildImageURIKey, "", "The URI of the docker image")
	createBuildCmd.Flags().StringSlice(buildSpecKey, []string{}, "Paths to files containing build specs")
	createBuildCmd.Flags().String(buildSpecNameKey, "", "The name of the build spec")
	createBuildCmd.MarkFlagsMutuallyExclusive(buildImageURIKey, buildSpecKey)

	listBuildsCmd.Flags().String(buildProjectKey, "", "List builds associated with this project")
	listBuildsCmd.MarkFlagRequired(buildProjectKey)
	listBuildsCmd.Flags().String(buildBranchKey, "", "List builds associated with this branch")
	listBuildsCmd.Flags().String(buildSystemKey, "", "List builds associated with this system")
	listBuildsCmd.MarkFlagsMutuallyExclusive(buildBranchKey, buildSystemKey) // We currently only support filtering by one, the other, or none
	listBuildsCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	updateBuildCmd.Flags().String(buildProjectKey, "", "The name or ID of the project the build belongs to")
	updateBuildCmd.MarkFlagRequired(buildProjectKey)
	updateBuildCmd.Flags().String(buildBuildIDKey, "", "The ID of the build to update")
	createBuildCmd.MarkFlagRequired(buildBuildIDKey)
	updateBuildCmd.Flags().String(buildBranchIDKey, "", "New value for the build's branch ID")
	updateBuildCmd.Flags().String(buildDescriptionKey, "", "New value for the description of the build")

	getBuildCmd.Flags().String(buildProjectKey, "", "The name or ID of the project the build belongs to")
	getBuildCmd.MarkFlagRequired(buildProjectKey)
	getBuildCmd.Flags().String(buildBuildIDKey, "", "The ID of the build to get")
	getBuildCmd.MarkFlagRequired(buildBuildIDKey)
	getBuildCmd.Flags().Bool(buildShowBuildSpecKey, false, "Print the build spec only.")

	buildCmd.AddCommand(createBuildCmd)
	buildCmd.AddCommand(listBuildsCmd)
	buildCmd.AddCommand(updateBuildCmd)
	buildCmd.AddCommand(getBuildCmd)

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

	inputBuildImageURI := viper.GetString(buildImageURIKey)
	var buildImageURI *string = nil

	buildSpecLocations := viper.GetStringSlice(buildSpecKey)
	var buildSpec *[]byte = nil

	// Check that at least one of image URI or build spec is provided
	if inputBuildImageURI == "" && len(buildSpecLocations) == 0 {
		log.Fatal("either --build-spec or --image is required")
	}

	// Check that the required flag is present when using build-spec
	if len(buildSpecLocations) > 0 && inputBuildImageURI == "" {
		// Replace YOUR_REQUIRED_FLAG with the flag name that should be required
		if viper.GetString(buildSpecNameKey) == "" {
			log.Fatal("--build-spec-name is required when using --build-spec")
		}
	}

	if inputBuildImageURI != "" {
		// Validate that the image URI is valid:
		_, err := name.ParseReference(inputBuildImageURI, name.StrictValidation)
		if err != nil {
			log.Fatal("failed to parse the image URI - it must be a valid docker image URI, including tag or digest")
		}

		buildImageURI = Ptr(inputBuildImageURI)
	}

	if len(buildSpecLocations) > 0 {
		buildSpecBytes, err := ParseBuildSpec(buildSpecLocations)
		if err != nil {
			log.Fatal("failed to parse build spec:", err)
		}
		buildSpec = Ptr(buildSpecBytes)
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
		Description:        Ptr(buildDescription),
		ImageUri:           buildImageURI,
		BuildSpecification: buildSpec,
		Version:            buildVersion,
		SystemID:           systemID,
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

func updateBuild(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(buildProjectKey))
	buildID, err := uuid.Parse(viper.GetString(buildBuildIDKey))
	if err != nil {
		log.Fatal("unable to parse build ID:", err)
	}
	// Check the build id exists
	_, err = Client.GetBuildWithResponse(context.Background(), projectID, buildID)
	if err != nil {
		log.Fatal("unable to get build:", err)
	}
	updateBuildInput := api.UpdateBuildInput{
		Build: &api.UpdateBuildFields{},
	}
	updateMask := []string{}
	if viper.IsSet(buildBranchIDKey) {
		branchID := getBranchID(Client, projectID, viper.GetString(buildBranchIDKey), true)
		updateBuildInput.Build.BranchID = Ptr(branchID)
		updateMask = append(updateMask, "branchID")
	}
	if viper.IsSet(buildDescriptionKey) {
		updateBuildInput.Build.Description = Ptr(viper.GetString(buildDescriptionKey))
		updateMask = append(updateMask, "description")
	}
	updateBuildInput.UpdateMask = Ptr(updateMask)
	response, err := Client.UpdateBuildWithResponse(context.Background(), projectID, buildID, updateBuildInput)
	if err != nil {
		log.Fatal("unable to update build:", err)
	}
	ValidateResponse(http.StatusOK, "unable to update build", response.HTTPResponse, response.Body)
	fmt.Println("Updated build successfully!")
}

func getBuild(ccmd *cobra.Command, args []string) {
	var build *api.Build
	projectID := getProjectID(Client, viper.GetString(buildProjectKey))
	buildID := getBuildID(Client, projectID, viper.GetString(buildBuildIDKey))
	response, err := Client.GetBuildWithResponse(context.Background(), projectID, buildID)
	if err != nil {
		log.Fatal("unable to retrieve build:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatal("failed to find build with requested id: ", projectID.String())
	} else {
		ValidateResponse(http.StatusOK, "unable to retrieve build", response.HTTPResponse, response.Body)
	}

	build = response.JSON200

	if viper.GetBool(buildShowBuildSpecKey) {
		// Show the build spec only if the flag is set.
		fmt.Println(build.BuildSpecification)
	} else {
		OutputJson(build)
	}
}

func getBuildID(client api.ClientWithResponsesInterface, projectID uuid.UUID, uuidString string) uuid.UUID {
	err := uuid.Validate(uuidString)
	if err != nil {
		log.Fatal("invalid build ID: ", uuidString)
	}
	potentialBuildID := uuid.MustParse(uuidString)
	response, err := client.GetBuildWithResponse(context.Background(), projectID, potentialBuildID)
	if err != nil {
		log.Fatal("failed to find build with ID: ", uuidString)
	}
	if response.HTTPResponse.StatusCode != http.StatusOK {
		log.Fatal("failed to find build with ID: ", uuidString)
	}
	return potentialBuildID
}

// Attempt to find an existing build for that branch, imageURI, and version
func getBuildIDFromImageURIAndVersion(client api.ClientWithResponsesInterface, projectID uuid.UUID, systemID uuid.UUID, branchID uuid.UUID, imageURI string, version string, shouldFail bool) uuid.UUID {
	var pageToken *string = nil
	for {
		response, err := client.ListBuildsWithResponse(context.Background(), projectID, &api.ListBuildsParams{
			PageSize:  Ptr(100),
			OrderBy:   Ptr("timestamp"),
			Search:    Ptr(fmt.Sprintf("branch_id=\"%v\"", branchID)),
			PageToken: pageToken,
		})
		if err != nil {
			log.Fatal("failed to list builds:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list builds", response.HTTPResponse, response.Body)
		if response.JSON200 == nil || response.JSON200.Builds == nil {
			log.Fatal("no builds")
		}
		pageToken = &response.JSON200.NextPageToken
		for _, build := range response.JSON200.Builds {
			if build.ImageUri == imageURI && build.Version == version {
				return build.BuildID
			}
		}
		if *pageToken == "" {
			break
		}
	}
	if shouldFail {
		log.Fatal("failed to find build with image URI and version: ", imageURI, version)
	}
	return uuid.Nil
}
