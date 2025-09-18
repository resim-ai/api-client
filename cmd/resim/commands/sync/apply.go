package sync

import (
	"context"
	"fmt"
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
	experienceUpdates ExperienceUpdates) error {

	numWorkers := 16
	err := runConcurrentUpdates("Create/Update Experiences", slices.Collect(maps.Values(experienceUpdates.MatchedExperiencesByNewName)),
		numWorkers,
		func(update ExperienceMatch) error {
			return updateSingleExperience(client, projectID, update)
		})
	if err != nil {
		return err
	}

	err = runConcurrentUpdates("Update Test Suites", experienceUpdates.TestSuiteUpdates,
		numWorkers,
		func(update TestSuiteUpdate) error {
			return updateSingleTestSuite(client, projectID, update)
		})
	if err != nil {
		return err
	}

	err = runConcurrentUpdates("Update Tags", slices.Collect(maps.Values(experienceUpdates.TagUpdatesByName)),
		numWorkers,
		func(update *TagUpdates) error {
			return updateSingleTag(client, projectID, *update)
		})
	if err != nil {
		return err
	}

	err = runConcurrentUpdates("Update Systems", slices.Collect(maps.Values(experienceUpdates.SystemUpdatesByName)),
		numWorkers,
		func(update *SystemUpdates) error {
			return updateSingleSystem(client, projectID, *update)
		})
	if err != nil {
		return err
	}

	// We archive experiences *after* everything else so that we don't end up inadvertently
	// revising test suites more than necessary.
	err = maybeArchiveExperiences(client, projectID, slices.Collect(maps.Values(experienceUpdates.MatchedExperiencesByNewName)))

	return err
}

// Helper to parallelize our updates and track progress with a bar
func runConcurrentUpdates[T any](
	label string,
	items []T,
	numWorkers int,
	task func(T) error,
) error {
	if len(items) == 0 {
		return nil
	}

	log.Printf("%s...", label)
	bar := progressbar.Default(int64(len(items)))

	var wg sync.WaitGroup
	inputsCh := make(chan T, len(items))
	errCh := make(chan error, len(items))

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range inputsCh {
				if err := task(item); err != nil {
					errCh <- err
				}
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
	close(errCh)

	var firstErr error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
		log.Printf("Error updating %s: %v", label, err)
	}
	return firstErr
}

func updateSingleExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update ExperienceMatch) error {

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
			return fmt.Errorf("failed to create experience: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusCreated, "failed to create experience", response.HTTPResponse, response.Body)
		if err != nil {
			return err
		}

		// Update the match with then new experience ID. This way the other updates (which
		// reference this by pointer) will have the ID.
		update.New.ExperienceID = &response.JSON201.ExperienceID
		return nil
	}
	if update.New.ExperienceID == nil {
		// Fatal since this should *never* happen. It's a bug in the api client if so.
		return fmt.Errorf("Trying to update with unset experience ID")
	}

	if update.New.Archived {
		// Skip this experience. We archive it later
		return nil
	}
	// Update
	experienceID := *update.New.ExperienceID
	if update.Original.Archived {
		// Restore
		response, err := client.RestoreExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			return fmt.Errorf("failed to restore experience: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusNoContent, "failed to restore experience", response.HTTPResponse, response.Body)
		if err != nil {
			return err
		}
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
		return fmt.Errorf("failed to update experience: %s", err)
	}
	err = utils.ValidateResponseSafe(http.StatusOK, "failed to update experience", response.HTTPResponse, response.Body)
	return err
}

func maybeArchiveExperiences(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates []ExperienceMatch) error {

	experiencesToArchive := []ExperienceID{}

	for _, update := range updates {
		if !update.New.Archived {
			continue
		}
		if update.New.ExperienceID == nil {
			// Fatal since this should *never* happen. It's a bug in the api client if so.
			return fmt.Errorf("Trying to archive with unset experience ID")
		}
		experiencesToArchive = append(experiencesToArchive, *update.New.ExperienceID)
	}
	if len(experiencesToArchive) == 0 {
		return nil
	}
	body := api.BulkArchiveExperiencesInput{
		ExperienceIDs: experiencesToArchive,
	}
	response, err := client.BulkArchiveExperiencesWithResponse(
		context.Background(),
		projectID,
		body,
	)
	if err != nil {
		return fmt.Errorf("failed to archive experiences: %s", err)
	}
	err = utils.ValidateResponseSafe(http.StatusOK, "failed to update tags", response.HTTPResponse, response.Body)
	return err
}

func removeTagFromExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	tagID TagID,
	experienceID ExperienceID) error {
	response, err := client.RemoveExperienceTagFromExperienceWithResponse(
		context.Background(),
		projectID,
		tagID,
		experienceID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tags: %s", err)
	}
	err = utils.ValidateResponseSafe(http.StatusNoContent, "failed to update tags", response.HTTPResponse, response.Body)
	return err
}

func updateSingleTag(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates TagUpdates) error {

	if len(updates.Additions) > 0 {
		experienceIDs := []ExperienceID{}
		for _, e := range updates.Additions {
			if e.ExperienceID == nil {
				return fmt.Errorf("Experience has no ID. Maybe we failed to create it? %s", e.Name)
			}
			experienceIDs = append(experienceIDs, *e.ExperienceID)
		}

		body := api.AddTagsToExperiencesInput{
			ExperienceTagIDs: []TagID{updates.TagID},
			Experiences:      &experienceIDs,
		}
		response, err := client.AddTagsToExperiencesWithResponse(context.Background(), projectID, body)
		if err != nil {
			return fmt.Errorf("failed to update tags: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusCreated, "failed to update tags", response.HTTPResponse, response.Body)
		if err != nil {
			return err
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(updates.Removals))
	for _, removal := range updates.Removals {
		wg.Add(1)
		go func() {
			if err := removeTagFromExperience(client, projectID,
				updates.TagID,
				*removal.ExperienceID); err != nil {
				errCh <- err
			}
			wg.Done()
		}()
	}
	wg.Wait()

	close(errCh)

	var firstErr error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
		log.Printf("Error removing tag: %s", updates.TagID)
	}
	return firstErr
}

func updateSingleSystem(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates SystemUpdates) error {
	if len(updates.Additions) > 0 {
		experienceIDs := []ExperienceID{}
		for _, e := range updates.Additions {
			if e.ExperienceID == nil {
				return fmt.Errorf("Experience has no ID. Maybe we failed to create it? %s", e.Name)
			}
			experienceIDs = append(experienceIDs, *e.ExperienceID)
		}
		body := api.MutateSystemsToExperienceInput{
			SystemIDs:   []SystemID{updates.SystemID},
			Experiences: &experienceIDs,
		}
		response, err := client.AddSystemsToExperiencesWithResponse(context.Background(), projectID, body)
		if err != nil {
			return fmt.Errorf("failed to update systems: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusCreated, "failed to update systems", response.HTTPResponse, response.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func updateSingleTestSuite(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update TestSuiteUpdate) error {

	experiences := []ExperienceID{}

	for _, exp := range update.Experiences {
		experiences = append(experiences, *exp.ExperienceID)
	}

	body := api.ReviseTestSuiteInput{
		Experiences: &experiences,
	}
	response, err := client.ReviseTestSuiteWithResponse(context.Background(), projectID, update.TestSuiteID, body)
	if err != nil {
		return fmt.Errorf("failed to revise test suite: %s", err)
	}
	err = utils.ValidateResponseSafe(http.StatusOK, "failed to revise test suite", response.HTTPResponse, response.Body)
	return err
}
