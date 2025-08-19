package sync

import (
	"context"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
	"net/http"
	"testing"
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
environment_variables:
  - name: ENV_VAR_1
    value: value1
cache_exempt: true
container_timeout_seconds: 7200
`
	creationMatch := ExperienceMatch{
		Original: nil,
		New:      &Experience{},
	}
	err := yaml.Unmarshal([]byte(experienceData), creationMatch.New)
	assert.NoError(t, err, "failed to unmarshal YAML")

	var createdExperience Experience
	client.On("CreateExperienceWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, body api.CreateExperienceInput, reqEditors ...api.RequestEditorFn) (*api.CreateExperienceResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		createdExperience.Name = body.Name
		createdExperience.ExperienceID = &ExperienceIDWrapper{ID: expectedExperienceID}
		createdExperience.Description = body.Description
		createdExperience.Locations = *body.Locations
		createdExperience.Profile = body.Profile
		createdExperience.EnvironmentVariables = body.EnvironmentVariables
		createdExperience.CacheExempt = *body.CacheExempt
		createdExperience.ContainerTimeoutSeconds = body.ContainerTimeoutSeconds

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
	applyUpdates(&client, expectedProjectID, updates)
	assert.Equal(t, createdExperience, *creationMatch.New)
	// Verify that the experience ID has been set
	assert.Equal(t, creationMatch.New.ExperienceID.ID, expectedExperienceID)
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
environment_variables:
  - name: ENV_VAR_1
    value: value1
cache_exempt: true
container_timeout_seconds: 7200
`
	archiveMatch := ExperienceMatch{
		Original: &Experience{},
		New:      &Experience{},
	}
	err := yaml.Unmarshal([]byte(experienceData), archiveMatch.Original)
	assert.NoError(t, err, "failed to unmarshal YAML")
	archiveMatch.Original.ExperienceID = &ExperienceIDWrapper{ID: uuid.New()}
	*archiveMatch.New = *archiveMatch.Original
	archiveMatch.New.Archived = true

	client.On("ArchiveExperienceWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, experienceID api.ExperienceID, reqEditors ...api.RequestEditorFn) (*api.ArchiveExperienceResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		assert.Equal(t, experienceID, archiveMatch.Original.ExperienceID.ID)

		return &api.ArchiveExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil
	})

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{archiveMatch.New.Name: archiveMatch},
		TagUpdatesByName:            make(map[string]*TagUpdates),
		SystemUpdatesByName:         make(map[string]*SystemUpdates),
	}

	// ACTION / VERIFICATION
	applyUpdates(&client, expectedProjectID, updates)
	client.AssertNumberOfCalls(t, "ArchiveExperienceWithResponse", 1)
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
environment_variables:
  - name: ENV_VAR_1
    value: value1
