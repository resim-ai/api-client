package commands

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

func (s *CommandsSuite) TestActualGetAssetByID() {
	projectID := uuid.New()
	assetID := uuid.New()
	assetName := "test-asset"

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Asset{
				AssetID:       assetID,
				AssetRevision: 1,
				Name:          assetName,
			},
		}, nil)

	asset := actualGetAsset(projectID, assetID.String(), nil, false)
	s.NotNil(asset)
	s.Equal(assetID, asset.AssetID)
	s.Equal(assetName, asset.Name)
	s.Equal(int64(1), asset.AssetRevision)
}

func (s *CommandsSuite) TestActualGetAssetByName() {
	projectID := uuid.New()
	assetID := uuid.New()
	assetName := "my-lidar-data"

	s.mockClient.On("ListAssetsWithResponse", matchContext, projectID,
		mock.AnythingOfType("*api.ListAssetsParams")).Return(
		&api.ListAssetsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetsOutput{
				Assets: []api.Asset{
					{
						AssetID:       uuid.New(),
						AssetRevision: 1,
						Name:          "other-asset",
					},
					{
						AssetID:       assetID,
						AssetRevision: 3,
						Name:          assetName,
					},
				},
				NextPageToken: "",
			},
		}, nil)

	asset := actualGetAsset(projectID, assetName, nil, false)
	s.NotNil(asset)
	s.Equal(assetID, asset.AssetID)
	s.Equal(assetName, asset.Name)
	s.Equal(int64(3), asset.AssetRevision)
}

func (s *CommandsSuite) TestActualGetAssetWithRevision() {
	projectID := uuid.New()
	assetID := uuid.New()
	assetName := "versioned-asset"

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Asset{
				AssetID:       assetID,
				AssetRevision: 5,
				Name:          assetName,
			},
		}, nil)

	s.mockClient.On("GetAssetRevisionWithResponse", matchContext, projectID, assetID, int64(2)).Return(
		&api.GetAssetRevisionResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Asset{
				AssetID:       assetID,
				AssetRevision: 2,
				Name:          assetName,
			},
		}, nil)

	rev := int64(2)
	asset := actualGetAsset(projectID, assetID.String(), &rev, false)
	s.NotNil(asset)
	s.Equal(assetID, asset.AssetID)
	s.Equal(int64(2), asset.AssetRevision)
}

func (s *CommandsSuite) TestCreateAsset() {
	projectID := uuid.New()
	assetID := uuid.New()

	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetNameKey, "my-new-asset")
	viper.Set(assetDescriptionKey, "A test asset")
	viper.Set(assetLocationsKey, "s3://bucket/path1,s3://bucket/path2")
	viper.Set(assetMountFolderKey, "/data/assets")
	viper.Set(assetVersionKey, "v1.0")

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Project{
				ProjectID: projectID,
				Name:      "test-project",
			},
		}, nil)

	s.mockClient.On("CreateAssetWithResponse", matchContext, projectID,
		mock.MatchedBy(func(body api.CreateAssetInput) bool {
			return body.Name == "my-new-asset" &&
				body.Description == "A test asset" &&
				len(body.Locations) == 2 &&
				body.Locations[0] == "s3://bucket/path1" &&
				body.Locations[1] == "s3://bucket/path2" &&
				body.MountFolder == "/data/assets" &&
				body.Version == "v1.0"
		})).Return(
		&api.CreateAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201: &api.Asset{
				AssetID:       assetID,
				AssetRevision: 1,
				Name:          "my-new-asset",
				Description:   "A test asset",
				Locations:     []string{"s3://bucket/path1", "s3://bucket/path2"},
				MountFolder:   "/data/assets",
				Version:       "v1.0",
			},
		}, nil)

	createAsset(nil, nil)
}

func (s *CommandsSuite) TestListAssets() {
	projectID := uuid.New()

	viper.Set(assetProjectKey, projectID.String())

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Project{
				ProjectID: projectID,
				Name:      "test-project",
			},
		}, nil)

	s.mockClient.On("ListAssetsWithResponse", matchContext, projectID,
		mock.AnythingOfType("*api.ListAssetsParams")).Return(
		&api.ListAssetsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetsOutput{
				Assets: []api.Asset{
					{AssetID: uuid.New(), Name: "asset-1", AssetRevision: 1},
					{AssetID: uuid.New(), Name: "asset-2", AssetRevision: 2},
				},
				NextPageToken: "",
			},
		}, nil)

	listAssets(nil, nil)
}

