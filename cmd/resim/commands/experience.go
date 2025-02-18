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
	experienceCmd = &cobra.Command{
		Use:     "experiences",
		Short:   "experiences contains commands for creating and managing experiences",
		Long:    ``,
		Aliases: []string{"experience"},
	}
	createExperienceCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new experience",
		Long:  ``,
		Run:   createExperience,
	}
	getExperienceCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Get information about an experience",
		Long:  ``,
		Run:   getExperience,
	}
	listExperiencesCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists experiences",
		Long:  ``,
		Run:   listExperiences,
	}
	tagExperienceCmd = &cobra.Command{
		Use:   "tag",
		Short: "tag - Add a tag to an experience",
		Long:  ``,
		Run:   tagExperience,
	}
	untagExperienceCmd = &cobra.Command{
		Use:   "untag",
		Short: "untag - Remove a tag from an experience",
		Long:  ``,
		Run:   untagExperience,
	}

	addSystemExperienceCmd = &cobra.Command{
		Use:   "add-system",
		Short: "add-system - Add a system as compatible with an experience",
		Long:  ``,
		Run:   addSystemToExperience,
	}
	removeSystemExperienceCmd = &cobra.Command{
		Use:   "remove-system",
		Short: "remove-system - Remove a system as compatible with an experience",
		Long:  ``,
		Run:   removeSystemFromExperience,
	}
)

const (
	experienceProjectKey       = "project"
	experienceSystemKey        = "system"
	experienceSystemsKey       = "systems"
	experienceNameKey          = "name"
	experienceKey              = "experience"
	experienceIDKey            = "id"
	experienceDescriptionKey   = "description"
	experienceLocationKey      = "location"
	experienceLaunchProfileKey = "launch-profile"
	experienceGithubKey        = "github"
	experienceTagKey           = "tag"
	experienceTimeoutKey       = "timeout"
)

func init() {
	createExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the project to associate with the experience")
	createExperienceCmd.MarkFlagRequired(experienceProjectKey)
	createExperienceCmd.Flags().String(experienceNameKey, "", "The name of the experience")
	createExperienceCmd.MarkFlagRequired(experienceNameKey)
	createExperienceCmd.Flags().String(experienceDescriptionKey, "", "The description of the experience")
	createExperienceCmd.MarkFlagRequired(experienceDescriptionKey)
	createExperienceCmd.Flags().String(experienceLocationKey, "", "The location of the experience, e.g. an S3 URI for the experience folder")
	createExperienceCmd.MarkFlagRequired(experienceLocationKey)
	createExperienceCmd.Flags().String(experienceLaunchProfileKey, "", "The UUID of the launch profile for this experience")
	createExperienceCmd.Flags().MarkDeprecated(experienceLaunchProfileKey, "launch profiles are deprecated in favor of systems to define resource requirements")
	createExperienceCmd.Flags().Bool(experienceGithubKey, false, "Whether to output format in github action friendly format")
	createExperienceCmd.Flags().StringSlice(experienceSystemsKey, []string{}, "A list of system names or IDs to register as compatible with the experience")
	createExperienceCmd.Flags().Int32(experienceTimeoutKey, 3600, "The timeout for the experience container in seconds. Default is 3600 (1 hour)")
	experienceCmd.AddCommand(createExperienceCmd)

	getExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the project to list the experiences within")
	getExperienceCmd.MarkFlagRequired(experienceProjectKey)
	getExperienceCmd.Flags().String(experienceKey, "", "The name or ID of the experience to get")
	getExperienceCmd.MarkFlagRequired(experienceKey)
	experienceCmd.AddCommand(getExperienceCmd)

	listExperiencesCmd.Flags().String(experienceProjectKey, "", "The name or ID of the project to list the experiences within")
	listExperiencesCmd.MarkFlagRequired(experienceProjectKey)
	experienceCmd.AddCommand(listExperiencesCmd)
	// Experience tag sub-commands:
	tagExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the associated project")
	tagExperienceCmd.MarkFlagRequired(experienceProjectKey)
	tagExperienceCmd.Flags().String(experienceTagKey, "", "The name of the tag to add")
	tagExperienceCmd.MarkFlagRequired(experienceTagKey)
	tagExperienceCmd.Flags().String(experienceIDKey, "", "The ID of the experience to tag")
	tagExperienceCmd.MarkFlagRequired(experienceNameKey)
	experienceCmd.AddCommand(tagExperienceCmd)

	untagExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the associated project")
	untagExperienceCmd.MarkFlagRequired(experienceProjectKey)
	untagExperienceCmd.Flags().String(experienceTagKey, "", "The name of the tag to remove")
	untagExperienceCmd.MarkFlagRequired(experienceTagKey)
	untagExperienceCmd.Flags().String(experienceIDKey, "", "The ID of the experience to untag")
	untagExperienceCmd.MarkFlagRequired(experienceNameKey)
	experienceCmd.AddCommand(untagExperienceCmd)
	// Systems-related sub-commands:
	addSystemExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the associated project")
	addSystemExperienceCmd.MarkFlagRequired(experienceProjectKey)
	addSystemExperienceCmd.Flags().String(experienceSystemKey, "", "The name or ID of the system to add")
	addSystemExperienceCmd.MarkFlagRequired(experienceSystemKey)
	addSystemExperienceCmd.Flags().String(experienceKey, "", "The name or ID of the experience register as compatible with the system")
	addSystemExperienceCmd.MarkFlagRequired(experienceKey)
	experienceCmd.AddCommand(addSystemExperienceCmd)
	removeSystemExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the associated project")
	removeSystemExperienceCmd.MarkFlagRequired(experienceProjectKey)
	removeSystemExperienceCmd.Flags().String(experienceSystemKey, "", "The name or ID  of the system to remove")
	removeSystemExperienceCmd.MarkFlagRequired(experienceSystemKey)
	removeSystemExperienceCmd.Flags().String(experienceKey, "", "The name or ID of the experience to deregister as compatible with the system")
	removeSystemExperienceCmd.MarkFlagRequired(experienceKey)
	experienceCmd.AddCommand(removeSystemExperienceCmd)

	rootCmd.AddCommand(experienceCmd)
}

