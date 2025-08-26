package sync

import (
	"fmt"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"os"
)

type ExperienceIDWrapper struct { // For custom unmarshalling
	ID ExperienceID
}

type Experience struct {
	Name                    string                 `yaml:"name"`
	Description             string                 `yaml:"description"`
	Locations               []string               `yaml:"locations,omitempty"`
	Tags                    []string               `yaml:"tags,omitempty"`                  // Only used on read from config
	Systems                 []string               `yaml:"systems,omitempty"`               // Only used on read from config
	Profile                 *string                `yaml:"profile,omitempty"`               // Optional
	ExperienceID            *ExperienceIDWrapper   `yaml:"experience_id,omitempty"`         // Optional
	EnvironmentVariables    *[]EnvironmentVariable `yaml:"environment_variables,omitempty"` // Optional
	CacheExempt             bool                   `yaml:"cache_exempt,omitempty"`
	ContainerTimeoutSeconds *int32                 `yaml:"container_timeout_seconds,omitempty"` // Optional
	Archived                bool                   `yaml:"-"`                                   // Shouldn't be in the config
}

type TestSuite struct {
	Name        string   `yaml:"name,omitempty"`
	Experiences []string `yaml:"experiences,omitempty"`
}

type ExperienceSyncConfig struct {
	Experiences           []*Experience `yaml:"experiences,omitempty"`
	TestSuites            []TestSuite   `yaml:"managed_test_suites,omitempty"`
	ManagedExperienceTags []string      `yaml:"managed_experience_tags,omitempty"`
}

func loadExperienceSyncConfig(path string) (*ExperienceSyncConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", path)
	}
	// Read file
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Failed to load config file: %s", err)
	}
	var cfg ExperienceSyncConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %s", err)
	}
	// Do some normalization and validation
	for _, experience := range cfg.Experiences {
		if experience.Name == "" {
			return nil, fmt.Errorf("Empty experience name.")
		}
		if experience.Description == "" {
			return nil, fmt.Errorf("Empty experience description for experience: %s", experience.Name)
		}
		if experience.Locations == nil || len(experience.Locations) == 0 {
			return nil, fmt.Errorf("No locations provided for experience: %s", experience.Name)
		}
	}
	return &cfg, nil
}

func (u *ExperienceIDWrapper) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	if s == "" {
		return nil // allow empty / missing
	}
	parsed, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	u.ID = parsed
	return nil
}

func (u ExperienceIDWrapper) MarshalYAML() (interface{}, error) {
	if u.ID == uuid.Nil {
		return "", nil
	}
	return u.ID.String(), nil
}
