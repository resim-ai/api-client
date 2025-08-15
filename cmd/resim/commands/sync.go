package commands

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
)

type Experience struct {
	Name                    string                     `yaml:"name"`
	Description             string                     `yaml:"description"`
	Locations               []string                   `yaml:"locations,omitempty"`
	Tags                    []string                   `yaml:"tags,omitempty"`                  // Only used on read from config
	Systems                 []string                   `yaml:"systems,omitempty"`               // Only used on read from config
	Profile                 *string                    `yaml:"profile,omitempty"`               // Optional
	ExperienceID            *string                    `yaml:"experience_id,omitempty"`         // Optional
	EnvironmentVariables    *[]api.EnvironmentVariable `yaml:"environment_variables,omitempty"` // Optional
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

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func loadExperienceSyncConfig(path string) *ExperienceSyncConfig {
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

func addApiExperienceToExperienceMap(experience api.Experience,
	currentExperiences *map[string]*Experience) {
	(*currentExperiences)[experience.Name] = &Experience{
		Name:                    experience.Name,
		Description:             experience.Description,
		Locations:               experience.Locations,
		Profile:                 &experience.Profile,
		ExperienceID:            Ptr(experience.ExperienceID.String()),
		EnvironmentVariables:    &experience.EnvironmentVariables,
		CacheExempt:             experience.CacheExempt,
		ContainerTimeoutSeconds: &experience.ContainerTimeoutSeconds,
		Archived:                experience.Archived,
	}
}

func getCurrentExperiences(apiExperiences []api.Experience,
	apiArchivedExperiences []api.Experience) *map[string]*Experience {
	result := make(map[string]*Experience)
	for _, experience := range apiExperiences {
		addApiExperienceToExperienceMap(experience, &result)
	}
	for _, experience := range apiArchivedExperiences {
		addApiExperienceToExperienceMap(experience, &result)
	}
	return &result
}

func byNameToByID(byName *map[string]*Experience) *map[string]*Experience {
	byID := make(map[string]*Experience)
	for _, v := range *byName {
		if v.ExperienceID != nil {
			byID[*v.ExperienceID] = v
		}
	}
	return &byID
}

func checkedInsert[K comparable, V any](m *map[K]V,
	key K, value V) {
	if _, exists := (*m)[key]; exists {
		log.Fatalf("Duplicate key!")
	}
	(*m)[key] = value
}

func getDesiredUpdates(config *ExperienceSyncConfig, currentExperiencesByName *map[string]*Experience) *map[string]ExperienceUpdate {
	// Our algorithm can be summarized like so:
	//
	// For each configured experience, we attempt to match it to an existing experience if
	// possible. This procedure works like so:
	//
	// 1. If an existing experience has the same name, we match with it or fail if it's already
	//    been matched with.
	//
	//    If the desired experience has a user-specified ID that is not the same as matched
	//    experience we fail. We fail because this means that some other experience currently
	//    has this name. Because the current state has unique names, this can only happen if
	//    we're trying to update an existing experience to have the name that another currently
	//    uses. It could still be the case that the final state would be valid (e.g. if the
	//    current owner of the name is also going to change *its* name) but we don't allow
	//    this. In some cases, there may exist some ordering of name updates that never tries to
	//    create an invalid state, we don't attempt to determine it. There are easy work-arounds
	//    in such cases (e.g. running this script once to add a prefix to all experiences, and
	//    then again to set them to their new desired names).
	//
	//    Once we perform the matching, we remove the current experience that we matched with
	//    from the remainingCurrentExperiencesByID map so that no future step or experience can
	//    match with it.
	//
	//    We also set the experience ID in the configuration so it can be output for the user.
	//
	// 2. If no existing experience has the same name, and we have a user-specified ID, we
	//    attempt to find a match in the remainingCurrentExperiencesByID map. If we can't,
	//    that's a failure because the experience with that ID either never existed or already
	//    got matched (implying that multiple configured experiences were really referring to
	//    the same experience). If we can match we remove the current experience that we matched
	//    with from the remainingCurrentExperiencesByID map so that no future step can match
	//    with it.
	//
	// 3. If no existing experience has the same name or the same ID, it must be new.
	//
	// After all desired experiences have been matched, any remaining unmatched existing
	// experiences should be archived if they aren't already.
	//
	// The above procedure guarantees that every desired experience has a unique name and ID and
	// is matched to a unique existing experience if that's possible. It also guarantees that no
	// desired experience has a name currently owned by another experience.
	desiredUpdates := make(map[string]ExperienceUpdate)

	remainingCurrentExperiencesByID := byNameToByID(currentExperiencesByName)

	for _, experience := range config.Experiences {
		// Step 1: Attempt to match by name
		currExp, exists := (*currentExperiencesByName)[experience.Name]
		if exists {
			// If the match target has already been matched with, that's a failure
			if _, isAvailable := (*remainingCurrentExperiencesByID)[*currExp.ExperienceID]; !isAvailable {
				log.Fatalf("Experience name collision: %s", currExp.Name)
			}

			// If it exists but it's ID doesn't match a hard-coded one we provide, that's a failure
			if currExp.ExperienceID == nil || (experience.ExperienceID != nil && *experience.ExperienceID != *currExp.ExperienceID) {
				log.Fatalf("Multiple experiences desire the same name.")
			}

			// Experience exists with the same name and should be updated
			experience.ExperienceID = currExp.ExperienceID
			checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
				Original: currExp,
				New:      experience,
			})
			delete(*remainingCurrentExperiencesByID, *currExp.ExperienceID)
			continue
		}
		// Step 2: Attempt to match by ID
		if experience.ExperienceID != nil {
			// Check if there's still an unmatched experience with this ID:
			currExp, exists := (*remainingCurrentExperiencesByID)[*experience.ExperienceID]
			if !exists {
				log.Fatalf("No existing experience available with ID. This could be due to multiple configured experiences requesting the same ID: %s", *experience.ExperienceID)
			}

			checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
				Original: currExp,
				New:      experience,
			})
			delete(*remainingCurrentExperiencesByID, *currExp.ExperienceID)
			continue
		}

		// Step 3: Must be new then:
		checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
			Original: nil,
			New:      experience,
		})

	}
	// Step 4: Any leftover experiences should be archived
	for _, experience := range *remainingCurrentExperiencesByID {
		if experience.Archived {
			// No updates needed
			continue
		}
		archivedVersion := *experience
		archivedVersion.Archived = true
		checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
			Original: experience,
			New:      &archivedVersion,
		})
	}
	return &desiredUpdates
}

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
	config := loadExperienceSyncConfig(configPath)

	projectID := getProjectID(Client, viper.GetString(experienceProjectKey))
	archived := true
	unarchived := false

	apiExperiences := fetchAllExperiences(projectID, unarchived)
	archivedApiExperiences := fetchAllExperiences(projectID, archived)

	apiExperienceTags := fetchAllExperienceTags(projectID)
	apiSystems := fetchAllSystems(projectID)

	currentExperiences := getCurrentExperiences(apiExperiences, archivedApiExperiences)
	desiredUpdates := getDesiredUpdates(config, currentExperiences)

	for _, update := range *desiredUpdates {
		updateSingleExperience(projectID, &update)
	}

	if viper.GetBool(experiencesUpdateConfigKey) {
		writeConfigToFile(config, configPath)
	}

	_ = projectID
	_ = apiSystems
	_ = apiExperienceTags

}
