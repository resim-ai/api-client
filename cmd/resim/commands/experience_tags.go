package commands

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
)

// This function takes a comma-separated list of experience tag names represented as a string
// and returns a separated array of parsed UUIDs.
func parseExperienceTagNames(client *api.ClientWithResponses, commaSeparatedNames string) []uuid.UUID {
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
func getExperienceTagIDForName(client *api.ClientWithResponses, experienceTagName string) uuid.UUID {
	// Page through experience tags until we find the one we want:
	var experienceTagID uuid.UUID = uuid.Nil
	var pageToken *string = nil
pageLoop:
	for {
		listResponse, err := client.ListExperienceTagsWithResponse(
			context.Background(), &api.ListExperienceTagsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("unable to list experience tags:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list experience tags", listResponse.HTTPResponse)
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
