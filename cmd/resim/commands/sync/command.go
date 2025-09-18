package sync

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

func SyncExperiences(client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	configPath string,
	updateConfig bool,
) {
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config, err := loadExperienceSyncConfig(configPath, false)
	if err != nil {
		log.Fatalf("%v", err)
	}
	currentState, err := getCurrentDatabaseState(client, projectID)
	if err != nil {
		log.Fatalf("%v", err)
	}
	experienceUpdates, err := computeExperienceUpdates(config, *currentState)
	if err != nil {
		log.Fatalf("%v", err)
	}
	err = applyUpdates(client, projectID, *experienceUpdates)
	if err != nil {
		log.Fatalf("%v", err)
	}

	if updateConfig {
		writeConfigToFile(config, configPath)
	}
}

func CloneExperiences(client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	configPath string) {
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config, err := loadExperienceSyncConfig(configPath, true)
	if err != nil {
		log.Fatalf("%v", err)
	}
	currentState, err := getCurrentDatabaseState(client, projectID)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for _, experience := range currentState.ExperiencesByName {
		if !experience.Archived {
			config.Experiences = append(config.Experiences, *experience)
		}
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