func (s *CommandsSuite) TestGetAssetAllRevisions() {
	projectID := uuid.New()
	assetID := uuid.New()

	viper.Reset()
	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())
	viper.Set(assetAllRevisionsKey, true)

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 3, Name: "my-asset"},
		}, nil)

	s.mockClient.On("ListAssetRevisionsWithResponse", matchContext, projectID, assetID,
		mock.AnythingOfType("*api.ListAssetRevisionsParams")).Return(
		&api.ListAssetRevisionsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetRevisionsOutput{
				Assets: []api.Asset{
					{AssetID: assetID, AssetRevision: 1},
					{AssetID: assetID, AssetRevision: 2},
					{AssetID: assetID, AssetRevision: 3},
				},
				NextPageToken: "",
			},
		}, nil)

	getAsset(nil, nil)
}

func (s *CommandsSuite) TestUpdateAsset() {
	projectID := uuid.New()
	assetID := uuid.New()

	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())
	viper.Set(assetNameKey, "renamed-asset")

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 1, Name: "old-name"},
		}, nil)

	s.mockClient.On("UpdateAssetWithResponse", matchContext, projectID, assetID,
		mock.MatchedBy(func(body api.UpdateAssetInput) bool {
			return body.Name != nil && *body.Name == "renamed-asset"
		})).Return(
		&api.UpdateAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 1, Name: "renamed-asset"},
		}, nil)

	updateAsset(nil, nil)
}

func (s *CommandsSuite) TestReviseAsset() {
	projectID := uuid.New()
	assetID := uuid.New()

	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())
	viper.Set(assetLocationsKey, "s3://bucket/new-path")
	viper.Set(assetVersionKey, "v2.0")

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 1, Name: "my-asset"},
		}, nil)

	s.mockClient.On("ReviseAssetWithResponse", matchContext, projectID, assetID,
		mock.MatchedBy(func(body api.ReviseAssetInput) bool {
			return len(body.Locations) == 1 &&
				body.Locations[0] == "s3://bucket/new-path" &&
				body.Version == "v2.0"
		})).Return(
		&api.ReviseAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201: &api.Asset{
				AssetID:       assetID,
				AssetRevision: 2,
				Name:          "my-asset",
				Version:       "v2.0",
			},
		}, nil)

	reviseAsset(nil, nil)
}

func (s *CommandsSuite) TestArchiveAsset() {
	projectID := uuid.New()
	assetID := uuid.New()

	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 1, Name: "my-asset"},
		}, nil)

	s.mockClient.On("ArchiveAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.ArchiveAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil)

	archiveAsset(nil, nil)
}

func (s *CommandsSuite) TestRestoreAsset() {
	projectID := uuid.New()
	assetID := uuid.New()

	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 1, Name: "archived-asset", Archived: true},
		}, nil)

	s.mockClient.On("RestoreAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.RestoreAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil)

	restoreAsset(nil, nil)
}

func (s *CommandsSuite) TestBuildsForAsset() {
	projectID := uuid.New()
	assetID := uuid.New()
	buildID := uuid.New()

	viper.Reset()
	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 1, Name: "my-asset"},
		}, nil)

	s.mockClient.On("ListBuildsForAssetWithResponse", matchContext, projectID, assetID,
		mock.AnythingOfType("*api.ListBuildsForAssetParams")).Return(
		&api.ListBuildsForAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetBuildsOutput{
				Builds: []api.AssetBuildReference{
					{BuildID: &buildID, AssetRevision: Ptr(int64(1))},
				},
				NextPageToken: "",
			},
		}, nil)

	buildsForAsset(nil, nil)
}

