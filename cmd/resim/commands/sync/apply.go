package sync

import (
	"context"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands/utils"
	"github.com/schollz/progressbar/v3"
	"log"
	"maps"
	"net/http"
	"slices"
	"sync"
)

// Apply the given ExperienceUpdates to the backend by calling the relevant endpoints.
func applyUpdates(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	experienceUpdates ExperienceUpdates) {

	numWorkers := 16
	runConcurrentUpdates("Experiences", slices.Collect(maps.Values(experienceUpdates.MatchedExperiencesByNewName)),
		numWorkers,
		func(update ExperienceMatch) {
			updateSingleExperience(client, projectID, update)
		})

	runConcurrentUpdates("Tags", slices.Collect(maps.Values(experienceUpdates.TagUpdatesByName)),
		numWorkers,
		func(update *TagUpdates) {
			updateSingleTag(client, projectID, *update)
		})

	runConcurrentUpdates("Systems", slices.Collect(maps.Values(experienceUpdates.SystemUpdatesByName)),
		numWorkers,
		func(update *SystemUpdates) {
			updateSingleSystem(client, projectID, *update)
		})

	runConcurrentUpdates("Test Suites", experienceUpdates.TestSuiteUpdates,
		numWorkers,
		func(update TestSuiteUpdate) {
			updateSingleTestSuite(client, projectID, update)
		})
}

// Helper to parallelize our updates and track progress with a bar
func runConcurrentUpdates[T any](
	label string,
	items []T,
	numWorkers int,
	task func(T),
) {
	if len(items) == 0 {
		return
	}

	log.Printf("Updating %s...", label)
	bar := progressbar.Default(int64(len(items)))

	var wg sync.WaitGroup
	inputsCh := make(chan T, len(items))

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range inputsCh {
				task(item)
				bar.Add(1)
			}
		}()
	}

	// Feed items
	for _, item := range items {
		inputsCh <- item
	}
	close(inputsCh)

	wg.Wait()
}

func updateSingleExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update ExperienceMatch) {

	newExperience := update.Original == nil
	if newExperience {
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
			log.Fatal("failed to create experience: ", err)
		}
		utils.ValidateResponse(http.StatusCreated, "failed to create experience", response.HTTPResponse, response.Body)

		// Update the match with then new experience ID. This way the other updates (which
		// reference this by pointer) will have the ID.
		update.New.ExperienceID = &ExperienceIDWrapper{ID: response.JSON201.ExperienceID}
		return
	}
	if update.New.ExperienceID == nil {
		// Fatal since this should *never* happen. It's a bug in the api client if so.
		log.Fatal("Trying to update with unset experience ID")
	}

	experienceID := update.New.ExperienceID.ID
	if update.New.Archived {
		// Archive
		response, err := client.ArchiveExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			log.Fatal("failed to archive experience: ", err)
		}
		utils.ValidateResponse(http.StatusNoContent, "failed to archive experience", response.HTTPResponse, response.Body)

		return
	}
	// Update
	if update.Original.Archived {
		// Restore
		response, err := client.RestoreExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			log.Fatal("failed to restore experience: ", err)
		}
		utils.ValidateResponse(http.StatusNoContent, "failed to restore experience", response.HTTPResponse, response.Body)
	}
	updateMask := []string{"name", "description", "cacheExempt", "locations"}

	// These are only updated if they're included. Otherwise they retain their current
	// value. Users should probably include them anyway.
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
		log.Fatal("failed to update experience: ", err)
	}
	utils.ValidateResponse(http.StatusOK, "failed to update experience", response.HTTPResponse, response.Body)

}

func removeTagFromExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	tagID TagID,
	experienceID ExperienceID) {
	response, err := client.RemoveExperienceTagFromExperienceWithResponse(
		context.Background(),
		projectID,
		tagID,
		experienceID,
	)
	if err != nil {
		log.Fatal("failed to update tags: ", err)
	}
	utils.ValidateResponse(http.StatusNoContent, "failed to update tags", response.HTTPResponse, response.Body)
}

func updateSingleTag(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates TagUpdates) {

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
			log.Fatal("failed to update tags: ", err)
		}
		utils.ValidateResponse(http.StatusCreated, "failed to update tags", response.HTTPResponse, response.Body)
	}

	var wg sync.WaitGroup
	for _, removal := range updates.Removals {
		wg.Add(1)
		go func() {
			removeTagFromExperience(client, projectID,
				updates.TagID,
				removal.ExperienceID.ID)
			wg.Done()
		}()
	}
	wg.Wait()
}

func updateSingleSystem(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates SystemUpdates) {
	if len(updates.Additions) > 0 {
		experienceIDs := []ExperienceID{}
		for _, e := range updates.Additions {
			if e.ExperienceID == nil {
				log.Fatal("Experience has no ID. Maybe we failed to create it? ", e.Name)
			}
			experienceIDs = append(experienceIDs, e.ExperienceID.ID)
		}
		body := api.MutateSystemsToExperienceInput{
			SystemIDs:   []SystemID{updates.SystemID},
			Experiences: &experienceIDs,
		}
		response, err := client.AddSystemsToExperiencesWithResponse(context.Background(), projectID, body)
		if err != nil {
			log.Fatal("failed to update systems: ", err)
		}
		utils.ValidateResponse(http.StatusCreated, "failed to update systems", response.HTTPResponse, response.Body)
	}
}

func updateSingleTestSuite(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update TestSuiteUpdate) {

	experiences := []ExperienceID{}

	for _, exp := range update.Experiences {
		experiences = append(experiences, exp.ExperienceID.ID)
	}

	body := api.ReviseTestSuiteInput{
		Experiences: &experiences,
	}
	response, err := client.ReviseTestSuiteWithResponse(context.Background(), projectID, update.TestSuiteID, body)
	if err != nil {
		log.Fatal("failed to revise test suite: ", err)
	}
	utils.ValidateResponse(http.StatusOK, "failed to revise test suite", response.HTTPResponse, response.Body)
}
