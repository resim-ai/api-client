package sync

import (
	"fmt"
	"github.com/resim-ai/api-client/api"
	"gopkg.in/yaml.v3"
	"os"
)

type Experience = api.ExperienceSyncExperience
type TestSuite = api.ExperienceSyncTestSuite
type ExperienceSyncConfig = api.ExperienceSyncConfig

func loadExperienceSyncConfig(path string, allowNew bool) (*ExperienceSyncConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if allowNew {
			return &ExperienceSyncConfig{}, nil
		}
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
	if err := NormalizeExperiences(cfg.Experiences); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func normalizeExperience(experience *Experience) error {
	if experience.Name == "" {
		return fmt.Errorf("Empty experience name.")
	}
	if experience.Description == "" {
		return fmt.Errorf("Empty experience description for experience: %s", experience.Name)
	}
	if len(experience.Locations) == 0 {
		return fmt.Errorf("No locations provided for experience: %s", experience.Name)
	}
	if experience.CacheMethod != nil {
		switch *experience.CacheMethod {
		case api.NONE, api.EBS, api.EFS:
		default:
			return fmt.Errorf("invalid cacheMethod %q for experience %s: expected NONE, EBS, or EFS", *experience.CacheMethod, experience.Name)
		}
		if experience.CacheExempt != nil && *experience.CacheExempt != (*experience.CacheMethod == api.NONE) {
			return fmt.Errorf("cacheExempt conflicts with cacheMethod for experience %s", experience.Name)
		}
	}
	return nil
}

func NormalizeExperiences(experiences []Experience) error {
	for ii := range experiences {
		err := normalizeExperience(&experiences[ii])
		if err != nil {
			return err
		}
	}
	return nil
}
