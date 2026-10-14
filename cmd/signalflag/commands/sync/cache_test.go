package sync

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSyncCacheMethodValidation(t *testing.T) {
	for _, tc := range []struct {
		cache   string
		wantErr bool
	}{
		{cache: ""},
		{cache: "cacheMethod: NONE"},
		{cache: "cacheMethod: EBS"},
		{cache: "cacheMethod: EFS"},
		{cache: "cacheExempt: true"},
		{cache: "cacheExempt: false"},
		{cache: "cacheMethod: NONE\ncacheExempt: true"},
		{cache: "cacheMethod: EFS\ncacheExempt: false"},
		{cache: "cacheMethod: S3_MOUNTPOINT", wantErr: true},
		{cache: "cacheMethod: other", wantErr: true},
		{cache: "cacheMethod: EFS\ncacheExempt: true", wantErr: true},
		{cache: "cacheMethod: NONE\ncacheExempt: false", wantErr: true},
	} {
		t.Run(tc.cache, func(t *testing.T) {
			var experience Experience
			require.NoError(t, yaml.Unmarshal([]byte("name: test\ndescription: test\nlocations: [s3://bucket/data]\n"+tc.cache), &experience))
			err := normalizeExperience(&experience)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSyncSendsCacheMethod(t *testing.T) {
	for _, method := range []api.CacheMethod{api.NONE, api.EBS, api.EFS} {
		for _, create := range []bool{true, false} {
			t.Run(string(method)+map[bool]string{true: "/create", false: "/update"}[create], func(t *testing.T) {
				client := mockapiclient.NewClientWithResponsesInterface(t)
				projectID, experienceID := uuid.New(), uuid.New()
				match := ExperienceMatch{New: &Experience{Name: "test", Description: "test", Locations: []string{"s3://bucket/data"}, CacheMethod: &method}}
				if create {
					client.On("CreateExperienceWithResponse", context.Background(), projectID,
						mock.MatchedBy(func(body api.CreateExperienceInput) bool {
							return body.CacheMethod != nil && *body.CacheMethod == method && body.CacheExempt == nil
						})).Return(&api.CreateExperienceResponse{HTTPResponse: &http.Response{StatusCode: http.StatusCreated}, JSON201: &api.Experience{ExperienceID: experienceID}}, nil)
				} else {
					match.Original = &Experience{Name: "test"}
					match.New.ExperienceID = &experienceID
					client.On("UpdateExperienceWithResponse", context.Background(), projectID, experienceID,
						mock.MatchedBy(func(body api.UpdateExperienceInput) bool {
							if body.Experience.CacheMethod == nil || *body.Experience.CacheMethod != method || body.Experience.CacheExempt != nil {
								return false
							}
							for _, field := range *body.UpdateMask {
								if field == "cacheMethod" {
									return true
								}
							}
							return false
						})).Return(&api.UpdateExperienceResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}}, nil)
				}
				require.NoError(t, updateSingleExperience(client, projectID, match))
			})
		}
	}
}

func TestSyncOmittedCacheMethodPreservesExistingTier(t *testing.T) {
	client := mockapiclient.NewClientWithResponsesInterface(t)
	projectID, experienceID := uuid.New(), uuid.New()
	match := ExperienceMatch{
		Original: &Experience{Name: "test", CacheMethod: Ptr(api.EFS)},
		New:      &Experience{Name: "test", Description: "test", Locations: []string{"s3://bucket/data"}, ExperienceID: &experienceID},
	}
	client.On("UpdateExperienceWithResponse", context.Background(), projectID, experienceID,
		mock.MatchedBy(func(body api.UpdateExperienceInput) bool {
			if body.Experience.CacheMethod != nil || body.Experience.CacheExempt != nil {
				return false
			}
			for _, field := range *body.UpdateMask {
				if field == "cacheMethod" || field == "cacheExempt" {
					return false
				}
			}
			return true
		})).Return(&api.UpdateExperienceResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}}, nil)
	require.NoError(t, updateSingleExperience(client, projectID, match))
}

func TestClonePreservesCacheMethod(t *testing.T) {
	for _, method := range []api.CacheMethod{api.NONE, api.EBS, api.EFS} {
		t.Run(string(method), func(t *testing.T) {
			experiences := make(map[string]*Experience)
			addApiExperienceToExperienceMap(api.Experience{Name: "test", Description: "test", Locations: []string{"s3://bucket/data"}, CacheMethod: method, CacheExempt: method == api.NONE}, experiences)
			data, err := yaml.Marshal(experiences["test"])
			require.NoError(t, err)
			require.Contains(t, string(data), "cacheMethod: "+string(method))
			require.NotContains(t, string(data), "cacheExempt:")
			var cloned Experience
			require.NoError(t, yaml.Unmarshal(data, &cloned))
			require.Equal(t, Ptr(method), cloned.CacheMethod)
			// A cloned NONE config can switch to EFS without a conflicting legacy field.
			cloned.CacheMethod = Ptr(api.EFS)
			require.NoError(t, normalizeExperience(&cloned))
		})
	}
}