func (s *CommandsSuite) TestBuildsForAssetRevision() {
	projectID := uuid.New()
	assetID := uuid.New()
	buildID := uuid.New()
	rev := int64(2)

	viper.Set(assetProjectKey, projectID.String())
	viper.Set(assetKey, assetID.String())
	viper.Set(assetRevisionKey, rev)

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 5, Name: "my-asset"},
		}, nil)

	s.mockClient.On("GetAssetRevisionWithResponse", matchContext, projectID, assetID, rev).Return(
		&api.GetAssetRevisionResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 2, Name: "my-asset"},
		}, nil)

	s.mockClient.On("ListBuildsForAssetRevisionWithResponse", matchContext, projectID, assetID, rev,
		mock.AnythingOfType("*api.ListBuildsForAssetRevisionParams")).Return(
		&api.ListBuildsForAssetRevisionResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetBuildsOutput{
				Builds: []api.AssetBuildReference{
					{BuildID: &buildID, AssetRevision: Ptr(int64(2))},
				},
				NextPageToken: "",
			},
		}, nil)

	buildsForAsset(nil, nil)
}

func (s *CommandsSuite) TestResolveAssetReferences_NameOnly() {
	projectID := uuid.New()
	assetID := uuid.New()

	s.mockClient.On("ListAssetsWithResponse", matchContext, projectID,
		mock.AnythingOfType("*api.ListAssetsParams")).Return(
		&api.ListAssetsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetsOutput{
				Assets: []api.Asset{
					{AssetID: assetID, AssetRevision: 5, Name: "my-asset"},
				},
				NextPageToken: "",
			},
		}, nil)

	links := resolveAssetReferences(Client, projectID, "my-asset")
	s.Len(links, 1)
	s.Equal(assetID, links[0].AssetID)
	s.Equal(int64(5), links[0].AssetRevision)
}

func (s *CommandsSuite) TestResolveAssetReferences_NameWithRevision() {
	projectID := uuid.New()
	assetID := uuid.New()

	s.mockClient.On("ListAssetsWithResponse", matchContext, projectID,
		mock.AnythingOfType("*api.ListAssetsParams")).Return(
		&api.ListAssetsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetsOutput{
				Assets: []api.Asset{
					{AssetID: assetID, AssetRevision: 5, Name: "my-asset"},
				},
				NextPageToken: "",
			},
		}, nil)

	links := resolveAssetReferences(Client, projectID, "my-asset:3")
	s.Len(links, 1)
	s.Equal(assetID, links[0].AssetID)
	s.Equal(int64(3), links[0].AssetRevision)
}

func (s *CommandsSuite) TestResolveAssetReferences_UUIDOnly() {
	projectID := uuid.New()
	assetID := uuid.New()

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 7, Name: "some-asset"},
		}, nil)

	links := resolveAssetReferences(Client, projectID, assetID.String())
	s.Len(links, 1)
	s.Equal(assetID, links[0].AssetID)
	s.Equal(int64(7), links[0].AssetRevision)
}

func (s *CommandsSuite) TestResolveAssetReferences_UUIDWithRevision() {
	projectID := uuid.New()
	assetID := uuid.New()

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 7, Name: "some-asset"},
		}, nil)

	links := resolveAssetReferences(Client, projectID, assetID.String()+":2")
	s.Len(links, 1)
	s.Equal(assetID, links[0].AssetID)
	s.Equal(int64(2), links[0].AssetRevision)
}

func (s *CommandsSuite) TestResolveAssetReferences_MixedList() {
	projectID := uuid.New()
	assetID1 := uuid.New()
	assetID2 := uuid.New()

	s.mockClient.On("ListAssetsWithResponse", matchContext, projectID,
		mock.AnythingOfType("*api.ListAssetsParams")).Return(
		&api.ListAssetsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAssetsOutput{
				Assets: []api.Asset{
					{AssetID: assetID1, AssetRevision: 3, Name: "named-asset"},
				},
				NextPageToken: "",
			},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID2).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID2, AssetRevision: 10, Name: "uuid-asset"},
		}, nil)

	refs := "named-asset," + assetID2.String() + ":4"
	links := resolveAssetReferences(Client, projectID, refs)
	s.Len(links, 2)
	s.Equal(assetID1, links[0].AssetID)
	s.Equal(int64(3), links[0].AssetRevision)
	s.Equal(assetID2, links[1].AssetID)
	s.Equal(int64(4), links[1].AssetRevision)
}

