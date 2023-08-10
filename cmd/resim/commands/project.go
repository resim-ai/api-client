package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
)

var (
	projectCmd = &cobra.Command{
		Use:   "project",
		Short: "project contains commands for creating and managing projects",
		Long:  ``,
	}
	createProjectCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new project",
		Long:  ``,
		Run:   createProject,
	}

	projectName        string
	projectDescription string
	projectGithub      bool
)

func init() {
	createProjectCmd.Flags().StringVar(&projectName, "name", "", "The name of the project, often a repository name")
	createProjectCmd.Flags().StringVar(&projectDescription, "description", "", "The description of the project")
	createProjectCmd.Flags().BoolVar(&projectGithub, "github", false, "Whether to output format in github action friendly format")
	projectCmd.AddCommand(createProjectCmd)
	rootCmd.AddCommand(projectCmd)
}

func createProject(ccmd *cobra.Command, args []string) {
	if !projectGithub {
		fmt.Println("Creating a project...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	if projectName == "" {
		log.Fatal("Empty project name")
	}

	if projectDescription == "" {
		log.Fatal("Empty project description")
	}

	body := api.CreateProjectJSONRequestBody{
		Name:        &projectName,
		Description: &projectDescription,
	}

	response, err := client.CreateProjectWithResponse(context.Background(), body)

	if err != nil {
		log.Fatal(err)
	}

	// Report the results back to the user
	success := response.HTTPResponse.StatusCode == http.StatusCreated
	if success {
		if projectGithub {
			fmt.Printf("project_id=%s\n", response.JSON201.ProjectID.String())
		} else {
			fmt.Println("Created project successfully!")
			fmt.Printf("Project ID: %s\n", response.JSON201.ProjectID.String())
		}
	} else {
		log.Fatal("Failed to create project!\n", string(response.Body))
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