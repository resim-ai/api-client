package sync

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"slices"
	"testing"
)

// A helper so we can load currentStateData and configData from yaml for convenience.
func loaderHelper(t *testing.T, currentStateData string, configData string, currentTags []string, currentSystems []string) (DatabaseState, ExperienceSyncConfig) {
	var currentExperiences []Experience
	err := yaml.Unmarshal([]byte(currentStateData), &currentExperiences)
	assert.NoError(t, err, "failed to unmarshal YAML")
	err = NormalizeExperiences(currentExperiences)
	assert.NoError(t, err, "invalid experiences")

	assert.NoError(t, err, "failed to unmarshal YAML")
	currentState := DatabaseState{
		ExperiencesByName: map[string]*Experience{},
		TagSetsByName:     map[string]TagSet{},
		SystemSetsByName:  map[string]SystemSet{},
	}
	for _, tag := range currentTags {
		currentState.TagSetsByName[tag] = TagSet{
			Name:          tag,
			TagID:         uuid.New(),
			ExperienceIDs: make(map[ExperienceID]struct{}),
		}

	}
	for _, system := range currentSystems {
		currentState.SystemSetsByName[system] = SystemSet{
			Name:          system,
			SystemID:      uuid.New(),
			ExperienceIDs: make(map[ExperienceID]struct{}),
		}

	}
	// Populate the tag and system sets based off what's listed in the YAML snippet.
	for ii := range currentExperiences {
		exp := &currentExperiences[ii]
		currentState.ExperiencesByName[exp.Name] = exp
		for _, tag := range exp.Tags {
			currentState.TagSetsByName[tag].ExperienceIDs[*exp.ExperienceID] = struct{}{}
		}
		for _, system := range exp.Systems {
			currentState.SystemSetsByName[system].ExperienceIDs[*exp.ExperienceID] = struct{}{}
		}
	}

	var config ExperienceSyncConfig
	err = yaml.Unmarshal([]byte(configData), &config)
	assert.NoError(t, err, "failed to unmarshal YAML")
	err = NormalizeExperiences(config.Experiences)
	assert.NoError(t, err, "invalid experiences")

	return currentState, config
}

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
	currentStateData := ``
	configData := `
experiences:
  - name: Test Experience
    description: This is a test experience
    locations:
      - s3://my-favorite-bucket/foo
    environmentVariables:
      - name: ENV_VAR_1
        value: value1
    cacheExempt: true
    containerTimeoutSeconds: 7200
managedExperienceTags:
managedTestSuites:
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	assert.Len(t, experienceUpdates.MatchedExperiencesByNewName, 1, "Should be one experience update")
	assert.Empty(t, experienceUpdates.TagUpdatesByName, "Should be no tag updates")

	match, exists := experienceUpdates.MatchedExperiencesByNewName[config.Experiences[0].Name]
	assert.True(t, exists, "Expected experience in updates")
	assert.Same(t, &config.Experiences[0], match.New, "Should be the same object (pointer equality)")
	assert.Nil(t, match.Original, "Experience is not new")
}

func TestArchiveSingleExperience(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: Test Experience
    description: This is a test experience
    experienceID: "3dd91177-1e66-426c-bf5b-fb46fe4a0c3b"
    locations:
      - s3://my-favorite-bucket/foo
    environmentVariables:
      - name: ENV_VAR_1
        value: value1
    cacheExempt: true
    containerTimeoutSeconds: 7200
`
	configData := `
experiences:
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	assert.Len(t, experienceUpdates.MatchedExperiencesByNewName, 1, "Should be one experience update")
	assert.Empty(t, experienceUpdates.TagUpdatesByName, "Should be no tag updates")

	match, exists := experienceUpdates.MatchedExperiencesByNewName["Test Experience"]
	assert.True(t, exists, "Expected experience in updates")
	assert.Same(t, currentState.ExperiencesByName["Test Experience"], match.Original, "Should be the same object (pointer equality)")

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
  - name: experience-to-update-by-name
    description: This is a test experience
    experienceID: "3dd91177-1e66-426c-bf5b-fb46fe4a0c3b"
    locations:
      - s3://my-favorite-bucket/foo
    environmentVariables:
      - name: ENV_VAR_1
        value: value1
    cacheExempt: true
    containerTimeoutSeconds: 7200
    archived: true  # This will be returned
  - name: experience-to-update-by-id
    description: This is a test experience2
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    locations:
      - s3://my-favorite-bucket/bar
    environmentVariables:
      - name: ENV_VAR_2
        value: value2
    cacheExempt: false
    containerTimeoutSeconds: 7200
`
	configData := `
experiences:
  - name: experience-to-update-by-name
    description: This is my new experience"
    locations:
      - s3://my-favorite-bucket/my-new-location
    environmentVariables:
      - name: ENV_VAR_1
        value: value_new
    cacheExempt: false
    containerTimeoutSeconds: 7300
  - name: new-name-for-experience-to-update-by-id
    description: This is a test experience2
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    locations:
      - s3://my-favorite-bucket/bar
    environmentVariables:
      - name: ENV_VAR_2
        value: value2
    cacheExempt: false
    containerTimeoutSeconds: 7300
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

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
		assert.Same(t, &config.Experiences[i], match.New, "Should be the same object (pointer equality)")
	}
}

func TestFailsOnAmbiguousRenaming(t *testing.T) {
	// SETUP
	// In this arrangement, the name of the first configured experience indicates it's the
	// update for current-experience. However, the ID for the second configured experience also
	// indicates it's the update for current-experience. This is a failure.
	currentStateData := `
  - name: current-experience
    description: Some experience
    locations: ["somewhere_over_the_rainbow"]
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	configData := `
experiences:
  - name: current-experience
    description: Some experience
    locations: ["somewhere_over_the_rainbow"]
  - name: new-name-for-current-experience
    description: Some experience
    locations: ["somewhere_over_the_rainbow"]
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.Error(t, err)

	// SETUP
	currentStateData = `
  - name: current-experience
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
`
	configData = `
