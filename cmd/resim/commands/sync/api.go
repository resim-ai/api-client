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
type TagID = api.ExperienceTagID
type EnvironmentVariable = api.EnvironmentVariable

func getCurrentExperiencesByName(client api.ClientWithResponsesInterface,
	projectID uuid.UUID) *map[string]*Experience{
	archived := true
	unarchived := false
	apiExperiences := fetchAllExperiences(client, projectID, unarchived)
	apiArchivedExperiences := fetchAllExperiences(client, projectID, archived)
	currentExperiencesByName := make(map[string]*Experience)
	for _, experience := range apiExperiences {
		addApiExperienceToExperienceMap(experience, &currentExperiencesByName)
	}
	for _, experience := range apiArchivedExperiences {
		addApiExperienceToExperienceMap(experience, &currentExperiencesByName)
	}
	return &currentExperiencesByName
}

type TagSet struct {
	Name string
        TagID TagID
        ExperienceIDs []ExperienceID
}


func getCurrentTagSets(
        client api.ClientWithResponsesInterface,
	projectID uuid.UUID) *[]TagSet {
	apiExperienceTags := fetchAllExperienceTags(client, projectID)

        currentTagSets := []TagSet{}

	_ = apiExperienceTags
	

	return &currentTagSets
}







func addApiExperienceToExperienceMap(experience api.Experience,
	currentExperiences *map[string]*Experience) {
	(*currentExperiences)[experience.Name] = &Experience{
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

func updateSingleExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update *ExperienceMatch) {
	if update.Original == nil {
		// New Experience
		log.Print(update.New.Name)
		log.Print(update.New.ContainerTimeoutSeconds)
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
