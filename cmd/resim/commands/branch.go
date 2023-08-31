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
	branchCmd = &cobra.Command{
		Use:     "branches",
		Short:   "branches contains commands for creating and managing branches",
		Long:    ``,
		Aliases: []string{"branch"},
	}
	createBranchCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new branch",
		Long:   ``,
		Run:    createBranch,
		PreRun: RegisterViperFlagsAndSetClient,
	}
)

const (
	branchNameKey      = "name"
	branchProjectIDKey = "project"
	branchTypeKey      = "type"
	branchGithubKey    = "github"
)

func init() {
	createBranchCmd.Flags().String(branchNameKey, "", "The name of the branch, often a repository name")
	createBranchCmd.MarkFlagRequired(branchNameKey)
	createBranchCmd.Flags().String(branchProjectIDKey, "", "The name or ID of the project to associate with the branch")
	createBranchCmd.MarkFlagRequired(branchProjectIDKey)
	createBranchCmd.Flags().String(branchTypeKey, "", "The type of the branch: 'RELEASE', 'MAIN', or 'CHANGE_REQUEST'")
	createBranchCmd.MarkFlagRequired(branchTypeKey)
	createBranchCmd.Flags().Bool(branchGithubKey, false, "Whether to output format in github action friendly format")
	createBranchCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)
	branchCmd.AddCommand(createBranchCmd)
	rootCmd.AddCommand(branchCmd)
}

func listBranches(ccmd *cobra.Command, args []string) {
	projectID, err := uuid.Parse(viper.GetString(branchProjectIDKey))
	if err != nil || projectID == uuid.Nil {
		log.Fatal("empty project ID")
	}

}

func createBranch(ccmd *cobra.Command, args []string) {
	if !viper.GetBool(branchGithubKey) {
		fmt.Println("Creating a branch...")
	}
	// Parse the various arguments from command line
	projectID := getProjectID(Client, viper.GetString(branchProjectIDKey))

	branchName := viper.GetString(branchNameKey)
	if branchName == "" {
		log.Fatal("empty branch name")
	}

	branchType := api.BranchType(viper.GetString(branchTypeKey))
	if branchType != api.RELEASE && branchType != api.MAIN && branchType != api.CHANGEREQUEST {
		log.Fatal("invalid branch type")
	}

	body := api.CreateBranchForProjectJSONRequestBody{
		Name:       &branchName,
		BranchType: &branchType,
	}

	response, err := Client.CreateBranchForProjectWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("unable to create branch: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create branch", response.HTTPResponse)
	if response.JSON201 == nil {
		log.Fatal("empty branch returned")
	}
	branch := *response.JSON201
	if branch.BranchID == nil {
		log.Fatal("no branch ID")
	}

	// Report the results back to the user
	if viper.GetBool(branchGithubKey) {
		fmt.Printf("branch_id=%s\n", branch.BranchID.String())
	} else {
		fmt.Println("Created branch successfully!")
		fmt.Printf("Branch ID: %s\n", branch.BranchID.String())
	}
}

func getBranchIDForName(client api.ClientWithResponsesInterface, projectID uuid.UUID, buildBranchName string) uuid.UUID {
	// Page through branches until we find the one we want:
	var branchID uuid.UUID = uuid.Nil
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListBranchesForProjectWithResponse(
			context.Background(), projectID, &api.ListBranchesForProjectParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list branches:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list branches", response.HTTPResponse)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Branches == nil {
			log.Fatal("no branches")
		}
		branches := *response.JSON200.Branches
		for _, branch := range branches {
			if *branch.Name == buildBranchName {
				branchID = *branch.BranchID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return branchID
}
