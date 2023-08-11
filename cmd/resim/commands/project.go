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
		log.Fatal("empty project name")
	}

	if projectDescription == "" {
		log.Fatal("empty project description")
	}

	body := api.CreateProjectJSONRequestBody{
		Name:        &projectName,
		Description: &projectDescription,
	}

	response, err := client.CreateProjectWithResponse(context.Background(), body)
	if err != nil || response.StatusCode() != http.StatusCreated {
		log.Fatal("failed to create project", err, string(response.Body))
	}
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	project := *response.JSON201
	if project.ProjectID == nil {
		log.Fatal("empty project ID")
	}

	// Report the results back to the user
	if projectGithub {
		fmt.Printf("project_id=%s\n", project.ProjectID.String())
	} else {
		fmt.Println("Created project successfully!")
		fmt.Printf("Project ID: %s\n", project.ProjectID.String())
	}
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func getProjectIDForName(client *api.ClientWithResponses, buildProjectName string) uuid.UUID {
	// Page through projects until we find the one we want:
	var projectID uuid.UUID = uuid.Nil
	var pageToken *string = nil
pageLoop:
	for {
		listResponse, err := client.ListProjectsWithResponse(
			context.Background(), &api.ListProjectsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil || listResponse.StatusCode() != http.StatusOK {
			log.Fatal("failed to list projects: ", err, string(listResponse.Body))
		}
		if listResponse.JSON200 == nil {
			log.Fatal("empty response")
		}

		pageToken = listResponse.JSON200.NextPageToken
		projects := *listResponse.JSON200.Projects
		for _, project := range projects {
			if project.Name == nil {
				log.Fatal("project has no name")
			}
			if project.ProjectID == nil {
				log.Fatal("project ID is empty")
			}
			if *project.Name == buildProjectName {
				projectID = *project.ProjectID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	if projectID == uuid.Nil {
		log.Fatal("failed to find project with requested name: ", buildProjectName)
	}
	return projectID
}
