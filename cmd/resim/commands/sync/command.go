package sync

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands/dvc"
	"gopkg.in/yaml.v3"
)

func SyncExperiences(client api.ClientWithResponsesInterface,
	projectID uuid.UUID,
	configPath string,
	updateConfig bool,
	shouldArchive bool,
	dvcRemote string,
) {
	if configPath == "" {
		log.Fatal("experiences-config not set")
	}
	config, err := loadExperienceSyncConfig(configPath, false)
	if err != nil {
		log.Fatalf("%v", err)
	}
	restoreLocations := func() {}
	if dvcRemote != "" {
		restoreLocations, err = translateDvcLocations(config, dvcRemote)
		if err != nil {
			log.Fatalf("%v", err)
		}
	}
	currentState, err := getCurrentDatabaseState(client, projectID)
	if err != nil {
		log.Fatalf("%v", err)
	}
	experienceUpdates, err := computeExperienceUpdates(config, *currentState, shouldArchive)
	if err != nil {
		log.Fatalf("%v", err)
	}
	err = applyUpdates(client, projectID, *experienceUpdates)
	if err != nil {
		log.Fatalf("%v", err)
	}

	if updateConfig {
		// The config keeps the local paths (the source of truth for future
		// syncs), not the hash-pinned locations we sent to the backend.
		restoreLocations()
		writeConfigToFile(config, configPath)
	}
}

// translateDvcLocations rewrites every local-path location in the config into
// a dvc+s3:// location pinned to the currently-tracked DVC hash; locations
// that already carry a URL scheme pass through unchanged. The returned restore
// function puts the original locations back.
func translateDvcLocations(config *ExperienceSyncConfig, dvcRemote string) (restore func(), err error) {
	resolver := dvc.NewResolver(dvcRemote)
	originals := make([][]string, len(config.Experiences))
	for ii := range config.Experiences {
		experience := &config.Experiences[ii]
		originals[ii] = experience.Locations
		translated := make([]string, len(experience.Locations))
		for jj, location := range experience.Locations {
			translated[jj], err = resolver.TranslateLocation(location)
			if err != nil {
				return nil, fmt.Errorf("experience %q: %w", experience.Name, err)
			}
		}
		experience.Locations = translated
	}
	return func() {
		for ii := range config.Experiences {
			config.Experiences[ii].Locations = originals[ii]
		}
	}, nil
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
	config.Experiences = []Experience{}
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
