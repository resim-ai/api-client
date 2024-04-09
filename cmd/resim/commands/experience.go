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

	addSystemCmd = &cobra.Command{
		Use:   "add-system",
		Short: "add-system - Add a system as compatible with an experience",
		Long:  ``,
		Run:   addSystem,
	}
	removeSystemCmd = &cobra.Command{
		Use:   "remove-system",
		Short: "remove-system - Remove a system as compatible with an experience",
		Long:  ``,
		Run:   removeSystem,
	}
)

const (
	experienceProjectKey       = "project"
	experienceSystemKey        = "system"
	experienceNameKey          = "name"
	experienceIDKey            = "id"
	experienceDescriptionKey   = "description"
	experienceLocationKey      = "location"
	experienceLaunchProfileKey = "launch-profile"
	experienceGithubKey        = "github"
	experienceTagKey           = "tag"
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
	experienceCmd.AddCommand(createExperienceCmd)

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
	addSystemCmd.Flags().String(experienceProjectKey, "", "The name or ID of the associated project")
	addSystemCmd.MarkFlagRequired(experienceProjectKey)
	addSystemCmd.Flags().String(experienceSystemKey, "", "The name of the system to add")
	addSystemCmd.MarkFlagRequired(experienceSystemKey)
	addSystemCmd.Flags().String(experienceNameKey, "", "The name of the experience to tag")
	addSystemCmd.MarkFlagRequired(experienceNameKey)
	experienceCmd.AddCommand(addSystemCmd)
	untagExperienceCmd.Flags().String(experienceProjectKey, "", "The name or ID of the associated project")
	untagExperienceCmd.MarkFlagRequired(experienceProjectKey)
	untagExperienceCmd.Flags().String(experienceSystemKey, "", "The name of the system to remove")
	untagExperienceCmd.MarkFlagRequired(experienceSystemKey)
	untagExperienceCmd.Flags().String(experienceNameKey, "", "The name of the experience to untag")
	untagExperienceCmd.MarkFlagRequired(experienceNameKey)
	experienceCmd.AddCommand(untagExperienceCmd)

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

	body := api.Experience{
		Name:        &experienceName,
		Description: &experienceDescription,
		Location:    &experienceLocation,
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
	if experience.ExperienceID == nil {
		log.Fatal("no experience ID")
	}

	validationResponse, err := Client.ValidateExperienceLocationWithResponse(context.Background(), api.ExperienceLocation{
		Location: experience.Location,
	})
	if err != nil {
		log.Fatal("could not validate experience after creation", err)
	}

	var objectsInExperience *[]string
	var objectsCount *int

	if validationResponse.JSON200 != nil {
		objectsInExperience = validationResponse.JSON200.Objects
		objectsCount = validationResponse.JSON200.ObjectCount
	}

	// Report the results back to the user
	if experienceGithub {
		fmt.Printf("experience_id=%s\n", experience.ExperienceID.String())
	} else {
		fmt.Println("Created experience successfully!")
		fmt.Printf("Experience ID: %s\n", experience.ExperienceID.String())
		if objectsCount != nil && *objectsCount > 0 {
			fmt.Printf("ReSim found %v file(s) in experience location:\n", *objectsCount)
			OutputJson(*objectsInExperience)
		} else {
			fmt.Println("WARNING: ReSim could not find any files in the provided location.")
		}
	}
}

func listExperiences(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	var pageToken *string = nil

	var allExperiences []api.Experience

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

func addSystem(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	if viper.GetString(experienceSystemKey) == "" {
		log.Fatal("empty system name")
	}
	systemID := getSystemID(Client, projectID, viper.GetString(experienceSystemKey), true)
	if viper.GetString(experienceNameKey) == "" {
		log.Fatal("empty experience name")
	}

	systemID := getExperienceID(Client, projectID, viper.GetString(experienceNameKey), true)
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
