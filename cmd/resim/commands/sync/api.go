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
	"sync"
)

type ExperienceID = api.ExperienceID
type TagID = api.ExperienceTagID
type EnvironmentVariable = api.EnvironmentVariable

func getCurrentExperiencesByName(client api.ClientWithResponsesInterface,
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

type TagSet struct {
	Name          string
	TagID         TagID
	ExperienceIDs map[ExperienceID]struct{}
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

func updateSingleExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update *ExperienceMatch) {
	if update.Original == nil {
		// New Experience
		body := api.CreateExperienceInput{
			Name:                    update.New.Name,
			Description:             update.New.Description,
			Locations:               &update.New.Locations,
			ContainerTimeoutSeconds: update.New.ContainerTimeoutSeconds,
			Profile:                 update.New.Profile,
			EnvironmentVariables:    update.New.EnvironmentVariables,
			CacheExempt:             &update.New.CacheExempt,
		}

		response, err := client.CreateExperienceWithResponse(context.Background(), projectID, body)
		if err != nil {
			log.Print("WARNING: failed to create experience: ", err)
		}
		err = utils.ValidateResponseSafe(http.StatusCreated, "failed to create experience", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING:", err)
		}

		update.New.ExperienceID = &ExperienceIDWrapper{ID: response.JSON201.ExperienceID}
		return
	}
	if update.New.ExperienceID == nil {
		log.Fatalf("Trying to update with unset experience ID")
	}
	experienceID := update.New.ExperienceID.ID
	if update.New.Archived {
		// Archive
		response, err := client.ArchiveExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			log.Print("WARNING: failed to archive experience: ", err)
		}
		err = utils.ValidateResponseSafe(http.StatusNoContent, "failed to archive experience", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING: ", err)
		}

		return
	}
	// Update
	if update.Original.Archived {
		// Restore
		response, err := client.RestoreExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			log.Print("WARNING: failed to restore experience: ", err)
		}
		err = utils.ValidateResponseSafe(http.StatusNoContent, "failed to restore experience", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING:", err)
		}
	}
	updateMask := []string{"name", "description", "cacheExempt", "locations"}

	if update.New.ContainerTimeoutSeconds != nil {
		updateMask = append(updateMask, "containerTimeoutSeconds")
	}
	if update.New.Profile != nil {
		updateMask = append(updateMask, "profile")
	}
	if update.New.EnvironmentVariables != nil {
		updateMask = append(updateMask, "environmentVariables")
	}

	body := api.UpdateExperienceInput{
		Experience: &api.UpdateExperienceFields{
			Name:                    &update.New.Name,
			Description:             &update.New.Description,
			Locations:               &update.New.Locations,
			ContainerTimeoutSeconds: update.New.ContainerTimeoutSeconds,
			Profile:                 update.New.Profile,
			EnvironmentVariables:    update.New.EnvironmentVariables,
			CacheExempt:             &update.New.CacheExempt,
		},
		UpdateMask: &updateMask,
	}
	response, err := client.UpdateExperienceWithResponse(context.Background(), projectID, experienceID, body)
	if err != nil {
		log.Print("WARNING: failed to update experience: ", err)
	}
	err = utils.ValidateResponseSafe(http.StatusOK, "failed to update experience", response.HTTPResponse, response.Body)
	if err != nil {
		log.Print("WARNING:", err)
	}

}

func asyncRemoveTagFromExperience(
	wg *sync.WaitGroup,
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	tagID TagID,
	experienceID ExperienceID) {
	defer wg.Done()
	response, err := client.RemoveExperienceTagFromExperienceWithResponse(
		context.Background(),
		projectID,
		tagID,
		experienceID,
	)
	if err != nil {
		log.Print("WARNING: failed to update tags: ", err)
	}
	err = utils.ValidateResponseSafe(http.StatusNoContent, "failed to update tags", response.HTTPResponse, response.Body)
	if err != nil {
		log.Print("WARNING:", err)
	}

}

func asyncUpdateSingleTag(
	wg *sync.WaitGroup,
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates *TagUpdates) {
	defer wg.Done()

	if len(updates.Additions) > 0 {
		experienceIDs := []ExperienceID{}
		for _, e := range updates.Additions {
			if e.ExperienceID == nil {
				log.Fatal("Experience has no ID. Maybe we failed to create it? ", e.Name)
			}
			experienceIDs = append(experienceIDs, e.ExperienceID.ID)
		}

		body := api.AddTagsToExperiencesInput{
			ExperienceTagIDs: []TagID{updates.TagID},
			Experiences:      &experienceIDs,
		}
		response, err := client.AddTagsToExperiencesWithResponse(context.Background(), projectID, body)
		if err != nil {
			log.Print("WARNING: failed to update tags: ", err)
		}
		err = utils.ValidateResponseSafe(http.StatusCreated, "failed to update tags", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING:", err)
		}
	}

	// Would be much better with a single endpoint for this
	for _, removal := range updates.Removals {
		wg.Add(1)
		go asyncRemoveTagFromExperience(wg, client, projectID,
			updates.TagID,
			removal.ExperienceID.ID)
	}

}