experiences:
  - name: new-name-for-current-experience
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
  - name: current-experience
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config = loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION / VERIFICATION
	_, err = computeExperienceUpdates(&config, currentState)
	assert.Error(t, err)
}

func TestFailsOnNonExistentID(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: current-experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
experiences:
  - name: new-name-for-current-experience
    experienceID: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.Error(t, err)
}

func TestFailsOnDuplicateID(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: current-experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
experiences:
  - name: new-name-for-current-experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: other-new-name-for-current-experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.Error(t, err)
}

func TestFailsOnClobberingExistingWithRename(t *testing.T) {
	// SETUP
	//
	// Here we swap the names so they clobber each other. Technically this should be possible,
	// but there's no order of updates that could be made without the database state being
	// temporarily invalid. We don't try to do any fancy swapping or other logic so we just
	// don't support this.
	currentStateData := `
  - name: new-name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: old-name
    experienceID: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
experiences:
  - name: new-name
    experienceID: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: old-name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.Error(t, err)
}

func TestFailsOnNameCollision(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: new-name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: old-name
    experienceID: "8f8e2af7-28d4-4462-8025-d313ccb61bd2"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
experiences:
  - name: new-name
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: new-name
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)

	// ACTION / VERIFICATION
	_, err := computeExperienceUpdates(&config, currentState)
	assert.Error(t, err)
}

func TestAddRemoveTags(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: Test Experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    tags: []
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: Unrenamed Experience
    experienceID: "62501c04-3da2-4a46-94b1-ab90e32b2059"
    tags: []
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: Old regression experience
    experienceID: "cddca442-9c25-4c06-9023-a4edfe9258a3"
    tags: ["regression", "my-special-tag"]
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
managedExperienceTags:
  - regression
experiences:
  - name: Test Experience
    tags: ["regression"]
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: Renamed Experience
    experienceID: "62501c04-3da2-4a46-94b1-ab90e32b2059"
    tags: ["regression", "my-special-tag"]
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
  - name: Old regression experience
    tags: []
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, []string{"regression", "my-special-tag"}, nil)

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	tagUpdates, contains := experienceUpdates.TagUpdatesByName["regression"]
	assert.True(t, contains, "Tag 'regression' should be contained in TagUpdatesByName")

	addedDesiredExperience := slices.Contains(tagUpdates.Additions, &config.Experiences[0])
	assert.True(t, addedDesiredExperience, "Not going to tag desired experience")
	addedDesiredExperience = slices.Contains(tagUpdates.Additions, &config.Experiences[1])
	assert.True(t, addedDesiredExperience, "Not going to tag desired experience")
	removedDesiredExperience := slices.Contains(tagUpdates.Removals, &config.Experiences[2])
	assert.True(t, removedDesiredExperience, "Not going to untag desired experience")

	tagUpdates, contains = experienceUpdates.TagUpdatesByName["my-special-tag"]
	assert.True(t, contains, "Tag 'my-special-tag' should be contained in TagUpdatesByName")
	addedDesiredExperience = slices.Contains(tagUpdates.Additions, &config.Experiences[1])
	assert.True(t, addedDesiredExperience, "Not going to tag desired experience")

	// We should *NOT* remove the unmanaged "my-special-tag" tag
	removedDesiredExperience = slices.Contains(tagUpdates.Removals, &config.Experiences[2])
	assert.False(t, removedDesiredExperience, "Going to untag desired experience with unmanaged tag.")
}

func TestAddSystems(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: Test Experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    systems: []
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
experiences:
  - name: Test Experience But with New Name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    systems: ["planner"]
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, []string{"planner"})

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)

	systemUpdates, contains := experienceUpdates.SystemUpdatesByName["planner"]
	assert.True(t, contains, "System 'planner' should be contained in SystemUpdatesByName")

	addedDesiredExperience := slices.Contains(systemUpdates.Additions, &config.Experiences[0])
	assert.True(t, addedDesiredExperience, "Not going to add system desired experience")
}

func TestReviseTestSuite(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: Test Experience
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
managedTestSuites:
  - name: "Nightly CI"
    experiences:
     - Test Experience But with New Name
experiences:
  - name: Test Experience But with New Name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)
	testSuiteID := uuid.New()
	currentState.TestSuiteIDsByName = map[string]TestSuiteID{
		"Nightly CI": testSuiteID,
	}

	// ACTION
	experienceUpdates, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.NoError(t, err)
	assert.Len(t, experienceUpdates.TestSuiteUpdates, 1)
	update := experienceUpdates.TestSuiteUpdates[0]
	assert.Equal(t, update.Name, "Nightly CI")
	assert.Equal(t, update.TestSuiteID, testSuiteID)
	assert.Len(t, update.Experiences, 1)
	assert.Equal(t, update.Experiences[0], &config.Experiences[0])
}

func TestReviseTestSuiteFailOnOldName(t *testing.T) {
	// SETUP
	currentStateData := `
  - name: Test Experience Old Name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	configData := `
managedExperienceTags:
  - regression
managedTestSuites:
  - name: "Nightly CI"
    experiences:
     - Test Experience Old Name
experiences:
  - name: Test Experience But with New Name
    experienceID: "628eccf2-2621-4fdf-a8d8-c6b057ce2f0d"
    description: Some current experience
    locations: ["somewhere_over_the_rainbow"]
`
	currentState, config := loaderHelper(t, currentStateData, configData, nil, nil)
	testSuiteID := uuid.New()
	currentState.TestSuiteIDsByName = map[string]TestSuiteID{
		"Nightly CI": testSuiteID,
	}

	// ACTION
	_, err := computeExperienceUpdates(&config, currentState)

	// VERIFICATION
	assert.Error(t, err)
}