func (s *CommandsSuite) TestParseAssetRef() {
	id, rev := parseAssetRef("my-asset")
	s.Equal("my-asset", id)
	s.Nil(rev)

	id, rev = parseAssetRef("my-asset:3")
	s.Equal("my-asset", id)
	s.NotNil(rev)
	s.Equal(int64(3), *rev)

	testUUID := uuid.New().String()
	id, rev = parseAssetRef(testUUID)
	s.Equal(testUUID, id)
	s.Nil(rev)

	id, rev = parseAssetRef(testUUID + ":42")
	s.Equal(testUUID, id)
	s.NotNil(rev)
	s.Equal(int64(42), *rev)

	// Colon followed by non-number: treat whole string as identifier
	id, rev = parseAssetRef("weird:name:here")
	s.Equal("weird:name:here", id)
	s.Nil(rev)

	// Colon followed by a number at the end
	id, rev = parseAssetRef("weird:name:5")
	s.Equal("weird:name", id)
	s.NotNil(rev)
	s.Equal(int64(5), *rev)
}

func (s *CommandsSuite) TestAddAssetsToBuild() {
	projectID := uuid.New()
	buildID := uuid.New()
	assetID := uuid.New()

	viper.Set(buildProjectKey, projectID.String())
	viper.Set(buildBuildIDKey, buildID.String())
	viper.Set(buildAssetsKey, assetID.String()+":1")

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetBuildWithResponse", matchContext, projectID, buildID).Return(
		&api.GetBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Build{BuildID: buildID},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 3, Name: "my-asset"},
		}, nil)

	s.mockClient.On("AddAssetsToBuildWithResponse", matchContext, projectID, buildID,
		mock.MatchedBy(func(body api.AddAssetsToBuildJSONRequestBody) bool {
			return len(body.Assets) == 1 &&
				body.Assets[0].AssetID == assetID &&
				body.Assets[0].AssetRevision == int64(1)
		})).Return(
		&api.AddAssetsToBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil)

	addAssetsToBuild(nil, nil)
}

func (s *CommandsSuite) TestRemoveAssetsFromBuild() {
	projectID := uuid.New()
	buildID := uuid.New()
	assetID := uuid.New()

	viper.Set(buildProjectKey, projectID.String())
	viper.Set(buildBuildIDKey, buildID.String())
	viper.Set(buildAssetsKey, assetID.String())

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetBuildWithResponse", matchContext, projectID, buildID).Return(
		&api.GetBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Build{BuildID: buildID},
		}, nil)

	s.mockClient.On("GetAssetWithResponse", matchContext, projectID, assetID).Return(
		&api.GetAssetResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Asset{AssetID: assetID, AssetRevision: 5, Name: "my-asset"},
		}, nil)

	s.mockClient.On("RemoveAssetsFromBuildWithResponse", matchContext, projectID, buildID,
		mock.MatchedBy(func(body api.RemoveAssetsFromBuildJSONRequestBody) bool {
			return len(body.Assets) == 1 &&
				body.Assets[0].AssetID == assetID &&
				body.Assets[0].AssetRevision == int64(5)
		})).Return(
		&api.RemoveAssetsFromBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil)

	removeAssetsFromBuild(nil, nil)
}

func (s *CommandsSuite) TestListAssetsForBuild() {
	projectID := uuid.New()
	buildID := uuid.New()
	assetID := uuid.New()

	viper.Set(buildProjectKey, projectID.String())
	viper.Set(buildBuildIDKey, buildID.String())

	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)

	s.mockClient.On("GetBuildWithResponse", matchContext, projectID, buildID).Return(
		&api.GetBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Build{BuildID: buildID},
		}, nil)

	buildAssets := api.ListBuildAssetsOutput{
		{
			Asset:   api.Asset{AssetID: assetID, AssetRevision: 1, Name: "linked-asset"},
			BuildID: buildID,
		},
	}
	s.mockClient.On("ListAssetsForBuildWithResponse", matchContext, projectID, buildID).Return(
		&api.ListAssetsForBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &buildAssets,
		}, nil)

	listAssetsForBuild(nil, nil)
}
