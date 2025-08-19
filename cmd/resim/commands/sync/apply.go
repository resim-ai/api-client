package sync

import (
	"context"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands/utils"
	"github.com/schollz/progressbar/v3"
	"log"
	"net/http"
	"sync"
)

func applyUpdates(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	experienceUpdates ExperienceUpdates) {
	num_experiences := len(experienceUpdates.MatchedExperiencesByNewName)
	inputs_chan := make(chan ExperienceMatch, num_experiences)
	progress_chan := make(chan struct{}, num_experiences)
	done := make(chan struct{})

	log.Print("Updating Experiences...")
	bar := progressbar.Default(int64(num_experiences))

	go func() {
		for ii := 0; ii < num_experiences; ii++ {
			<-progress_chan
			bar.Add(1)
		}
		done <- struct{}{}
	}()

	numWorkers := 16

	for ii := 0; ii < numWorkers; ii++ {
		go func() {
			for update := range inputs_chan {
				updateSingleExperience(client, projectID, update)
				progress_chan <- struct{}{}
			}
		}()
	}

	for _, update := range experienceUpdates.MatchedExperiencesByNewName {
		inputs_chan <- update
	}
	close(inputs_chan)
	<-done

	log.Print("Updating Tags...")
	var wg sync.WaitGroup
	bar = progressbar.Default(int64(len(experienceUpdates.TagUpdatesByName)))
	for _, update := range experienceUpdates.TagUpdatesByName {
		wg.Add(1)
		go func() {
			updateSingleTag(client, projectID, *update)
			bar.Add(1)
			wg.Done()
		}()
	}
	wg.Wait()
	log.Print("Updating Systems...")
	bar = progressbar.Default(int64(len(experienceUpdates.SystemUpdatesByName)))
	for _, update := range experienceUpdates.SystemUpdatesByName {
		wg.Add(1)
		go func() {
			updateSingleSystem(client, projectID, *update)
			bar.Add(1)
			wg.Done()
		}()
	}
	wg.Wait()
	log.Print("Updating Test Suites...")
	bar = progressbar.Default(int64(len(experienceUpdates.SystemUpdatesByName)))
	for _, update := range experienceUpdates.TestSuiteUpdates {
		wg.Add(1)
		go func() {
			updateSingleTestSuite(client, projectID, update)
			bar.Add(1)
			wg.Done()
		}()
	}
	wg.Wait()

}

func updateSingleExperience(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	update ExperienceMatch) {
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
		log.Print("WARNING: failed to update tags: ", err)
	}
	err = utils.ValidateResponseSafe(http.StatusNoContent, "failed to update tags", response.HTTPResponse, response.Body)
	if err != nil {
		log.Print("WARNING:", err)
	}

}

func updateSingleTag(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	updates TagUpdates) {

	var wg sync.WaitGroup
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
			log.Print("WARNING: failed to update systems: ", err)
		}
		err = utils.ValidateResponseSafe(http.StatusCreated, "failed to update systems", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING:", err)
		}
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
		log.Print("WARNING: failed to revise test suite: ", err)
	}
	err = utils.ValidateResponseSafe(http.StatusOK, "failed to revise test suite", response.HTTPResponse, response.Body)
	if err != nil {
		log.Print("WARNING:", err)
	}
}
