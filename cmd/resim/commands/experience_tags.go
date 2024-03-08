package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	experienceTagCmd = &cobra.Command{
		Use:     "experience-tags",
		Short:   "experience-tags contains commands for creating and managing experience tags",
		Long:    ``,
		Aliases: []string{"experience-tag"},
	}
	createExperienceTagCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new experience tag",
		Long:  ``,
		Run:   createExperienceTag,
	}
	listExperienceTagsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List experience tags",
		Long:  ``,
		Run:   listExperienceTags,
	}
	listExperiencesWithTagCmd = &cobra.Command{
		Use:   "list-experiences",
		Short: "list-experiences - Lists the experiences for a tag",
		Long:  ``,
		Run:   listExperiencesWithTag,
	}
)

const (
	experienceTagNameKey        = "name"
	experienceTagDescriptionKey = "description"
	experienceTagExperiencesKey = "experiences"
)

func init() {
	createExperienceTagCmd.Flags().String(experienceTagNameKey, "", "The name of the experience tag")
	createExperienceTagCmd.MarkFlagRequired(experienceTagNameKey)
	createExperienceTagCmd.Flags().String(experienceTagDescriptionKey, "", "The description of the experience tag")
	createExperienceTagCmd.MarkFlagRequired(experienceTagDescriptionKey)
	createExperienceTagCmd.Flags().String(experienceTagExperiencesKey, "", "Which experiences to add to this tag on tag creation")
	experienceTagCmd.AddCommand(createExperienceTagCmd)
	experienceTagCmd.AddCommand(listExperienceTagsCmd)
	listExperiencesWithTagCmd.Flags().String(experienceTagNameKey, "", "The name of the experience tag")
	listExperiencesWithTagCmd.MarkFlagRequired(experienceTagNameKey)
	experienceTagCmd.AddCommand(listExperiencesWithTagCmd)
	rootCmd.AddCommand(experienceTagCmd)
}

func createExperienceTag(ccmd *cobra.Command, args []string) {
	experienceTagName := viper.GetString(experienceTagNameKey)
	if experienceTagName == "" {
		log.Fatal("empty experience tag name")
	}

	experienceTagDescription := viper.GetString(experienceTagDescriptionKey)
	if experienceTagDescription == "" {
		log.Fatal("empty experience tag description")
	}

	// add experiences if they are set

	body := api.ExperienceTag{
		Name:        &experienceTagName,
		Description: &experienceTagDescription,
	}

	response, err := Client.CreateExperienceTagWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal("failed to create experience tag: ", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create experience tag", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	experienceTag := response.JSON201
	if experienceTag.ExperienceTagID == nil {
		log.Fatal("no experience tag ID")
	}

	fmt.Println("Created experience tag")
	fmt.Printf("Experience Tag: %s\n", *experienceTag.Name)
}

func listExperienceTags(ccmd *cobra.Command, args []string) {
	var pageToken *string = nil
	var experienceTags []api.ExperienceTag

	for {
		response, err := Client.ListExperienceTagsWithResponse(context.Background(),
			&api.ListExperienceTagsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list experience tags: ", err)
		}
		ValidateResponse(http.StatusOK, "failed to list experience tags", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.ExperienceTags) == 0 {
			log.Fatal("no experience tags")
		}
		experienceTags = append(experienceTags, *response.JSON200.ExperienceTags...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(experienceTags)
}

func listExperiencesWithTag(ccmd *cobra.Command, args []string) {
	experienceTagName := viper.GetString(experienceTagNameKey)
	if experienceTagName == "" {
		log.Fatal("empty experience tag name")
	}

	experienceTagID := getExperienceTagIDForName(Client, experienceTagName)

	var pageToken *string = nil
	var experiences []api.Experience

	for {
		response, err := Client.ListExperiencesWithExperienceTagWithResponse(
			context.Background(),
			experienceTagID,
			&api.ListExperiencesWithExperienceTagParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list experiences: ", err)
		}
		ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			log.Fatal("no experiences in tag ", experienceTagName)
		}
		experiences = append(experiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(experiences)
}

// This function takes a comma-separated list of experience tag names represented as a string
// and returns a separated array of parsed UUIDs.
func parseExperienceTagNames(client api.ClientWithResponsesInterface, commaSeparatedNames string) []uuid.UUID {
	if commaSeparatedNames == "" {
		return []uuid.UUID{}
	}
	strs := strings.Split(commaSeparatedNames, ",")
	result := make([]uuid.UUID, len(strs))

	for i := 0; i < len(strs); i++ {
		id := getExperienceTagIDForName(client, strings.TrimSpace(strs[i]))
		result[i] = id
	}
	return result
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func getExperienceTagIDForName(client api.ClientWithResponsesInterface, experienceTagName string) uuid.UUID {
	// Page through experience tags until we find the one we want:
	var experienceTagID uuid.UUID = uuid.Nil
	var pageToken *string = nil
pageLoop:
	for {
		listResponse, err := client.ListExperienceTagsWithResponse(
			context.Background(), &api.ListExperienceTagsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("unable to list experience tags:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list experience tags", listResponse.HTTPResponse, listResponse.Body)
		if listResponse.JSON200 == nil {
			log.Fatal("empty response")
		}

		pageToken = listResponse.JSON200.NextPageToken
		experienceTags := *listResponse.JSON200.ExperienceTags
		for _, experienceTag := range experienceTags {
			if experienceTag.Name == nil {
				log.Fatal("experience tag has no name")
			}
			if experienceTag.ExperienceTagID == nil {
				log.Fatal("experience tag ID is empty")
			}
			if *experienceTag.Name == experienceTagName {
				experienceTagID = *experienceTag.ExperienceTagID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	if experienceTagID == uuid.Nil {
		log.Fatal("failed to find experience tag with requested name: ", experienceTagName)
	}
	return experienceTagID
}
