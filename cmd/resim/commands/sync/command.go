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

func SyncExperiences(client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	configPath string,
	updateConfig bool,
) {
	return
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config := loadExperienceSyncConfig(configPath)
	
	currentExperiencesByName := getCurrentExperiencesByName(client, projectID)
	currentTagSetsByName := getCurrentTagSetsByName(client, projectID)
	currentSystemSetsByName := getCurrentSystemSetsByName(client, projectID)

	
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

	_ = currentSystemSetsByName

}

func CloneExperiences(client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	configPath string) {
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config := loadExperienceSyncConfig(configPath)
	config.Experiences = []*Experience{}

	currentExperiencesByName := getCurrentExperiencesByName(client, projectID)
	currentTagSetsByName := getCurrentTagSetsByName(client, projectID)
	currentSystemSetsByName := getCurrentSystemSetsByName(client, projectID)
	
	for _, experience := range currentExperiencesByName {
		if experience.Archived {
			continue
		}
		for tag, tag_set := range currentTagSetsByName {
			if _, has_tag := tag_set.ExperienceIDs[experience.ExperienceID.ID]; has_tag {
				experience.Tags = append(experience.Tags, tag)
			}
		}
		for system, system_set := range currentSystemSetsByName {
			if _, has_system := system_set.ExperienceIDs[experience.ExperienceID.ID]; has_system {
				experience.Systems = append(experience.Systems, system)
			}
		}
		config.Experiences = append(config.Experiences, experience)
	}
	writeConfigToFile(config, configPath)	
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