func createExperience(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	experienceGithub := viper.GetBool(experienceGithubKey)
	if !experienceGithub {
		fmt.Println("Creating an experience...")
	}

	// Parse the various arguments from command line
	experienceName := viper.GetString(experienceNameKey)
	if experienceName == "" {
		log.Fatal("empty experience name")
	}

	experienceDescription := viper.GetString(experienceDescriptionKey)
	if experienceDescription == "" {
		log.Fatal("empty experience description")
	}

	experienceLocation := viper.GetString(experienceLocationKey)
	if experienceLocation == "" {
		log.Fatal("empty experience location")
	}

	containerTimeoutSeconds := viper.GetInt32(experienceTimeoutKey)

	body := api.CreateExperienceInput{
		Name:                    experienceName,
		Description:             experienceDescription,
		Location:                experienceLocation,
		ContainerTimeoutSeconds: &containerTimeoutSeconds,
	}

	if viper.IsSet(experienceLaunchProfileKey) {
		fmt.Println("Launch profiles are deprecated in favor of systems to define resource requirements, parameter will be ignored.")
	}

	response, err := Client.CreateExperienceWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create experience: ", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create experience", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	experience := response.JSON201
	if experience.ExperienceID == uuid.Nil {
		log.Fatal("no experience ID")
	}

	// For each system, add that system to the experience:
	systems := viper.GetStringSlice(experienceSystemsKey)
	for _, systemName := range systems {
		systemID := getSystemID(Client, projectID, systemName, true)
		_, err := Client.AddSystemToExperienceWithResponse(
			context.Background(), projectID,
			systemID,
			experience.ExperienceID,
		)
		if err != nil {
			log.Fatal("failed to register experience with system", err)
		}
	}

	validationResponse, err := Client.ValidateExperienceLocationWithResponse(context.Background(), api.ExperienceLocation{
		Location: Ptr(experience.Location),
	})
	if err != nil {
		log.Fatal("could not validate experience after creation", err)
	}

	var objectsInExperience *[]string
	var objectsCount *int
	var isCloud bool

	if validationResponse.JSON200 != nil {
		objectsInExperience = validationResponse.JSON200.Objects
		objectsCount = validationResponse.JSON200.ObjectCount
		isCloud = *validationResponse.JSON200.IsCloud
	}

	// Report the results back to the user
	if experienceGithub {
		fmt.Printf("experience_id=%s\n", experience.ExperienceID.String())
	} else {
		fmt.Println("Created experience successfully!")
		fmt.Printf("Experience ID: %s\n", experience.ExperienceID.String())
		if isCloud && objectsCount != nil && *objectsCount > 0 {
			fmt.Printf("ReSim found %v file(s) in experience location:\n", *objectsCount)
			OutputJson(*objectsInExperience)
		} else {
			fmt.Println("WARNING: ReSim could not find any files in the provided location.")
		}
	}
}

func getExperience(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	experienceID := getExperienceID(Client, projectID, viper.GetString(experienceKey), true)

	response, err := Client.GetExperienceWithResponse(context.Background(), projectID, experienceID)
	if err != nil {
		log.Fatal("failed to get experience:", err)
	}
	ValidateResponse(http.StatusOK, "failed to get experience", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response")
	}
	experience := response.JSON200
	OutputJson(experience)
}

