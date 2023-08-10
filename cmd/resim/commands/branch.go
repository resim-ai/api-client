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

	branchName            string
	branchProjectIDString string
	branchTypeString      string
	branchGithub          bool
)

func init() {
	createBranchCmd.Flags().StringVar(&branchName, "name", "", "The name of the branch, often a repository name")
	createBranchCmd.Flags().StringVar(&branchProjectIDString, "project_id", "", "The ID of the project to associate the branch to")
	createBranchCmd.Flags().StringVar(&branchTypeString, "type", "", "The type of the branch: 'RELEASE', 'MAIN', or 'CHANGE_REQUEST'")
	createBranchCmd.Flags().BoolVar(&branchGithub, "github", false, "Whether to output format in github action friendly format")
	branchCmd.AddCommand(createBranchCmd)
	rootCmd.AddCommand(branchCmd)
}

func createBranch(ccmd *cobra.Command, args []string) {
	if !branchGithub {
		fmt.Println("Creating a branch...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	projectID, err := uuid.Parse(branchProjectIDString)
	if err != nil || projectID == uuid.Nil {
		log.Fatal("Empty project id")
	}

	if branchName == "" {
		log.Fatal("Empty Branch name")
	}

	branchType := api.BranchType(branchTypeString)
	if branchType != api.RELEASE && branchType != api.MAIN && branchType != api.CHANGEREQUEST {
		log.Fatal("Invalid branch type")
	}

	body := api.CreateBranchForProjectJSONRequestBody{
		Name:       &branchName,
		BranchType: &branchType,
	}

	response, err := client.CreateBranchForProjectWithResponse(context.Background(), projectID, body)

	if err != nil {
		log.Fatal(err)
	}

	// Report the results back to the user
	success := response.HTTPResponse.StatusCode == http.StatusCreated
	if success {
		if branchGithub {
			fmt.Printf("branch_id=%s\n", response.JSON201.BranchID.String())
		} else {
			fmt.Println("Created branch successfully!")
			fmt.Printf("Branch ID: %s\n", response.JSON201.BranchID.String())
		}
	} else {
		log.Fatal("Failed to create branch!\n", string(response.Body))
	}

}
