package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	projectCmd = &cobra.Command{
		Use:     "projects",
		Short:   "projects contains commands for creating and managing projects",
		Long:    ``,
		Aliases: []string{"project"},
	}
	createProjectCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new project",
		Long:   ``,
		Run:    createProject,
		PreRun: RegisterViperFlagsAndSetClient,
	}

	getProjectCmd = &cobra.Command{
		Use:    "get",
		Short:  "get - Gets details about a project",
		Long:   ``,
		Run:    getProject,
		PreRun: RegisterViperFlags,
	}

	deleteProjectCmd = &cobra.Command{
		Use:    "delete",
		Short:  "delete - Deletes a project",
		Long:   ``,
		Run:    deleteProject,
		PreRun: RegisterViperFlags,
	}
)

const (
	projectIDKey          = "project-id"
	projectNameKey        = "name"
	projectDescriptionKey = "description"
	projectGithubKey      = "github"
)

func init() {
	createProjectCmd.Flags().String(projectNameKey, "", "The name of the project, often a repository name")
	createProjectCmd.MarkFlagRequired(projectNameKey)
	createProjectCmd.Flags().String(projectDescriptionKey, "", "The description of the project")
	createProjectCmd.MarkFlagRequired(projectDescriptionKey)
	createProjectCmd.Flags().Bool(projectGithubKey, false, "Whether to output format in github action friendly format")
	projectCmd.AddCommand(createProjectCmd)

	getProjectCmd.Flags().String(projectIDKey, "", "The ID of the project to get")
	getProjectCmd.Flags().String(projectNameKey, "", "The Name of the project to get (e.g. my-project)")
	getProjectCmd.MarkFlagsMutuallyExclusive(projectIDKey, projectNameKey)
	projectCmd.AddCommand(getProjectCmd)

	deleteProjectCmd.Flags().String(projectIDKey, "", "The ID of the project to delete")
	deleteProjectCmd.Flags().String(projectNameKey, "", "The Name of the project to delete (e.g. my-project)")
	deleteProjectCmd.MarkFlagsMutuallyExclusive(projectIDKey, projectNameKey)
	projectCmd.AddCommand(deleteProjectCmd)

	rootCmd.AddCommand(projectCmd)
}

func createProject(ccmd *cobra.Command, args []string) {
	projectGithub := viper.GetBool(projectGithubKey)
	if !projectGithub {
		fmt.Println("Creating a project...")
	}

	// Parse the various arguments from command line
	projectName := viper.GetString(projectNameKey)
	if projectName == "" {
		log.Fatal("empty project name")
	}

	projectDescription := viper.GetString(projectDescriptionKey)
	if projectDescription == "" {
		log.Fatal("empty project description")
	}

	body := api.CreateProjectJSONRequestBody{
		Name:        &projectName,
		Description: &projectDescription,
	}

	response, err := Client.CreateProjectWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal(err)
	}
	ValidateResponse(http.StatusCreated, "failed to create project", response.HTTPResponse)
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

func getProject(ccmd *cobra.Command, args []string) {
	var project *api.Project
	if viper.IsSet(projectIDKey) {
		projectID, err := uuid.Parse(viper.GetString(projectIDKey))
		if err != nil {
			log.Fatal("unable to parse project ID: ", err)
		}
		response, err := Client.GetProjectWithResponse(context.Background(), projectID)
		if err != nil {
			log.Fatal("unable to retrieve project:", err)
		}
		if response.HTTPResponse.StatusCode == http.StatusNotFound {
			log.Fatal("failed to find project with requested id: ", projectID.String())
		} else {
			ValidateResponse(http.StatusOK, "unable to retrieve project", response.HTTPResponse)
		}
		project = response.JSON200
	} else if viper.IsSet(projectNameKey) {
		projectName := viper.GetString(projectNameKey)
		projectID := getProjectIDForName(Client, projectName)
		response, err := Client.GetProjectWithResponse(context.Background(), projectID)
		if err != nil {
			log.Fatal("unable to retrieve project:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve project", response.HTTPResponse)
		project = response.JSON200
	} else {
		log.Fatal("must specify either the project ID or the project name")
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	enc.Encode(project)
}

func deleteProject(ccmd *cobra.Command, args []string) {
	var projectID uuid.UUID
	if viper.IsSet(projectIDKey) {
		projectID = uuid.MustParse(viper.GetString(projectIDKey))
	} else if viper.IsSet(projectNameKey) {
		projectName := viper.GetString(projectNameKey)
		projectID = getProjectIDForName(Client, projectName)
	} else {
		log.Fatal("must specify either the project ID or the project name")
	}
	response, err := Client.DeleteProjectWithResponse(context.Background(), projectID)
	if err != nil {
		log.Fatal("unable to delete project:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatal("failed to delete project. No project exists with requested id: ", projectID.String())
	} else {
		ValidateResponse(http.StatusNoContent, "unable to delete project", response.HTTPResponse)
	}
	fmt.Println("Delete project successfully!")
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func getProjectIDForName(client api.ClientWithResponsesInterface, projectName string) uuid.UUID {
	// Page through projects until we find the one we want:
	var projectID uuid.UUID = uuid.Nil
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListProjectsWithResponse(
			context.Background(), &api.ListProjectsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list projects:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list projects", response.HTTPResponse)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}

		pageToken = response.JSON200.NextPageToken
		projects := *response.JSON200.Projects
		for _, project := range projects {
			if project.Name == nil {
				log.Fatal("project has no name")
			}
			if project.ProjectID == nil {
				log.Fatal("project ID is empty")
			}
			if *project.Name == projectName {
				projectID = *project.ProjectID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	if projectID == uuid.Nil {
		log.Fatal("failed to find project with requested name: ", projectName)
	}
	return projectID
}
