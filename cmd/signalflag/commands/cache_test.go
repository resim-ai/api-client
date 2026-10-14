package commands

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCacheMethodFromFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		method  *string
		exempt  *bool
		want    *api.CacheMethod
		wantErr bool
	}{
		{name: "omitted uses server default"},
		{name: "none", method: Ptr("NONE"), want: Ptr(api.NONE)},
		{name: "ebs", method: Ptr("EBS"), want: Ptr(api.EBS)},
		{name: "efs lowercase", method: Ptr("efs"), want: Ptr(api.EFS)},
		{name: "empty", method: Ptr(""), wantErr: true},
		{name: "invalid", method: Ptr("other"), wantErr: true},
		{name: "reserved", method: Ptr("S3_MOUNTPOINT"), wantErr: true},
		{name: "legacy alone", exempt: Ptr(true)},
		{name: "agreeing legacy", method: Ptr("NONE"), exempt: Ptr(true), want: Ptr(api.NONE)},
		{name: "cached legacy", method: Ptr("EFS"), exempt: Ptr(false), want: Ptr(api.EFS)},
		{name: "conflicting exemption", method: Ptr("EFS"), exempt: Ptr(true), wantErr: true},
		{name: "conflicting caching", method: Ptr("NONE"), exempt: Ptr(false), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			if tc.method != nil {
				viper.Set(cacheMethodKey, *tc.method)
			}
			if tc.exempt != nil {
				viper.Set(assetCacheExemptKey, *tc.exempt)
			}
			got, err := cacheMethodFromFlags(true)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func (s *CommandsSuite) TestCreateExperienceCacheMethod() {
	viper.Reset()
	defer viper.Reset()
	projectID, experienceID := uuid.New(), uuid.New()
	viper.Set(experienceProjectKey, projectID.String())
	viper.Set(experienceNameKey, "cached-experience")
	viper.Set(experienceDescriptionKey, "An EFS experience")
	viper.Set(experienceLocationsKey, []string{"s3://bucket/data"})
	viper.Set(cacheMethodKey, "EFS")
	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, JSON200: &api.Project{ProjectID: projectID}}, nil)
	s.mockClient.On("CreateExperienceWithResponse", matchContext, projectID,
		mock.MatchedBy(func(body api.CreateExperienceInput) bool {
			return body.CacheMethod != nil && *body.CacheMethod == api.EFS && body.CacheExempt == nil
		})).Return(&api.CreateExperienceResponse{HTTPResponse: &http.Response{StatusCode: http.StatusCreated}, JSON201: &api.Experience{ExperienceID: experienceID}}, nil)
	s.mockClient.On("ValidateExperienceLocationWithResponse", matchContext, mock.Anything).Return(
		&api.ValidateExperienceLocationResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}}, nil)
	createExperience(nil, nil)
}

func (s *CommandsSuite) TestUpdateExperienceCacheMethod() {
	viper.Reset()
	defer viper.Reset()
	projectID, experienceID := uuid.New(), uuid.New()
	viper.Set(experienceProjectKey, projectID.String())
	viper.Set(experienceKey, experienceID.String())
	viper.Set(cacheMethodKey, "EFS")
	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, JSON200: &api.Project{ProjectID: projectID}}, nil)
	s.mockClient.On("GetExperienceWithResponse", matchContext, projectID, experienceID).Return(
		&api.GetExperienceResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, JSON200: &api.Experience{ExperienceID: experienceID}}, nil)
	s.mockClient.On("UpdateExperienceWithResponse", matchContext, projectID, experienceID,
		mock.MatchedBy(func(body api.UpdateExperienceInput) bool {
			return body.Experience.CacheMethod != nil && *body.Experience.CacheMethod == api.EFS &&
				body.Experience.CacheExempt == nil && body.UpdateMask != nil &&
				len(*body.UpdateMask) == 1 && (*body.UpdateMask)[0] == "cacheMethod"
		})).Return(&api.UpdateExperienceResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}}, nil)
	updateExperience(nil, nil)
}

func (s *CommandsSuite) TestCreateAssetCacheMethod() {
	viper.Reset()
	defer viper.Reset()
	projectID, assetID := uuid.New(), uuid.New()
	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetNameKey, "cached-asset")
	viper.Set(assetDescriptionKey, "An EFS asset")
	viper.Set(assetLocationsKey, "s3://bucket/data")
	viper.Set(assetMountFolderKey, "/data")
	viper.Set(assetVersionKey, "v1")
	viper.Set(cacheMethodKey, "EFS")
	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, JSON200: &api.Project{ProjectID: projectID}}, nil)
	s.mockClient.On("CreateAssetWithResponse", matchContext, projectID,
		mock.MatchedBy(func(body api.CreateAssetInput) bool {
			return body.CacheMethod != nil && *body.CacheMethod == api.EFS && body.CacheExempt == nil
		})).Return(&api.CreateAssetResponse{HTTPResponse: &http.Response{StatusCode: http.StatusCreated}, JSON201: &api.Asset{AssetID: assetID}}, nil)
	createAsset(nil, nil)
}

func (s *CommandsSuite) TestReviseAssetCacheMethod() {
	viper.Reset()
	defer viper.Reset()
	projectID, assetID := uuid.New(), uuid.New()
	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())
	viper.Set(assetLocationsKey, "s3://bucket/data")
	viper.Set(assetVersionKey, "v2")
	viper.Set(cacheMethodKey, "NONE")
	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, JSON200: &api.Project{ProjectID: projectID}}, nil)
	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, JSON200: &api.Asset{AssetID: assetID, CacheMethod: api.EFS}}, nil)
	s.mockClient.On("ReviseAssetWithResponse", matchContext, projectID, assetID,
		mock.MatchedBy(func(body api.ReviseAssetInput) bool {
			return body.CacheMethod != nil && *body.CacheMethod == api.NONE
		})).Return(&api.ReviseAssetResponse{HTTPResponse: &http.Response{StatusCode: http.StatusCreated}, JSON201: &api.Asset{AssetID: assetID}}, nil)
	reviseAsset(nil, nil)
}