func listExperiences(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	var allExperiences []api.Experience
	var pageToken *string = nil

	for {
		response, err := Client.ListExperiencesWithResponse(
			context.Background(), projectID, &api.ListExperiencesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list experiences:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			log.Fatal("no experiences")
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(allExperiences)
}

func tagExperience(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	experienceTagName := viper.GetString(experienceTagKey)
	if experienceTagName == "" {
		log.Fatal("empty experience tag name")
	}

	experienceID, err := uuid.Parse(viper.GetString(experienceIDKey))
	if err != nil || experienceID == uuid.Nil {
		log.Fatal("failed to parse experience ID: ", err)
	}

	experienceTagID := getExperienceTagIDForName(Client, projectID, experienceTagName)

	response, err := Client.AddExperienceTagToExperienceWithResponse(
		context.Background(), projectID,
		experienceTagID,
		experienceID,
	)
	if err != nil {
		log.Fatal("failed to tag experience", err)
	}
	if response.HTTPResponse.StatusCode == 409 {
		log.Fatal("failed to tag experience, it may already be tagged ", experienceTagName)
	}
	ValidateResponse(http.StatusCreated, "failed to tag experience", response.HTTPResponse, response.Body)
}

func untagExperience(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	experienceTagName := viper.GetString(experienceTagKey)
	if experienceTagName == "" {
		log.Fatal("empty experience tag name")
	}

	experienceID, err := uuid.Parse(viper.GetString(experienceIDKey))
	if err != nil || experienceID == uuid.Nil {
		log.Fatal("failed to parse experience ID: ", err)
	}

	experienceTagID := getExperienceTagIDForName(Client, projectID, experienceTagName)
	response, err := Client.RemoveExperienceTagFromExperienceWithResponse(
		context.Background(), projectID,
		experienceTagID,
		experienceID,
	)
	if err != nil {
		log.Fatal("failed to untag experience: ", err)
	}
	if response.HTTPResponse.StatusCode == 404 {
		log.Fatal("failed to untag experience, it may not be tagged ", experienceTagName)
	}
	ValidateResponse(http.StatusNoContent, "failed to untag experience", response.HTTPResponse, response.Body)
}

func addSystemToExperience(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))

	systemName := viper.GetString(experienceSystemKey)
	if systemName == "" {
		log.Fatal("empty system name")
	}
	systemID := getSystemID(Client, projectID, systemName, true)

	if viper.GetString(experienceKey) == "" {
		log.Fatal("empty experience name")
	}
	experienceID := getExperienceID(Client, projectID, viper.GetString(experienceKey), true)

	response, err := Client.AddSystemToExperienceWithResponse(
		context.Background(), projectID,
		systemID,
		experienceID,
	)
	if err != nil {
		log.Fatal("failed to register experience with system", err)
	}
	if response.HTTPResponse.StatusCode == 409 {
		log.Fatal("failed to register experience with system, it may already be registered ", systemName)
	}
	ValidateResponse(http.StatusCreated, "failed to register experience with system", response.HTTPResponse, response.Body)
}

func removeSystemFromExperience(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))

	systemName := viper.GetString(experienceSystemKey)
	if systemName == "" {
		log.Fatal("empty system name")
	}
	systemID := getSystemID(Client, projectID, systemName, true)
	if viper.GetString(experienceKey) == "" {
		log.Fatal("empty experience name")
	}
	experienceID := getExperienceID(Client, projectID, viper.GetString(experienceKey), true)

	response, err := Client.RemoveSystemFromExperienceWithResponse(
		context.Background(), projectID,
		systemID,
		experienceID,
	)
	if err != nil {
		log.Fatal("failed to deregister experience with system", err)
	}
	if response.HTTPResponse.StatusCode == 409 {
		log.Fatal("failed to deregister experience with system, it may not be registered ", systemName)
	}
	ValidateResponse(http.StatusNoContent, "failed to deregister experience with system", response.HTTPResponse, response.Body)
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func checkExperienceID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string) uuid.UUID {
	// Page through experiences until we find the one with either a name or an ID
	// that matches the identifier string.
	experienceID := uuid.Nil
	// First try the assumption that identifier is a UUID.
	err := uuid.Validate(identifier)
	if err == nil {
		// The identifier is a uuid - but does it refer to an existing experience?
		potentialExperienceID := uuid.MustParse(identifier)
		response, _ := client.GetExperienceWithResponse(context.Background(), projectID, potentialExperienceID)
		if response.HTTPResponse.StatusCode == http.StatusOK {
			// Experience found with ID
			return potentialExperienceID
		}
	}
	// If we're here then either the identifier is not a UUID or the UUID was not
	// found. Users could choose to name experiences with UUIDs so regardless of how
	// we got here we now search for identifier as a string name.
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListExperiencesWithResponse(
			context.Background(), projectID, &api.ListExperiencesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list experiences:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		pageToken = response.JSON200.NextPageToken
		experiences := *response.JSON200.Experiences
		for _, experience := range experiences {
			if experience.Name == "" {
				log.Fatal("experience has no name")
			}
			if experience.ExperienceID == uuid.Nil {
				log.Fatal("experience ID is empty")
			}
			if experience.Name == identifier {
				experienceID = experience.ExperienceID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return experienceID
}

func getExperienceID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string, failWhenNotFound bool) uuid.UUID {
	experienceID := checkExperienceID(client, projectID, identifier)
	if experienceID == uuid.Nil && failWhenNotFound {
		log.Fatal("failed to find experience with name or ID: ", identifier)
	}
	return experienceID
}
