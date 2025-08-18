package sync

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log"
	"testing"
)

func TestUpdateExperiencesEmpty(t *testing.T) {
	// SETUP
	config := ExperienceSyncConfig{}
	currentState := DatabaseState{}

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	assert.Empty(t, experienceUpdates.MatchedExperiencesByNewName, "Should be no updates")
	assert.Empty(t, experienceUpdates.TagUpdatesByName, "Should be no updates")
}

func TestUpdateAddSingleExperience(t *testing.T) {
	// SETUP
	currentStateData := `
experiences:
`
	configData := `
experiences:
  - name: Test Experience
    description: This is a test experience
    locations:
      - s3://my-favorite-bucket/foo
    environment_variables:
      - name: ENV_VAR_1
        value: value1
    cache_exempt: true
    container_timeout_seconds: 7200
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	assert.Len(t, experienceUpdates.MatchedExperiencesByNewName, 1, "Should be one experience update")
	assert.Empty(t, experienceUpdates.TagUpdatesByName, "Should be no tag updates")

	match, exists := experienceUpdates.MatchedExperiencesByNewName[config.Experiences[0].Name]
	assert.True(t, exists, "Expected experience in updates")
	assert.Same(t, config.Experiences[0], match.New, "Should be the same object (pointer equality)")
	assert.Nil(t, match.Original, "Experience is not new")
}

func TestArchiveSingleExperience(t *testing.T) {
	// SETUP
	currentStateData := `
experiences:
  - name: Test Experience
    description: This is a test experience
    experience_id: "3dd91177-1e66-426c-bf5b-fb46fe4a0c3b"
    locations:
      - s3://my-favorite-bucket/foo
    environment_variables:
      - name: ENV_VAR_1
        value: value1
    cache_exempt: true
    container_timeout_seconds: 7200
`
	configData := `
experiences:
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	assert.Len(t, experienceUpdates.MatchedExperiencesByNewName, 1, "Should be one experience update")
	assert.Empty(t, experienceUpdates.TagUpdatesByName, "Should be no tag updates")

	match, exists := experienceUpdates.MatchedExperiencesByNewName["Test Experience"]
	assert.True(t, exists, "Expected experience in updates")
	assert.Same(t, currentState.ExperiencesByName["Test Experience"], match.Original, "Should be the same object (pointer equality)")

	log.Print(match.Original.Locations)
	log.Print(match.New.Locations)

	assert.Equal(t, match.Original.Name, match.New.Name)
	assert.Equal(t, match.Original.Description, match.New.Description)
	assert.Equal(t, match.Original.Name, match.New.Name)
	assert.Equal(t, match.Original.Profile, match.New.Profile)
	assert.Equal(t, match.Original.ExperienceID, match.New.ExperienceID)
	assert.Equal(t, match.Original.EnvironmentVariables, match.New.EnvironmentVariables)
	assert.Equal(t, match.Original.CacheExempt, match.New.CacheExempt)
	assert.Equal(t, match.Original.ContainerTimeoutSeconds, match.New.ContainerTimeoutSeconds)
	assert.True(t, match.New.Archived, "Experience should be archived")
}

