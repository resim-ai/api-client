package sync

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
)

func TestCreateExperience(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	var expectedProjectID = uuid.New()
	var expectedExperienceID = uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
customFields:
  - name: foo
    type: text
    values:
      - bar
      - baz
cacheExempt: true
containerTimeoutSeconds: 7200
`
	creationMatch := ExperienceMatch{
		Original: nil,
		New:      &Experience{},
	}
	err := yaml.Unmarshal([]byte(experienceData), creationMatch.New)
	assert.NoError(t, err, "failed to unmarshal YAML")

	var createdExperience Experience
	client.On("CreateExperienceWithResponse",
		context.Background(),
		expectedProjectID,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, body api.CreateExperienceInput, reqEditors ...api.RequestEditorFn) (*api.CreateExperienceResponse, error) {
		createdExperience.Name = body.Name
		createdExperience.ExperienceID = &expectedExperienceID
		createdExperience.Description = body.Description
		createdExperience.Locations = *body.Locations
		createdExperience.Profile = body.Profile
		createdExperience.EnvironmentVariables = body.EnvironmentVariables
		createdExperience.CacheExempt = body.CacheExempt
		createdExperience.ContainerTimeoutSeconds = body.ContainerTimeoutSeconds
		createdExperience.CustomFields = body.CustomFields

		return &api.CreateExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201: &api.Experience{
				Name:                    body.Name,
				ExperienceID:            expectedExperienceID,
				Description:             body.Description,
				Locations:               *body.Locations,
				Profile:                 *body.Profile,
				EnvironmentVariables:    *body.EnvironmentVariables,
				CacheExempt:             *body.CacheExempt,
				ContainerTimeoutSeconds: *body.ContainerTimeoutSeconds,
				CustomFields:            *body.CustomFields,
				Archived:                false,
			},
		}, nil
	})

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{creationMatch.New.Name: creationMatch},
		TagUpdatesByName:            make(map[string]*TagUpdates),
		SystemUpdatesByName:         make(map[string]*SystemUpdates),
	}

	// ACTION
	err = applyUpdates(&client, expectedProjectID, updates)

	// VERIFICATION
	assert.NoError(t, err)
	assert.Equal(t, createdExperience, *creationMatch.New)
	// Verify that the experience ID has been set
	assert.Equal(t, *creationMatch.New.ExperienceID, expectedExperienceID)
}

func TestArchiveExperience(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	var expectedProjectID = uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
cacheExempt: true
containerTimeoutSeconds: 7200
`
	archiveMatch := ExperienceMatch{
		Original: &Experience{},
		New:      &Experience{},
	}
	err := yaml.Unmarshal([]byte(experienceData), archiveMatch.Original)
	assert.NoError(t, err, "failed to unmarshal YAML")
	archiveMatch.Original.ExperienceID = Ptr(uuid.New())
	*archiveMatch.New = *archiveMatch.Original
	archiveMatch.New.Archived = true

	// Another match for an experience we don't want to archive
	dontArchiveMatch := ExperienceMatch{
		Original: &Experience{},
		New:      &Experience{},
	}
	*dontArchiveMatch.Original = *archiveMatch.Original
	dontArchiveMatch.Original.ExperienceID = Ptr(uuid.New())
	dontArchiveMatch.Original.Name = "Don't archive me, bro"
	*dontArchiveMatch.New = *dontArchiveMatch.Original

	client.On("BulkArchiveExperiencesWithResponse",
		context.Background(),
		expectedProjectID,
		mock.Anything,
	).Return(func(ctx context.Context,
		projectID api.ProjectID,
		body api.BulkArchiveExperiencesInput,
		reqEditors ...api.RequestEditorFn) (*api.BulkArchiveExperiencesResponse, error) {
		assert.Len(t, body.ExperienceIDs, 1)
		assert.Equal(t, body.ExperienceIDs[0], *archiveMatch.Original.ExperienceID)
		return &api.BulkArchiveExperiencesResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		}, nil
	}).Once()

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{archiveMatch.New.Name: archiveMatch},
		TagUpdatesByName:            make(map[string]*TagUpdates),
		SystemUpdatesByName:         make(map[string]*SystemUpdates),
	}

	// ACTION / VERIFICATION
	err = applyUpdates(&client, expectedProjectID, updates)
	assert.NoError(t, err)
	client.AssertNumberOfCalls(t, "BulkArchiveExperiencesWithResponse", 1)
}

