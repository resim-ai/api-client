package commands

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMcapCreateParserCmdRequiredFlags(t *testing.T) {
	for _, name := range []string{mcapProjectKey, mcapParserDescriptionKey, mcapParserImageURIKey} {
		flag := mcapCreateParserCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on create-parser", name)
		assert.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should be required", name)
	}
	flag := mcapCreateParserCmd.Flags().Lookup(mcapParserNameKey)
	assert.NotNil(t, flag, "--%s flag should exist on create-parser", mcapParserNameKey)
	assert.Nil(t, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should be optional", mcapParserNameKey)
}

func TestMcapListParsersCmdRequiredFlags(t *testing.T) {
	flag := mcapListParsersCmd.Flags().Lookup(mcapProjectKey)
	assert.NotNil(t, flag)
	assert.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag])
}

func TestMcapIngestCmdRequiredFlags(t *testing.T) {
	required := []string{
		mcapProjectKey,
		mcapIngestSessionNameKey,
		mcapIngestSessionDescriptionKey,
		mcapIngestLocationKey,
		mcapIngestParserIDKey,
	}
	for _, name := range required {
		flag := mcapIngestCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on ingest", name)
		assert.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should be required", name)
	}
}

func (s *CommandsSuite) mockMcapProjectByID(projectID uuid.UUID) {
	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Project{ProjectID: projectID, Name: "test-project"},
		}, nil)
}

func (s *CommandsSuite) mockMcapListSystemsEmpty(projectID uuid.UUID) {
	s.mockClient.On("ListSystemsWithResponse", matchContext, projectID, mock.AnythingOfType("*api.ListSystemsParams")).Return(
		&api.ListSystemsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListSystemsOutput{Systems: &[]api.System{}, NextPageToken: nil},
		}, nil)
}

func (s *CommandsSuite) mockMcapListSystemsWithParser(projectID, systemID uuid.UUID) {
	s.mockClient.On("ListSystemsWithResponse", matchContext, projectID, mock.AnythingOfType("*api.ListSystemsParams")).Return(
		&api.ListSystemsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListSystemsOutput{
				Systems:       &[]api.System{{SystemID: systemID, Name: mcapParserSystemName}},
				NextPageToken: nil,
			},
		}, nil)
}

func (s *CommandsSuite) mockMcapListBranchesEmpty(projectID uuid.UUID) {
	s.mockClient.On("ListBranchesForProjectWithResponse", matchContext, projectID, mock.Anything).Return(
		&api.ListBranchesForProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListBranchesOutput{Branches: &[]api.Branch{}, NextPageToken: nil},
		}, nil)
}

func (s *CommandsSuite) mockMcapListBranchesWithMain(projectID, branchID uuid.UUID) {
	s.mockClient.On("ListBranchesForProjectWithResponse", matchContext, projectID, mock.Anything).Return(
		&api.ListBranchesForProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBranchesOutput{
				Branches:      &[]api.Branch{{BranchID: branchID, Name: mcapParserBranchName}},
				NextPageToken: nil,
			},
		}, nil)
}

func (s *CommandsSuite) TestMcapCreateParserBootstrapsSystem() {
	viper.Reset()
	projectID := uuid.New()
	systemID := uuid.New()
	metricsBuildID := uuid.New()
	branchID := uuid.New()
	buildID := uuid.New()

	viper.Set(mcapProjectKey, projectID.String())
	viper.Set(mcapParserDescriptionKey, "parser desc")
	viper.Set(mcapParserImageURIKey, "public.ecr.aws/resim/foo:1")

	s.mockMcapProjectByID(projectID)
	s.mockMcapListSystemsEmpty(projectID)
	s.mockClient.On("CreateSystemWithResponse", matchContext, projectID, mock.AnythingOfType("api.CreateSystemInput")).Return(
		&api.CreateSystemResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.System{SystemID: systemID, Name: mcapParserSystemName},
		}, nil)
	s.mockClient.On("CreateMetricsBuildWithResponse", matchContext, projectID, mock.AnythingOfType("api.CreateMetricsBuildInput")).Return(
		&api.CreateMetricsBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.MetricsBuild{MetricsBuildID: metricsBuildID},
		}, nil)
	s.mockClient.On("AddSystemToMetricsBuildWithResponse", matchContext, projectID, systemID, metricsBuildID).Return(
		&api.AddSystemToMetricsBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
		}, nil)
	s.mockMcapListBranchesEmpty(projectID)
	s.mockClient.On("CreateBranchForProjectWithResponse", matchContext, projectID, mock.AnythingOfType("api.CreateBranchInput")).Return(
		&api.CreateBranchForProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.Branch{BranchID: branchID, Name: mcapParserBranchName},
		}, nil)
	s.mockClient.On("CreateBuildForBranchWithResponse", matchContext, projectID, branchID, mock.AnythingOfType("api.CreateBuildForBranchInput")).Return(
		&api.CreateBuildForBranchResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.Build{BuildID: buildID},
		}, nil)

	mcapCreateParser(nil, nil)

	var sawBuildCall bool
	for _, call := range s.mockClient.Calls {
		if call.Method == "CreateBuildForBranchWithResponse" {
			sawBuildCall = true
			body := call.Arguments.Get(3).(api.CreateBuildForBranchInput)
			s.Equal(systemID, body.SystemID)
			s.Require().NotNil(body.ImageUri)
			s.Equal("public.ecr.aws/resim/foo:1", *body.ImageUri)
			s.Require().NotNil(body.Name)
			s.Equal("parser desc", *body.Name, "name should fall back to description when --name not set")
			s.Require().NotNil(body.Description)
			s.Equal("parser desc", *body.Description)
		}
	}
	s.True(sawBuildCall, "CreateBuildForBranchWithResponse should have been called")
}

