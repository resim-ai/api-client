package commands

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	. "github.com/resim-ai/api-client/cmd/resim/commands/sync"	
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
)




func updateSingleExperience(
	projectID uuid.UUID,
	update *ExperienceUpdate) {
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

		response, err := Client.CreateExperienceWithResponse(context.Background(), projectID, body)
		if err != nil {
			log.Print("WARNING: failed to create experience: ", err)
		}
		err = ValidateResponseSafe(http.StatusCreated, "failed to create experience", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING:", err)
		}

		update.New.ExperienceID = Ptr(response.JSON201.ExperienceID.String())
		return
	}
	if update.New.Archived {
		// Archive
		experienceID, err := uuid.Parse(*update.New.ExperienceID)
		if err != nil {
			log.Print("WARNING: failed to archive experience: ", err)
		}
		response, err := Client.ArchiveExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			log.Print("WARNING: failed to archive experience: ", err)
		}
		err = ValidateResponseSafe(http.StatusNoContent, "failed to archive experience", response.HTTPResponse, response.Body)
		if err != nil {
			log.Print("WARNING: ", err)
		}

		return
	}
	// Update
	experienceID, err := uuid.Parse(*update.New.ExperienceID)
	if err != nil {
		log.Print("WARNING: failed to update experience: ", err)
	}
	if update.Original.Archived {
		// Restore
		response, err := Client.RestoreExperienceWithResponse(context.Background(), projectID, experienceID)
		if err != nil {
			log.Print("WARNING: failed to restore experience: ", err)
		}
		err = ValidateResponseSafe(http.StatusNoContent, "failed to restore experience", response.HTTPResponse, response.Body)
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
	response, err := Client.UpdateExperienceWithResponse(context.Background(), projectID, experienceID, body)
	if err != nil {
		log.Print("WARNING: failed to update experience: ", err)
	}
	err = ValidateResponseSafe(http.StatusOK, "failed to update experience", response.HTTPResponse, response.Body)
	if err != nil {
		log.Print("WARNING:", err)
	}

}

func writeConfigToFile(config *ExperienceSyncConfig, path string) {
	data, err := yaml.Marshal(config)
	if err != nil {
		log.Fatal("Failed to marshal updated config:", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Fatal("Failed to write updated config to file:", err)
	}

	fmt.Printf("Updated config written to %s\n", path)
}

func syncExperience(ccmd *cobra.Command, args []string) {
	configPath := viper.GetString(experiencesConfigKey)
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config := LoadExperienceSyncConfig(configPath)

	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	archived := true
	unarchived := false

	apiExperiences := FetchAllExperiences(Client, projectID, unarchived)
	archivedApiExperiences := FetchAllExperiences(Client, projectID, archived)

	apiExperienceTags := FetchAllExperienceTags(Client, projectID)
	apiSystems := FetchAllSystems(Client, projectID)

	currentExperiences := GetCurrentExperiences(apiExperiences, archivedApiExperiences)
	desiredUpdates := GetDesiredUpdates(config, currentExperiences)

	for _, update := range *desiredUpdates {
		updateSingleExperience(projectID, &update)
	}

	if viper.GetBool(experiencesUpdateConfigKey) {
		writeConfigToFile(config, configPath)
	}

	_ = apiSystems
	_ = apiExperienceTags

}