func TestRestoreAndUpdateExperience(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	var expectedProjectID = uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
customFields:
  - name: foo
    type: text
    values:
      - bar
      - baz
cacheExempt: true
containerTimeoutSeconds: 7200
`
	updateMatch := ExperienceMatch{
		Original: &Experience{},
		New:      &Experience{},
	}
	err := yaml.Unmarshal([]byte(experienceData), updateMatch.Original)
	assert.NoError(t, err, "failed to unmarshal YAML")
	updateMatch.Original.ExperienceID = Ptr(uuid.New())
	*updateMatch.New = *updateMatch.Original
	updateMatch.Original.Archived = true
	updateMatch.New.Archived = false

	var updatedExperience Experience

	client.On("UpdateExperienceWithResponse",
		context.Background(),
		expectedProjectID,
		*updateMatch.Original.ExperienceID,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, experienceID api.ExperienceID,
		body api.UpdateExperienceInput,
		reqEditors ...api.RequestEditorFn) (*api.UpdateExperienceResponse, error) {
		updatedExperience.Name = *body.Experience.Name
		updatedExperience.ExperienceID = &experienceID
		updatedExperience.Description = *body.Experience.Description
		updatedExperience.Locations = *body.Experience.Locations
		updatedExperience.Profile = body.Experience.Profile
		updatedExperience.EnvironmentVariables = body.Experience.EnvironmentVariables
		updatedExperience.CacheExempt = body.Experience.CacheExempt
		updatedExperience.ContainerTimeoutSeconds = body.Experience.ContainerTimeoutSeconds
		updatedExperience.CustomFields = body.Experience.CustomFields
		updatedExperience.Archived = updateMatch.New.Archived

		return &api.UpdateExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Experience{
				Name:                    *body.Experience.Name,
				ExperienceID:            experienceID,
				Description:             *body.Experience.Description,
				Locations:               *body.Experience.Locations,
				Profile:                 *body.Experience.Profile,
				EnvironmentVariables:    *body.Experience.EnvironmentVariables,
				CacheExempt:             *body.Experience.CacheExempt,
				ContainerTimeoutSeconds: *body.Experience.ContainerTimeoutSeconds,
				CustomFields:            *body.Experience.CustomFields,
				Archived:                false,
			},
		}, nil

	})

	client.On("RestoreExperienceWithResponse",
		context.Background(),
		expectedProjectID,
		*updateMatch.Original.ExperienceID,
		mock.Anything,
	).Return(&api.RestoreExperienceResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
	}, nil)

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{updateMatch.New.Name: updateMatch},
		TagUpdatesByName:            make(map[string]*TagUpdates),
		SystemUpdatesByName:         make(map[string]*SystemUpdates),
	}

	// ACTION / VERIFICATION
	err = applyUpdates(&client, expectedProjectID, updates)
	assert.NoError(t, err)
	client.AssertNumberOfCalls(t, "UpdateExperienceWithResponse", 1)
	client.AssertNumberOfCalls(t, "RestoreExperienceWithResponse", 1)
	assert.Equal(t, updatedExperience, *updateMatch.New)
}

func TestAddTags(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	var expectedProjectID = uuid.New()
	var expectedTagID = uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
cacheExempt: true
containerTimeoutSeconds: 7200
`

	experienceToTag := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToTag)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToTag.ExperienceID = Ptr(uuid.New())

	client.On("AddTagsToExperiencesWithResponse",
		context.Background(),
		expectedProjectID,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, body api.AddTagsToExperiencesInput,
		reqEditors ...api.RequestEditorFn) (*api.AddTagsToExperiencesResponse, error) {
		assert.Len(t, body.ExperienceTagIDs, 1)
		assert.Equal(t, body.ExperienceTagIDs[0], expectedTagID)
		assert.NotEqual(t, body.Experiences, nil)
		assert.Len(t, *body.Experiences, 1)
		assert.Equal(t, (*body.Experiences)[0], *experienceToTag.ExperienceID)
		return &api.AddTagsToExperiencesResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
		}, nil
	})

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{},
		TagUpdatesByName: map[string]*TagUpdates{
			experienceToTag.Name: {
				Name:      "regression",
				TagID:     expectedTagID,
				Additions: []*Experience{experienceToTag},
			},
		},
		SystemUpdatesByName: make(map[string]*SystemUpdates),
	}

	// ACTION
	err = applyUpdates(&client, expectedProjectID, updates)
	assert.NoError(t, err)

	// VERIFICATION
	client.AssertNumberOfCalls(t, "AddTagsToExperiencesWithResponse", 1)
}