func (s *CommandsSuite) TestMcapCreateParserReusesExistingSystem() {
	viper.Reset()
	projectID := uuid.New()
	systemID := uuid.New()
	branchID := uuid.New()
	buildID := uuid.New()

	viper.Set(mcapProjectKey, projectID.String())
	viper.Set(mcapParserNameKey, "my-parser")
	viper.Set(mcapParserDescriptionKey, "my desc")
	viper.Set(mcapParserImageURIKey, "public.ecr.aws/resim/foo:1")

	s.mockMcapProjectByID(projectID)
	s.mockMcapListSystemsWithParser(projectID, systemID)
	s.mockMcapListBranchesWithMain(projectID, branchID)
	s.mockClient.On("CreateBuildForBranchWithResponse", matchContext, projectID, branchID, mock.AnythingOfType("api.CreateBuildForBranchInput")).Return(
		&api.CreateBuildForBranchResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.Build{BuildID: buildID},
		}, nil)

	mcapCreateParser(nil, nil)

	for _, call := range s.mockClient.Calls {
		switch call.Method {
		case "CreateSystemWithResponse",
			"CreateMetricsBuildWithResponse",
			"AddSystemToMetricsBuildWithResponse",
			"CreateBranchForProjectWithResponse":
			s.Failf("unexpected call", "should not have called %s when system and branch already exist", call.Method)
		case "CreateBuildForBranchWithResponse":
			body := call.Arguments.Get(3).(api.CreateBuildForBranchInput)
			s.Equal(systemID, body.SystemID)
			s.Require().NotNil(body.Name)
			s.Equal("my-parser", *body.Name)
			s.Require().NotNil(body.Description)
			s.Equal("my desc", *body.Description)
		}
	}
}

func (s *CommandsSuite) TestMcapListParsers() {
	viper.Reset()
	projectID := uuid.New()
	systemID := uuid.New()
	buildID := uuid.New()

	viper.Set(mcapProjectKey, projectID.String())
	s.mockMcapProjectByID(projectID)
	s.mockMcapListSystemsWithParser(projectID, systemID)
	s.mockClient.On("ListBuildsForSystemWithResponse", matchContext, projectID, systemID, mock.AnythingOfType("*api.ListBuildsForSystemParams")).Return(
		&api.ListBuildsForSystemResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBuildsOutput{
				Builds:        []api.Build{{BuildID: buildID}},
				NextPageToken: "",
			},
		}, nil)

	mcapListParsers(nil, nil)
}

