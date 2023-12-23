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
		Use:    "create",
		Short:  "create - Creates a new experience",
		Long:   ``,
		Run:    createExperience,
		PreRun: RegisterViperFlagsAndSetClient,
	}
	listExperiencesCmd = &cobra.Command{
		Use:    "list",
		Short:  "list - Lists experiences",
		Long:   ``,
		Run:    listExperiences,
		PreRun: RegisterViperFlagsAndSetClient,
	}
	tagExperienceCmd = &cobra.Command{
		Use:    "tag",
		Short:  "tag - Add a tag to an experience",
		Long:   ``,
		Run:    tagExperience,
		PreRun: RegisterViperFlagsAndSetClient,
	}
	untagExperienceCmd = &cobra.Command{
		Use:    "untag",
		Short:  "untag - Remove a tag from an experience",
		Long:   ``,
		Run:    untagExperience,
		PreRun: RegisterViperFlagsAndSetClient,
	}
)

const (
	experienceNameKey          = "name"
	experienceIDKey            = "id"
	experienceDescriptionKey   = "description"
	experienceLocationKey      = "location"
	experienceLaunchProfileKey = "launch-profile"
	experienceGithubKey        = "github"
	experienceTagKey           = "tag"
)

func init() {
	createExperienceCmd.Flags().String(experienceNameKey, "", "The name of the experience")
	createExperienceCmd.MarkFlagRequired(experienceNameKey)
	createExperienceCmd.Flags().String(experienceDescriptionKey, "", "The description of the experience")
	createExperienceCmd.MarkFlagRequired(experienceDescriptionKey)
	createExperienceCmd.Flags().String(experienceLocationKey, "", "The location of the experience, e.g. an S3 URI for the experience folder")
	createExperienceCmd.MarkFlagRequired(experienceLocationKey)
	createExperienceCmd.Flags().String(experienceLaunchProfileKey, "", "The UUID of the launch profile for this experience")
	createExperienceCmd.Flags().Bool(experienceGithubKey, false, "Whether to output format in github action friendly format")
	experienceCmd.AddCommand(createExperienceCmd)
	experienceCmd.AddCommand(listExperiencesCmd)
	tagExperienceCmd.Flags().String(experienceTagKey, "", "The name of the tag to add")
	tagExperienceCmd.MarkFlagRequired(experienceTagKey)
	tagExperienceCmd.Flags().String(experienceIDKey, "", "The ID of the experience to tag")
	tagExperienceCmd.MarkFlagRequired(experienceNameKey)
	experienceCmd.AddCommand(tagExperienceCmd)
	untagExperienceCmd.Flags().String(experienceTagKey, "", "The name of the tag to remove")
	untagExperienceCmd.MarkFlagRequired(experienceTagKey)
	untagExperienceCmd.Flags().String(experienceIDKey, "", "The ID of the experience to untag")
	untagExperienceCmd.MarkFlagRequired(experienceNameKey)
	experienceCmd.AddCommand(untagExperienceCmd)
	rootCmd.AddCommand(experienceCmd)
}

func createExperience(ccmd *cobra.Command, args []string) {
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

	body := api.CreateExperienceJSONRequestBody{
		Name:        &experienceName,
		Description: &experienceDescription,
		Location:    &experienceLocation,
	}

	if viper.IsSet(experienceLaunchProfileKey) {
		experienceLaunchProfileString := viper.GetString(experienceLaunchProfileKey)
		if experienceLaunchProfileString == "" {
			log.Fatal("empty experience launch profile")
		}
		experienceLaunchProfile, err := uuid.Parse(experienceLaunchProfileString)
		if err != nil || experienceLaunchProfile == uuid.Nil {
			log.Fatal("failed to parse experience launch profile: ", err)
		}
		body.LaunchProfileID = &experienceLaunchProfile
	}

	response, err := Client.CreateExperienceWithResponse(context.Background(), body)
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

	// Report the results back to the user
	if experienceGithub {
		fmt.Printf("experience_id=%s\n", experience.ExperienceID.String())
	} else {
		fmt.Println("Created experience successfully!")
		fmt.Printf("Experience ID: %s\n", experience.ExperienceID.String())
	}
}

func listExperiences(ccmd *cobra.Command, args []string) {
	var pageToken *string = nil

	var allExperiences []api.Experience

	for {
		response, err := Client.ListExperiencesWithResponse(
			context.Background(), &api.ListExperiencesParams{
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
	experienceTagName := viper.GetString(experienceTagKey)
	if experienceTagName == "" {
		log.Fatal("empty experience tag name")
	}

	experienceID, err := uuid.Parse(viper.GetString(experienceIDKey))
	if err != nil || experienceID == uuid.Nil {
		log.Fatal("failed to parse experience ID: ", err)
	}

	experienceTagID := getExperienceTagIDForName(Client, experienceTagName)

	response, err := Client.AddExperienceTagToExperienceWithResponse(
		context.Background(),
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
	experienceTagName := viper.GetString(experienceTagKey)
	if experienceTagName == "" {
		log.Fatal("empty experience tag name")
	}

	experienceID, err := uuid.Parse(viper.GetString(experienceIDKey))
	if err != nil || experienceID == uuid.Nil {
		log.Fatal("failed to parse experience ID: ", err)
	}

	experienceTagID := getExperienceTagIDForName(Client, experienceTagName)
	response, err := Client.RemoveExperienceTagFromExperienceWithResponse(
		context.Background(),
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
