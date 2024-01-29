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
	"github.com/spf13/pflag"
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
		PreRun: RegisterViperFlagsAndSetClient,
	}

	deleteProjectCmd = &cobra.Command{
		Use:    "delete",
		Short:  "delete - Deletes a project",
		Long:   ``,
		Run:    deleteProject,
		PreRun: RegisterViperFlagsAndSetClient,
	}

	listProjectsCmd = &cobra.Command{
		Use:    "list",
		Short:  "list - Lists projects",
		Long:   ``,
		Run:    listProjects,
		PreRun: RegisterViperFlagsAndSetClient,
	}
)

const (
	projectKey            = "project"
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

	getProjectCmd.Flags().String(projectKey, "", "The name or the ID of the project")
	getProjectCmd.MarkFlagRequired(projectKey)
	getProjectCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	projectCmd.AddCommand(getProjectCmd)

	deleteProjectCmd.Flags().String(projectKey, "", "The name or the ID of the project to delete")
	deleteProjectCmd.MarkFlagRequired(projectKey)
	deleteProjectCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	projectCmd.AddCommand(deleteProjectCmd)

	projectCmd.AddCommand(listProjectsCmd)

	rootCmd.AddCommand(projectCmd)
}

func listProjects(ccmd *cobra.Command, args []string) {
	var pageToken *string = nil

	var allProjects []api.Project

	for {
		response, err := Client.ListProjectsWithResponse(
			context.Background(), &api.ListProjectsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list projects:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list projects", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		pageToken = response.JSON200.NextPageToken
		allProjects = append(allProjects, *response.JSON200.Projects...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(allProjects)
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
	// Because we allow users to pass both names and IDs to locate projects, we
	// need to protect the edge case that a user specifes the ID of one project as
	// the name of another.
	existingID := checkProjectID(Client, projectName)
	if existingID != uuid.Nil {
		log.Fatal("the specified project name matches an existing project's name or ID")
	}
	response, err := Client.CreateProjectWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal(err)
	}
	ValidateResponse(http.StatusCreated, "failed to create project", response.HTTPResponse, response.Body)
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
	if viper.IsSet(projectKey) {
		projectID := getProjectID(Client, viper.GetString(projectKey))
		response, err := Client.GetProjectWithResponse(context.Background(), projectID)
		if err != nil {
			log.Fatal("unable to retrieve project:", err)
		}
		if response.HTTPResponse.StatusCode == http.StatusNotFound {
			log.Fatal("failed to find project with requested id: ", projectID.String())
		} else {
			ValidateResponse(http.StatusOK, "unable to retrieve project", response.HTTPResponse, response.Body)
		}
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
	if viper.IsSet(projectKey) {
		projectID = getProjectID(Client, viper.GetString(projectKey))
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
		ValidateResponse(http.StatusNoContent, "unable to delete project", response.HTTPResponse, response.Body)
	}
	fmt.Println("Deleted project successfully!")
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func checkProjectID(client api.ClientWithResponsesInterface, identifier string) uuid.UUID {
	// Page through projects until we find the one with either a name or an ID
	// that matches the identifier string.
	projectID := uuid.Nil
	// First try the assumption that identifier is a UUID.
	err := uuid.Validate(identifier)
	if err == nil {
		// The identifier is a uuid - but does it refer to an existing project?
		potentialProjectID := uuid.MustParse(identifier)
		response, _ := client.GetProjectWithResponse(context.Background(), potentialProjectID)
		if response.HTTPResponse.StatusCode == http.StatusOK {
			// Project found with ID
			return potentialProjectID
		}
	}
	// If we're here then either the identifier is not a UUID or the UUID was not
	// found. Users could choose to name projects with UUIDs so regardless of how
	// we got here we now search for identifier as a string name.
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListProjectsWithResponse(
			context.Background(), &api.ListProjectsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list projects:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list projects", response.HTTPResponse, response.Body)
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
			if *project.Name == identifier {
				projectID = *project.ProjectID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return projectID
}

func getProjectID(client api.ClientWithResponsesInterface, identifier string) uuid.UUID {
	projectID := checkProjectID(client, identifier)
	if projectID == uuid.Nil {
		log.Fatal("failed to find project with name or ID: ", identifier)
	}
	return projectID
}

func aliasProjectNameFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "name":
		name = "project"
	case "project-id":
		name = "project"
	}
	return pflag.NormalizedName(name)
}