cache_exempt: true
container_timeout_seconds: 7200
`
	updateMatch := ExperienceMatch{
		Original: &Experience{},
		New:      &Experience{},
	}
	err := yaml.Unmarshal([]byte(experienceData), updateMatch.Original)
	assert.NoError(t, err, "failed to unmarshal YAML")
	updateMatch.Original.ExperienceID = &ExperienceIDWrapper{ID: uuid.New()}
	*updateMatch.New = *updateMatch.Original
	updateMatch.Original.Archived = true
	updateMatch.New.Archived = false

	var updatedExperience Experience

	client.On("UpdateExperienceWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, experienceID api.ExperienceID,
		body api.UpdateExperienceInput,
		reqEditors ...api.RequestEditorFn) (*api.UpdateExperienceResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		assert.Equal(t, experienceID, updateMatch.Original.ExperienceID.ID)
		updatedExperience.Name = *body.Experience.Name
		updatedExperience.ExperienceID = &ExperienceIDWrapper{ID: experienceID}
		updatedExperience.Description = *body.Experience.Description
		updatedExperience.Locations = *body.Experience.Locations
		updatedExperience.Profile = body.Experience.Profile
		updatedExperience.EnvironmentVariables = body.Experience.EnvironmentVariables
		updatedExperience.CacheExempt = *body.Experience.CacheExempt
		updatedExperience.ContainerTimeoutSeconds = body.Experience.ContainerTimeoutSeconds

		return &api.UpdateExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON200: &api.Experience{
				Name:                    *body.Experience.Name,
				ExperienceID:            experienceID,
				Description:             *body.Experience.Description,
				Locations:               *body.Experience.Locations,
				Profile:                 *body.Experience.Profile,
				EnvironmentVariables:    *body.Experience.EnvironmentVariables,
				CacheExempt:             *body.Experience.CacheExempt,
				ContainerTimeoutSeconds: *body.Experience.ContainerTimeoutSeconds,
				Archived:                false,
			},
		}, nil

	})

	client.On("RestoreExperienceWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, experienceID api.ExperienceID,
		reqEditors ...api.RequestEditorFn) (*api.RestoreExperienceResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		assert.Equal(t, experienceID, updateMatch.Original.ExperienceID.ID)
		return &api.RestoreExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil

	})

	updates := ExperienceUpdates{
		MatchedExperiencesByNewName: map[string]ExperienceMatch{updateMatch.New.Name: updateMatch},
		TagUpdatesByName:            make(map[string]*TagUpdates),
		SystemUpdatesByName:         make(map[string]*SystemUpdates),
	}

	// ACTION / VERIFICATION
	applyUpdates(&client, expectedProjectID, updates)
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
environment_variables:
  - name: ENV_VAR_1
    value: value1
cache_exempt: true
container_timeout_seconds: 7200
`

	experienceToTag := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToTag)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToTag.ExperienceID = &ExperienceIDWrapper{ID: uuid.New()}

	client.On("AddTagsToExperiencesWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, body api.AddTagsToExperiencesInput,
		reqEditors ...api.RequestEditorFn) (*api.AddTagsToExperiencesResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		assert.Equal(t, len(body.ExperienceTagIDs), 1)
		assert.Equal(t, body.ExperienceTagIDs[0], expectedTagID)
		assert.NotEqual(t, body.Experiences, nil)
		assert.Equal(t, len(*body.Experiences), 1)
		assert.Equal(t, (*body.Experiences)[0], experienceToTag.ExperienceID.ID)
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
	applyUpdates(&client, expectedProjectID, updates)
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
environment_variables:
  - name: ENV_VAR_1
    value: value1
cache_exempt: true
container_timeout_seconds: 7200
`

	experienceToUnTag := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToUnTag)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToUnTag.ExperienceID = &ExperienceIDWrapper{ID: uuid.New()}

	client.On("RemoveExperienceTagFromExperienceWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, experienceTagID api.ExperienceTagID, experienceID api.ExperienceID, reqEditors ...api.RequestEditorFn) (*api.RemoveExperienceTagFromExperienceResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		assert.Equal(t, experienceTagID, expectedTagID)
		assert.Equal(t, experienceID, experienceToUnTag.ExperienceID.ID)
		return &api.RemoveExperienceTagFromExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil
	})
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
	applyUpdates(&client, expectedProjectID, updates)
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
environment_variables:
  - name: ENV_VAR_1
    value: value1
cache_exempt: true
container_timeout_seconds: 7200
`
	experienceToAddToSystem := &Experience{}
	err := yaml.Unmarshal([]byte(experienceData), experienceToAddToSystem)
	assert.NoError(t, err, "failed to unmarshal YAML")
	experienceToAddToSystem.ExperienceID = &ExperienceIDWrapper{ID: uuid.New()}

	client.On("AddSystemsToExperiencesWithResponse",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, projectID api.ProjectID, body api.MutateSystemsToExperienceInput,
		reqEditors ...api.RequestEditorFn) (*api.AddSystemsToExperiencesResponse, error) {
		assert.Equal(t, projectID, expectedProjectID)
		assert.Equal(t, len(body.SystemIDs), 1)
		assert.Equal(t, body.SystemIDs[0], expectedSystemID)
		assert.NotEqual(t, body.Experiences, nil)
		assert.Equal(t, len(*body.Experiences), 1)
		assert.Equal(t, (*body.Experiences)[0], experienceToAddToSystem.ExperienceID.ID)
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
	applyUpdates(&client, expectedProjectID, updates)
	client.AssertNumberOfCalls(t, "AddSystemsToExperiencesWithResponse", 1)
}
