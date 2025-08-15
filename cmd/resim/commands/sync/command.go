package sync

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)


func SyncExperience(client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	configPath string,
	updateConfig bool,
) {
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config := loadExperienceSyncConfig(configPath)


	currentExperiencesByName := getCurrentExperiencesByName(client, projectID)
	currentTagSets := getCurrentTagSets(client, projectID)
	apiSystems := fetchAllSystems(client, projectID)
	

	matchedExperiencesByNewName := matchExperiences(config, currentExperiencesByName)


	for _, update := range *matchedExperiencesByNewName {
		updateSingleExperience(client, projectID, &update)
	}

	if updateConfig {
		writeConfigToFile(config, configPath)
	}

	_ = apiSystems
	_ = currentTagSets

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
