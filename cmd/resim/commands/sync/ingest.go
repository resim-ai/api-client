package sync

import (
	"context"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"log"
	"net/http"
)

type ExperienceID = api.ExperienceID
type SystemID = api.SystemID
type TagID = api.ExperienceTagID
type EnvironmentVariable = api.EnvironmentVariable

type TagSet struct {
	Name          string
	TagID         TagID
	ExperienceIDs map[ExperienceID]struct{}
}

type SystemSet struct {
	Name          string
	SystemID      SystemID
	ExperienceIDs map[ExperienceID]struct{}
}

type DatabaseState struct {
	ExperiencesByName map[string]*Experience
	TagSetsByName     map[string]TagSet
	SystemSetsByName  map[string]SystemSet
}

func getCurrentDatabaseState(client api.ClientWithResponsesInterface,
	projectID uuid.UUID) DatabaseState {
	expCh := make(chan map[string]*Experience)
	tagCh := make(chan map[string]TagSet)
	sysCh := make(chan map[string]SystemSet)

	go func() { expCh <- getCurrentExperiencesByName(client, projectID) }()
	go func() { tagCh <- getCurrentTagSetsByName(client, projectID) }()
	go func() { sysCh <- getCurrentSystemSetsByName(client, projectID) }()

	state := DatabaseState{
		ExperiencesByName: <-expCh,
		TagSetsByName:     <-tagCh,
		SystemSetsByName:  <-sysCh,
	}

	// Update the tags in each experience
	for _, experience := range state.ExperiencesByName {
		if experience.Archived {
			continue
		}
		for tag, tagSet := range state.TagSetsByName {
			if _, has_tag := tagSet.ExperienceIDs[experience.ExperienceID.ID]; has_tag {
				experience.Tags = append(experience.Tags, tag)
			}
		}
		for system, systemSet := range state.SystemSetsByName {
			if _, has_system := systemSet.ExperienceIDs[experience.ExperienceID.ID]; has_system {
				experience.Systems = append(experience.Systems, system)
			}
		}
	}
	return state
}

func getCurrentExperiencesByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) map[string]*Experience {
	archived := true
	unarchived := false
	apiExperiences := fetchAllExperiences(client, projectID, unarchived)
	apiArchivedExperiences := fetchAllExperiences(client, projectID, archived)
	currentExperiencesByName := make(map[string]*Experience)
	for _, experience := range apiExperiences {
		addApiExperienceToExperienceMap(experience, currentExperiencesByName)
	}
	for _, experience := range apiArchivedExperiences {
		addApiExperienceToExperienceMap(experience, currentExperiencesByName)
	}
	return currentExperiencesByName
}

func getCurrentTagSetsByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) map[string]TagSet {
	apiExperienceTags := fetchAllExperienceTags(client, projectID)

	currentTagSets := make(map[string]TagSet)

	for _, tag := range apiExperienceTags {
		archived := true
		unarchived := false
		apiExperiences := fetchAllExperiencesWithTag(client, projectID, tag.ExperienceTagID, unarchived)
		apiArchivedExperiences := fetchAllExperiencesWithTag(client, projectID, tag.ExperienceTagID, archived)
		apiExperiences = append(apiExperiences, apiArchivedExperiences...)

		experienceIDs := make(map[ExperienceID]struct{})
		for _, experience := range apiExperiences {
			experienceIDs[experience.ExperienceID] = struct{}{}
		}
		currentTagSets[tag.Name] = TagSet{
			Name:          tag.Name,
			TagID:         tag.ExperienceTagID,
			ExperienceIDs: experienceIDs,
		}
	}
	return currentTagSets
}

func getCurrentSystemSetsByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) map[string]SystemSet {
	apiSystems := fetchAllSystems(client, projectID)

	currentSystemSets := make(map[string]SystemSet)

	for _, system := range apiSystems {
		archived := true
		unarchived := false
		apiExperiences := fetchAllExperiencesWithSystem(client, projectID, system.SystemID, unarchived)
		apiArchivedExperiences := fetchAllExperiencesWithSystem(client, projectID, system.SystemID, archived)
		apiExperiences = append(apiExperiences, apiArchivedExperiences...)

		experienceIDs := make(map[ExperienceID]struct{})
		for _, experience := range apiExperiences {
			experienceIDs[experience.ExperienceID] = struct{}{}
		}
		currentSystemSets[system.Name] = SystemSet{
			Name:          system.Name,
			SystemID:      system.SystemID,
			ExperienceIDs: experienceIDs,
		}
	}
	return currentSystemSets
}

func addApiExperienceToExperienceMap(experience api.Experience,
	currentExperiences map[string]*Experience) {
	currentExperiences[experience.Name] = &Experience{
		Name:                    experience.Name,
		Description:             experience.Description,
		Locations:               experience.Locations,
		Profile:                 &experience.Profile,
		ExperienceID:            &ExperienceIDWrapper{ID: experience.ExperienceID},
		EnvironmentVariables:    &experience.EnvironmentVariables,
		CacheExempt:             experience.CacheExempt,
		ContainerTimeoutSeconds: &experience.ContainerTimeoutSeconds,
		Archived:                experience.Archived,
	}
}

func fetchAllExperiences(client api.ClientWithResponsesInterface,
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

func fetchAllExperienceTags(client api.ClientWithResponsesInterface,
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

func fetchAllSystems(client api.ClientWithResponsesInterface,
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

func fetchAllExperiencesWithTag(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	tagID TagID,
	archived bool) []api.Experience {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperiencesWithExperienceTagWithResponse(
			context.Background(), projectID, tagID, &api.ListExperiencesWithExperienceTagParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
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

func fetchAllExperiencesWithSystem(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	systemID SystemID,
	archived bool) []api.Experience {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperiencesForSystemWithResponse(
			context.Background(), projectID, systemID, &api.ListExperiencesForSystemParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
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