func TestRemoveTag(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	var expectedProjectID = uuid.New()
	var expectedTagID = uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
cacheExempt: true
containerTimeoutSeconds: 7200
`

	experienceToUnTag := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToUnTag)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToUnTag.ExperienceID = Ptr(uuid.New())

	client.On("RemoveExperienceTagFromExperienceWithResponse",
		context.Background(),
		expectedProjectID,
		expectedTagID,
		*experienceToUnTag.ExperienceID,
	).Return(&api.RemoveExperienceTagFromExperienceResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
	}, nil)
	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{},
		TagUpdatesByName: map[string]*TagUpdates{
			experienceToUnTag.Name: {
				Name:     "regression",
				TagID:    expectedTagID,
				Removals: []*Experience{experienceToUnTag},
			},
		},
		SystemUpdatesByName: make(map[string]*SystemUpdates),
	}

	// ACTION
	err = applyUpdates(&client, expectedProjectID, updates)
	assert.NoError(t, err)

	// VERIFICATION
	client.AssertNumberOfCalls(t, "RemoveExperienceTagFromExperienceWithResponse", 1)
}

func TestAddSystemsToExperience(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	var expectedProjectID = uuid.New()
	var expectedSystemID = uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
cacheExempt: true
containerTimeoutSeconds: 7200
`
	experienceToAddToSystem := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToAddToSystem)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToAddToSystem.ExperienceID = Ptr(uuid.New())

	client.On("AddSystemsToExperiencesWithResponse",
		context.Background(),
		expectedProjectID,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, body api.MutateSystemsToExperienceInput,
		reqEditors ...api.RequestEditorFn) (*api.AddSystemsToExperiencesResponse, error) {
		assert.Len(t, body.SystemIDs, 1)
		assert.Equal(t, body.SystemIDs[0], expectedSystemID)
		assert.NotEqual(t, body.Experiences, nil)
		assert.Len(t, *body.Experiences, 1)
		assert.Equal(t, (*body.Experiences)[0], *experienceToAddToSystem.ExperienceID)
		return &api.AddSystemsToExperiencesResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
		}, nil
	})

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{},
		SystemUpdatesByName: map[string]*SystemUpdates{
			experienceToAddToSystem.Name: {
				Name:      "regression",
				SystemID:  expectedSystemID,
				Additions: []*Experience{experienceToAddToSystem},
			},
		},
	}

	// ACTION
	err = applyUpdates(&client, expectedProjectID, updates)
	assert.NoError(t, err)

	// VERIFICATION
	client.AssertNumberOfCalls(t, "AddSystemsToExperiencesWithResponse", 1)
}

func TestReviseTestSuiteApply(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	expectedProjectID := uuid.New()
	expectedTestSuiteID := uuid.New()
	expectedExperienceID := uuid.New()

	experienceData := `
name: Test Experience
description: This is a test experience
locations:
  - s3://my-favorite-bucket/foo
profile: ""
environmentVariables:
  - name: ENV_VAR_1
    value: value1
cacheExempt: true
containerTimeoutSeconds: 7200
`
	experienceToAddToTestSuite := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToAddToTestSuite)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToAddToTestSuite.ExperienceID = &expectedExperienceID

	client.On("ReviseTestSuiteWithResponse",
		context.Background(),
		expectedProjectID,
		expectedTestSuiteID,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, testSuiteID api.TestSuiteID, body api.ReviseTestSuiteInput,
		reqEditors ...api.RequestEditorFn) (*api.ReviseTestSuiteResponse, error) {
		// VERIFICATION
		assert.Len(t, *body.Experiences, 1)
		assert.Equal(t, (*body.Experiences)[0], expectedExperienceID)
		return &api.ReviseTestSuiteResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		}, nil
	})

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{},
		TestSuiteUpdates: []TestSuiteUpdate{
			{
				Name:        "regression",
				TestSuiteID: expectedTestSuiteID,
				Experiences: []*Experience{experienceToAddToTestSuite},
			},
		},
	}

	// ACTION
	err = applyUpdates(&client, expectedProjectID, updates)
	assert.NoError(t, err)

	// VERIFICATION
	client.AssertNumberOfCalls(t, "ReviseTestSuiteWithResponse", 1)
}