func (s *CommandsSuite) TestMcapIngestCreatesExperienceAndBatch() {
	viper.Reset()
	projectID := uuid.New()
	systemID := uuid.New()
	buildID := uuid.New()
	experienceID := uuid.New()
	batchID := uuid.New()

	viper.Set(mcapProjectKey, projectID.String())
	viper.Set(mcapIngestSessionNameKey, "session-1")
	viper.Set(mcapIngestSessionDescriptionKey, "session desc")
	viper.Set(mcapIngestLocationKey, "s3://bucket/path")
	viper.Set(mcapIngestParserIDKey, buildID.String())

	s.mockMcapProjectByID(projectID)
	s.mockMcapListSystemsWithParser(projectID, systemID)
	s.mockClient.On("ListExperiencesWithResponse", matchContext, projectID, mock.AnythingOfType("*api.ListExperiencesParams")).Return(
		&api.ListExperiencesResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListExperiencesOutput{Experiences: &[]api.Experience{}, NextPageToken: nil},
		}, nil)
	s.mockClient.On("CreateExperienceWithResponse", matchContext, projectID, mock.AnythingOfType("api.CreateExperienceInput")).Return(
		&api.CreateExperienceResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.Experience{ExperienceID: experienceID, Name: "session-1"},
		}, nil)

	batchStatus := api.BatchStatusSUBMITTED
	friendly := "session-1"
	s.mockClient.On("CreateBatchWithResponse", matchContext, projectID, mock.AnythingOfType("api.BatchInput")).Return(
		&api.CreateBatchResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.Batch{BatchID: &batchID, FriendlyName: &friendly, Status: &batchStatus},
		}, nil)

	mcapIngest(nil, nil)

	var sawExperience, sawBatch bool
	for _, call := range s.mockClient.Calls {
		switch call.Method {
		case "CreateExperienceWithResponse":
			sawExperience = true
			body := call.Arguments.Get(2).(api.CreateExperienceInput)
			s.Equal("session-1", body.Name)
			s.Equal("session desc", body.Description)
			s.Require().NotNil(body.Locations)
			s.Equal([]string{"s3://bucket/path"}, *body.Locations)
			s.Require().NotNil(body.SystemIDs)
			s.Equal([]api.SystemID{systemID}, *body.SystemIDs)
			s.Require().NotNil(body.ContainerTimeoutSeconds)
			s.Equal(mcapParserContainerTimeoutSecs, *body.ContainerTimeoutSeconds)
			s.Require().NotNil(body.CacheExempt)
			s.True(*body.CacheExempt)
		case "CreateBatchWithResponse":
			sawBatch = true
			body := call.Arguments.Get(2).(api.BatchInput)
			s.Require().NotNil(body.BuildID)
			s.Equal(buildID, *body.BuildID)
			s.Require().NotNil(body.ExperienceIDs)
			s.Equal([]api.ExperienceID{experienceID}, *body.ExperienceIDs)
			s.Require().NotNil(body.BatchName)
			s.Equal("session-1", *body.BatchName)
			s.Require().NotNil(body.Parameters)
			s.Equal("session-1", (*body.Parameters)[mcapBatchSessionNameParameter])
		}
	}
	s.True(sawExperience, "CreateExperienceWithResponse should have been called")
	s.True(sawBatch, "CreateBatchWithResponse should have been called")
}

func (s *CommandsSuite) TestMcapIngestReusesExistingExperience() {
	viper.Reset()
	projectID := uuid.New()
	systemID := uuid.New()
	buildID := uuid.New()
	experienceID := uuid.New()
	batchID := uuid.New()

	viper.Set(mcapProjectKey, projectID.String())
	viper.Set(mcapIngestSessionNameKey, "session-1")
	viper.Set(mcapIngestSessionDescriptionKey, "session desc")
	viper.Set(mcapIngestLocationKey, "s3://bucket/path")
	viper.Set(mcapIngestParserIDKey, buildID.String())

	s.mockMcapProjectByID(projectID)
	s.mockMcapListSystemsWithParser(projectID, systemID)
	s.mockClient.On("ListExperiencesWithResponse", matchContext, projectID, mock.AnythingOfType("*api.ListExperiencesParams")).Return(
		&api.ListExperiencesResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListExperiencesOutput{
				Experiences:   &[]api.Experience{{ExperienceID: experienceID, Name: "session-1"}},
				NextPageToken: nil,
			},
		}, nil)

	batchStatus := api.BatchStatusSUBMITTED
	friendly := "session-1"
	s.mockClient.On("CreateBatchWithResponse", matchContext, projectID, mock.AnythingOfType("api.BatchInput")).Return(
		&api.CreateBatchResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201:      &api.Batch{BatchID: &batchID, FriendlyName: &friendly, Status: &batchStatus},
		}, nil)

	mcapIngest(nil, nil)

	var sawBatch bool
	for _, call := range s.mockClient.Calls {
		if call.Method == "CreateExperienceWithResponse" {
			s.Fail("should not create experience when one already matches the session name")
		}
		if call.Method == "CreateBatchWithResponse" {
			sawBatch = true
			body := call.Arguments.Get(2).(api.BatchInput)
			s.Require().NotNil(body.ExperienceIDs)
			s.Equal([]api.ExperienceID{experienceID}, *body.ExperienceIDs)
			s.Require().NotNil(body.Parameters)
			s.Equal("session-1", (*body.Parameters)[mcapBatchSessionNameParameter])
		}
	}
	s.True(sawBatch, "CreateBatchWithResponse should have been called")
}
