package sync

import (
	"github.com/google/uuid"	
	"gopkg.in/yaml.v3"
	"os"
	"log"
)



type Experience struct {
	Name                    string                     `yaml:"name"`
	Description             string                     `yaml:"description"`
	Locations               []string                   `yaml:"locations,omitempty"`
	Tags                    []string                   `yaml:"tags,omitempty"`                  // Only used on read from config
	Systems                 []string                   `yaml:"systems,omitempty"`               // Only used on read from config
	Profile                 *string                    `yaml:"profile,omitempty"`               // Optional
	ExperienceID            *string                    `yaml:"experience_id,omitempty"`         // Optional
	EnvironmentVariables    *[]EnvironmentVariable `yaml:"environment_variables,omitempty"` // Optional
	CacheExempt             bool                       `yaml:"cache_exempt,omitempty"`
	ContainerTimeoutSeconds *int32                     `yaml:"container_timeout_seconds,omitempty"` // Optional
	Archived                bool                       `yaml:"-"`                                   // Shouldn't be in the config
}

type TestSuite struct {
	Name        string   `yaml:"name,omitempty"`
	Experiences []string `yaml:"experiences,omitempty"`
}

type ExperienceSyncConfig struct {
	RemovableTags []string      `yaml:"removable_tags,omitempty"`
	Experiences   []*Experience `yaml:"experiences,omitempty"`
	TestSuites    []TestSuite   `yaml:"test_suites,omitempty"`
}

type ExperienceUpdate struct {
	Original *Experience
	New      *Experience
}


func LoadExperienceSyncConfig(path string) *ExperienceSyncConfig {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", path)
	}
	// Read file
	data, err := os.ReadFile(path)

	if err != nil {
		log.Fatalf("Failed to load config file: %s", err)
	}
	var cfg ExperienceSyncConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("failed to unmarshal config: %s", err)
	}
	// Do some normalization and validation
	for _, experience := range cfg.Experiences {
		if experience.Name == "" {
			log.Fatal("Empty experience name.")
		}
		if experience.Description == "" {
			log.Fatal("Empty experience description for experience: ", experience.Name)
		}
		if experience.Locations == nil || len(experience.Locations) == 0 {
			log.Fatal("No locations provided for experience: ", experience.Name)
		}
		if experience.ExperienceID != nil && !isValidUUID(*experience.ExperienceID) {
			log.Fatal("Invalid experience ID: ", *experience.ExperienceID)
		}
	}
	return &cfg
}



func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
