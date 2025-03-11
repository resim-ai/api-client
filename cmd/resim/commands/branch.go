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
		Use:   "create",
		Short: "create - Creates a new branch",
		Long:  ``,
		Run:   createBranch,
	}
	listBranchesCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List branches for a project",
		Long:  ``,
		Run:   listBranches,
	}
)

const (
	branchNameKey    = "name"
	branchProjectKey = "project"
	branchTypeKey    = "type"
	branchGithubKey  = "github"
)

func init() {
	createBranchCmd.Flags().String(branchNameKey, "", "The name of the branch, often a repository name")
	createBranchCmd.MarkFlagRequired(branchNameKey)
	createBranchCmd.Flags().String(branchProjectKey, "", "The name or ID of the project to associate with the branch")
	createBranchCmd.MarkFlagRequired(branchProjectKey)
	createBranchCmd.Flags().String(branchTypeKey, "", "The type of the branch: 'RELEASE', 'MAIN', or 'CHANGE_REQUEST'")
	createBranchCmd.MarkFlagRequired(branchTypeKey)
	createBranchCmd.Flags().Bool(branchGithubKey, false, "Whether to output format in GitHub Actions friendly format")
	createBranchCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	listBranchesCmd.Flags().String(branchProjectKey, "", "The name or ID of the project from which to list branches")
	listBranchesCmd.MarkFlagRequired(branchProjectKey)
	listBranchesCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	branchCmd.AddCommand(createBranchCmd)
	branchCmd.AddCommand(listBranchesCmd)
	rootCmd.AddCommand(branchCmd)
}

func listBranches(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(branchProjectKey))

	var pageToken *string = nil

	var allBranches []api.Branch

	for {
		response, err := Client.ListBranchesForProjectWithResponse(
			context.Background(), projectID, &api.ListBranchesForProjectParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list branches:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list branches", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Branches == nil {
			log.Fatal("no branches")
		}
		allBranches = append(allBranches, *response.JSON200.Branches...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(allBranches)
}

func createBranch(ccmd *cobra.Command, args []string) {
	if !viper.GetBool(branchGithubKey) {
		fmt.Println("Creating a branch...")
	}
	// Parse the various arguments from command line
	projectID := getProjectID(Client, viper.GetString(branchProjectKey))

	branchName := viper.GetString(branchNameKey)
	if branchName == "" {
		log.Fatal("empty branch name")
	}

	branchType := api.BranchType(viper.GetString(branchTypeKey))
	if branchType != api.RELEASE && branchType != api.MAIN && branchType != api.CHANGEREQUEST {
		log.Fatal("invalid branch type")
	}

	body := api.CreateBranchInput{
		Name:       branchName,
		BranchType: branchType,
	}

	response, err := Client.CreateBranchForProjectWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("unable to create branch: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create branch", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty branch returned")
	}
	branch := *response.JSON201
	if branch.BranchID == uuid.Nil {
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

// Returns the branch ID for the given branch identifier. If the branch identifier is a name, it is looked up. If it is a UUID, it is returned as-is.
// If the branch is not found, uuid.Nil is returned unless failWhenNotFound is true, when it will log a fatal error.
func getBranchID(client api.ClientWithResponsesInterface, projectID uuid.UUID, branchIdentifier string, failWhenNotFound bool) uuid.UUID {
	branchID := checkBranchID(client, projectID, branchIdentifier)
	if branchID == uuid.Nil && failWhenNotFound {
		log.Fatalf("branch '%s' not found", branchIdentifier)
	}
	return branchID
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func checkBranchID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string) uuid.UUID {
	branchID := uuid.Nil
	// First try the assumption that identifier is a UUID.
	err := uuid.Validate(identifier)
	if err == nil {
		// The identifier is a uuid - but does it refer to an existing branch?
		potentialBranchID := uuid.MustParse(identifier)
		response, _ := client.GetBranchForProjectWithResponse(context.Background(), projectID, potentialBranchID)
		if response.HTTPResponse.StatusCode == http.StatusOK {
			// Branch found with ID
			return potentialBranchID
		}
	}
	// If we're here then either the identifier is not a UUID or the UUID was not
	// found. Users could choose to name branches with UUIDs so regardless of how
	// we got here we now search for identifier as a string name.
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListBranchesForProjectWithResponse(
			context.Background(), projectID, &api.ListBranchesForProjectParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list branches:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list branches", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		pageToken = response.JSON200.NextPageToken
		branches := *response.JSON200.Branches
		for _, branch := range branches {
			if branch.Name == "" {
				log.Fatal("branch has no name")
			}
			if branch.BranchID == uuid.Nil {
				log.Fatal("branch ID is empty")
			}
			if branch.Name == identifier {
				branchID = branch.BranchID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return branchID
}

func getOrCreateBranchID(client api.ClientWithResponsesInterface, projectID uuid.UUID, branchName string, github bool) uuid.UUID {
	branchID := getBranchID(client, projectID, branchName, false)
	if branchID == uuid.Nil {
		if !github {
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
		if !github {
			fmt.Printf("Created branch with ID %v\n", branchID)
		}
	}
	return branchID
}
