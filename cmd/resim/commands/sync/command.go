package sync

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"sync"
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
	currentTagSetsByName := getCurrentTagSetsByName(client, projectID)
	apiSystems := fetchAllSystems(client, projectID)

	matchedExperiencesByNewName := matchExperiences(config, currentExperiencesByName)
	tagUpdates := getTagUpdates(matchedExperiencesByNewName, currentTagSetsByName, config.ManagedExperienceTags)

	for _, update := range matchedExperiencesByNewName {
		updateSingleExperience(client, projectID, update)
	}

	var wg sync.WaitGroup

	for _, update := range tagUpdates {
		wg.Add(1)
		go asyncUpdateSingleTag(&wg, client, projectID, update)
	}
	wg.Wait()

	if updateConfig {
		writeConfigToFile(config, configPath)
	}

	_ = currentTagSetsByName
	//_ = matchedExperiencesByOldID
	_ = tagUpdates
	_ = apiSystems

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
