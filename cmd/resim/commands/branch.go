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
		Use:   "branch",
		Short: "branch contains commands for creating and managing branches",
		Long:  ``,
	}
	createBranchCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new branch",
		Long:  ``,
		Run:   createBranch,
	}
)

const (
	branchNameKey      = "name"
	branchProjectIDKey = "project_id"
	branchTypeKey      = "type"
	branchGithubKey    = "github"
)

func init() {
	createBranchCmd.Flags().String(branchNameKey, "", "The name of the branch, often a repository name")
	createBranchCmd.Flags().String(branchProjectIDKey, "", "The ID of the project to associate the branch to")
	createBranchCmd.Flags().String(branchTypeKey, "", "The type of the branch: 'RELEASE', 'MAIN', or 'CHANGE_REQUEST'")
	createBranchCmd.Flags().Bool(branchGithubKey, false, "Whether to output format in github action friendly format")
	branchCmd.AddCommand(createBranchCmd)
	rootCmd.AddCommand(branchCmd)
}

func createBranch(ccmd *cobra.Command, args []string) {
	if !viper.GetBool(branchGithubKey) {
		fmt.Println("Creating a branch...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	projectID, err := uuid.Parse(viper.GetString(branchProjectIDKey))
	if err != nil || projectID == uuid.Nil {
		log.Fatal("empty project ID")
	}

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

	response, err := client.CreateBranchForProjectWithResponse(context.Background(), projectID, body)
	if err != nil || response.StatusCode() != http.StatusCreated {
		var message string
		if response != nil && response.Body != nil {
			message = string(response.Body)
		}
		log.Fatal("unable to create branch ", err, message)
	}
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

func getBranchIDForName(client *api.ClientWithResponses, projectID uuid.UUID, buildBranchName string) uuid.UUID {
	// Page through branches until we find the one we want:
	var branchID uuid.UUID = uuid.Nil
	var pageToken *string = nil
pageLoop:
	for {
		listResponse, err := client.ListBranchesForProjectWithResponse(
			context.Background(), projectID, &api.ListBranchesForProjectParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil || listResponse.StatusCode() != http.StatusOK {
			var message string
			if listResponse.Body != nil {
				message = string(listResponse.Body)
			}
			log.Fatal("failed to find branch: ", err, message)
		}

		pageToken = listResponse.JSON200.NextPageToken
		if listResponse.JSON200 == nil || listResponse.JSON200.Branches == nil {
			log.Fatal("no branches")
		}
		branches := *listResponse.JSON200.Branches
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