func TestUpdateSingleExperiencesByNameAndID(t *testing.T) {
	// SETUP
	// This includes a restore
	currentStateData := `
experiences:
  - name: experience-to-update-by-name
    description: This is a test experience
    experience_id: "3dd91177-1e66-426c-bf5b-fb46fe4a0c3b"
    locations:
      - s3://my-favorite-bucket/foo
    environment_variables:
      - name: ENV_VAR_1
        value: value1
    cache_exempt: true
    container_timeout_seconds: 7200
    archived: true  # This will be restured
  - name: experience-to-update-by-id
    description: This is a test experience2
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    locations:
      - s3://my-favorite-bucket/bar
    environment_variables:
      - name: ENV_VAR_2
        value: value2
    cache_exempt: false
    container_timeout_seconds: 7200
`
	configData := `
experiences:
  - name: experience-to-update-by-name
    description: This is my new experience"
    locations:
      - s3://my-favorite-bucket/my-new-location
    environment_variables:
      - name: ENV_VAR_1
        value: value_new
    cache_exempt: false
    container_timeout_seconds: 7300
  - name: new-name-for-experience-to-update-by-id
    description: This is a test experience2
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    locations:
      - s3://my-favorite-bucket/bar
    environment_variables:
      - name: ENV_VAR_2
        value: value2
    cache_exempt: false
    container_timeout_seconds: 7300
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	numUpdates := 2
	assert.Len(t, experienceUpdates.MatchedExperiencesByNewName, numUpdates, "Should be one experience update")
	assert.Empty(t, experienceUpdates.TagUpdatesByName, "Should be no tag updates")

	originalNames := []string{"experience-to-update-by-name", "experience-to-update-by-id"}
	for i := 0; i < numUpdates; i++ {
		match, exists := experienceUpdates.MatchedExperiencesByNewName[config.Experiences[i].Name]
		assert.True(t, exists, "Expected experience in updates")
		assert.Same(t, currentState.ExperiencesByName[originalNames[i]], match.Original, "Should be the same object (pointer equality)")
		assert.Same(t, config.Experiences[i], match.New, "Should be the same object (pointer equality)")
	}
}

func TestFailsOnAmbiguousRenaming(t *testing.T) {
	// SETUP
	// In this arrangement, the name of the first configured experience indicates it's the
	// update for current-experience. However, the ID for the second configured experience also
	// indicates it's the update for current-experience. This is a failure.
	currentStateData := `
experiences:
  - name: current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	configData := `
experiences:
  - name: current-experience
  - name: new-name-for-current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.NoError(t, err)

	// SETUP
	currentStateData = `
experiences:
  - name: current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	configData = `
experiences:
  - name: new-name-for-current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
  - name: current-experience
`
	currentState, config = loaderHelper(t, currentStateData, configData)

	// ACTION / VERIFICATION
	_, err = computeExperienceUpdates(&config, currentState)
	assert.NoError(t, err)
}

func TestFailsOnNonExistentID(t *testing.T) {
	// SETUP
	currentStateData := `
experiences:
  - name: current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	configData := `
experiences:
  - name: new-name-for-current-experience
    experience_id: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.NoError(t, err)
}

func TestFailsOnDuplicateID(t *testing.T) {
	// SETUP
	currentStateData := `
experiences:
  - name: current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	configData := `
experiences:
  - name: new-name-for-current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
  - name: other-new-name-for-current-experience
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.NoError(t, err)
}

func TestFailsOnClobberingExistingWithRename(t *testing.T) {
	// SETUP
	currentStateData := `
experiences:
  - name: new-name
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
  - name: old-name
    experience_id: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
`
	configData := `
experiences:
  - name: new-name
    experience_id: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
  - name: old-name
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.NoError(t, err)
}

func TestFailsOnNameCollision(t *testing.T) {
	// SETUP
	currentStateData := `
experiences:
  - name: new-name
    experience_id: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
  - name: old-name
    experience_id: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
`
	configData := `
experiences:
  - name: new-name
  - name: new-name
`
	currentState, config := loaderHelper(t, currentStateData, configData)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.NoError(t, err)
}

func loaderHelper(t *testing.T, currentStateData string, configData string) (DatabaseState, ExperienceSyncConfig) {
	var currentExperiences map[string][]*Experience
	err := yaml.Unmarshal([]byte(currentStateData), &currentExperiences)
	assert.NoError(t, err, "failed to unmarshal YAML")
	log.Print(currentExperiences)
	currentState := DatabaseState{
		ExperiencesByName: map[string]*Experience{},
	}
	for _, exp := range currentExperiences["experiences"] {
		currentState.ExperiencesByName[exp.Name] = exp
	}

	var config ExperienceSyncConfig
	err = yaml.Unmarshal([]byte(configData), &config)
	assert.NoError(t, err, "failed to unmarshal YAML")

	return currentState, config
}
