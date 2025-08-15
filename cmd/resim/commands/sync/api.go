package sync

import (
	"context"
	"log"
	"net/http"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands/utils"	
	. "github.com/resim-ai/api-client/ptr"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
)

type EnvironmentVariable = api.EnvironmentVariable

func addApiExperienceToExperienceMap(experience api.Experience,
	currentExperiences *map[string]*Experience) {
	(*currentExperiences)[experience.Name] = &Experience{
		Name:                    experience.Name,
		Description:             experience.Description,
		Locations:               experience.Locations,
		Profile:                 &experience.Profile,
		ExperienceID:            Ptr(experience.ExperienceID.String()),
		EnvironmentVariables:    &experience.EnvironmentVariables,
		CacheExempt:             experience.CacheExempt,
		ContainerTimeoutSeconds: &experience.ContainerTimeoutSeconds,
		Archived:                experience.Archived,
	}
}

func GetCurrentExperiences(apiExperiences []api.Experience,
	apiArchivedExperiences []api.Experience) *map[string]*Experience {
	result := make(map[string]*Experience)
	for _, experience := range apiExperiences {
		addApiExperienceToExperienceMap(experience, &result)
	}
	for _, experience := range apiArchivedExperiences {
		addApiExperienceToExperienceMap(experience, &result)
	}
	return &result
}


func FetchAllExperiences(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	archived bool) []api.Experience {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperiencesWithResponse(
			context.Background(), projectID, &api.ListExperiencesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list experiences:", err)
		}
		utils.ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperiences
}

func FetchAllExperienceTags(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID) []api.ExperienceTag {
	allExperienceTags := []api.ExperienceTag{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperienceTagsWithResponse(
			context.Background(), projectID, &api.ListExperienceTagsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list experiences:", err)
		}
		utils.ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.ExperienceTags) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allExperienceTags = append(allExperienceTags, *response.JSON200.ExperienceTags...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperienceTags
}

func FetchAllSystems(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID) []api.System {
	allSystems := []api.System{}
	var pageToken *string = nil

	for {
		response, err := client.ListSystemsWithResponse(
			context.Background(), projectID, &api.ListSystemsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list experiences:", err)
		}
		utils.ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Systems) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allSystems = append(allSystems, *response.JSON200.Systems...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allSystems
}
