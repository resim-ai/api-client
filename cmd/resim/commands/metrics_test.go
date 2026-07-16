package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testSpinner() *Spinner {
	return NewSpinner(&cobra.Command{})
}

func TestSyncMetricsCmdHasMetricsConfigPathFlag(t *testing.T) {
	flag := syncMetricsCmd.Flags().Lookup("metrics-config-path")
	assert.NotNil(t, flag, "--metrics-config-path flag should exist on syncMetricsCmd")
}

func TestSyncMetricsCmdHasDeprecatedConfigPathFlag(t *testing.T) {
	flag := syncMetricsCmd.Flags().Lookup("config-path")
	assert.NotNil(t, flag, "--config-path flag should exist on syncMetricsCmd")
	assert.True(t, flag.Deprecated != "", "--config-path flag should be marked deprecated")
}

func TestDebugMetricsCmdHasRequiredFlags(t *testing.T) {
	flags := []string{
		"project",
		"emissions-file",
		"metrics-config-path",
		"templates-path",
		"timeout",
		"poll-interval",
		"branch",
		"metrics-set",
		"media-file",
	}
	for _, name := range flags {
		flag := debugMetricsCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on debugMetricsCmd", name)
	}
}

func TestConfigSchemaCmdIsRegistered(t *testing.T) {
	var found bool
	for _, sub := range metricsCmd.Commands() {
		if sub.Name() == "config-schema" {
			found = true
			break
		}
	}
	assert.True(t, found, "config-schema subcommand should be registered under metricsCmd")
}

func TestValidateMetricsCmdHasRequiredFlags(t *testing.T) {
	flags := []string{"project", "branch", "metrics-config-path", "templates-path"}
	for _, name := range flags {
		flag := validateMetricsCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on validateMetricsCmd", name)
	}
}

// mockGraphQLClient implements graphql.Client for testing.
type mockGraphQLClient struct {
	mock.Mock
}

func (m *mockGraphQLClient) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	args := m.Called(ctx, req, resp)

	// If a response was configured via RunFn, it will have already populated resp.Data.
	// Otherwise, check if a response payload was provided as argument index 1.
	return args.Error(0)
}

// withGetDashboardResponse is a helper that configures a mock call to return a
// specific GetDashboard response by populating the resp.Data pointer.
func withGetDashboardResponse(lastRanAt string) func(args mock.Arguments) {
	return withGetDashboardResponseFull(lastRanAt, false)
}

func withGetDashboardResponseFull(lastRanAt string, isStale bool) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.GetDashboardResponse)
		data.Dashboard = bff.GetDashboardDashboard{
			Id:        "test-dashboard-id",
			Name:      "test-dashboard",
			IsStale:   isStale,
			LastRanAt: lastRanAt,
		}
	}
}

func isGetDashboardRequest(req *graphql.Request) bool {
	return req.OpName == "GetDashboard"
}

func TestWaitForDashboardReady_ImmediateSuccess(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetDashboardRequest), mock.Anything).
		Run(withGetDashboardResponse("2026-02-20T12:00:00Z")).
		Return(nil).Once()

	err := waitForDashboardReady(context.Background(), mockClient, "test-dashboard-id", 5*time.Second, 50*time.Millisecond, testSpinner())
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestWaitForDashboardReady_TransitionFromStale(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	// First call: not ready (lastRanAt empty)
	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetDashboardRequest), mock.Anything).
		Run(withGetDashboardResponse("")).
		Return(nil).Once()

	// Second call: ready
	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetDashboardRequest), mock.Anything).
		Run(withGetDashboardResponse("2026-02-20T12:00:00Z")).
		Return(nil).Once()

	err := waitForDashboardReady(context.Background(), mockClient, "test-dashboard-id", 5*time.Second, 50*time.Millisecond, testSpinner())
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestWaitForDashboardReady_Timeout(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	// Always return not ready
	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetDashboardRequest), mock.Anything).
		Run(withGetDashboardResponse("")).
		Return(nil).Maybe()

	err := waitForDashboardReady(context.Background(), mockClient, "test-dashboard-id", 100*time.Millisecond, 30*time.Millisecond, testSpinner())
	assert.Error(t, err)

	timeoutErr, ok := err.(*TimeoutError)
	assert.True(t, ok, "expected TimeoutError, got %T", err)
	assert.True(t, timeoutErr.IsTimeout())
}

func TestWaitForDashboardReady_APIError(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetDashboardRequest), mock.Anything).
		Return(fmt.Errorf("connection refused")).Once()

	err := waitForDashboardReady(context.Background(), mockClient, "test-dashboard-id", 5*time.Second, 50*time.Millisecond, testSpinner())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
	mockClient.AssertExpectations(t)
}

func isValidateMetricsConfigRequest(req *graphql.Request) bool {
	return req.OpName == "ValidateMetricsConfig"
}

func TestValidateMetricsConfig_Success(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isValidateMetricsConfigRequest), mock.Anything).
		Run(func(args mock.Arguments) {
			resp := args.Get(2).(*graphql.Response)
			data := resp.Data.(*bff.ValidateMetricsConfigResponse)
			data.ValidateMetricsConfig = true
		}).
		Return(nil).Once()

	resp, err := bff.ValidateMetricsConfig(
		context.Background(), mockClient, "branch-id", "config", []bff.MetricsTemplate{},
	)
	assert.NoError(t, err)
	assert.True(t, resp.ValidateMetricsConfig)
	mockClient.AssertExpectations(t)
}

func TestValidateMetricsConfig_SurfacesValidationError(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isValidateMetricsConfigRequest), mock.Anything).
		Return(fmt.Errorf("topic 'foo' is invalid")).Once()

	_, err := bff.ValidateMetricsConfig(
		context.Background(), mockClient, "branch-id", "config", []bff.MetricsTemplate{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "topic 'foo' is invalid")
	mockClient.AssertExpectations(t)
}

func TestInferAppURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "production API",
			input:    "https://api.resim.ai/v1/",
			expected: "https://app.resim.ai",
		},
		{
			name:     "dev API",
			input:    "https://api.dev.resim.io/v1/",
			expected: "https://app.dev.resim.io",
		},
		{
			name:     "localhost",
			input:    "http://localhost:4000/v1/",
			expected: "http://localhost:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferAppURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Ensure the mock satisfies the generated code's usage pattern by verifying
// that response data can be JSON-unmarshalled as the generated client does.
func TestMockGraphQLClient_SatisfiesInterface(t *testing.T) {
	var _ graphql.Client = (*mockGraphQLClient)(nil)

	// Verify the mock can handle CreateDebugDashboard-shaped responses
	mockClient := new(mockGraphQLClient)
	mockClient.On("MakeRequest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			resp := args.Get(2).(*graphql.Response)
			raw := `{"createDebugDashboard":{"id":"abc-123","name":"test"}}`
			json.Unmarshal([]byte(raw), resp.Data)
		}).
		Return(nil).Once()

	result, err := bff.CreateDebugDashboard(
		context.Background(), mockClient,
		"project-id", "config", []bff.MetricsTemplate{}, "emissions", "", "", []bff.MediaFileInput{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "abc-123", result.CreateDebugDashboard.Id)
	mockClient.AssertExpectations(t)
}

func isPreviewTopicArchivalsRequest(req *graphql.Request) bool {
	return req.OpName == "PreviewTopicArchivals"
}

func isUpdateMetricsConfigRequest(req *graphql.Request) bool {
	return req.OpName == "UpdateMetricsConfig"
}

func withPreviewTopicArchivalsResponse(previews []bff.PreviewTopicArchivalsPreviewTopicArchivalsTopicArchivalPreview) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.PreviewTopicArchivalsResponse)
		data.PreviewTopicArchivals = previews
	}
}

func withUpdateMetricsConfigSuccess() func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.UpdateMetricsConfigResponse)
		data.UpdateMetricsConfig = "Success"
	}
}

// withMockClient stubs the package-level REST Client with a mock that returns a
// named branch for any GetBranchForProjectWithResponse call — SyncMetricsConfig's
// first call, needed before it ever touches BffClient.
func withMockClient(t *testing.T, branchName string) *mockapiclient.ClientWithResponsesInterface {
	t.Helper()
	mockClient := mockapiclient.NewClientWithResponsesInterface(t)
	mockClient.On("GetBranchForProjectWithResponse", mock.Anything, mock.Anything, mock.Anything).
		Return(&api.GetBranchForProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Branch{Name: branchName},
		}, nil)

	origClient := Client
	Client = mockClient
	t.Cleanup(func() { Client = origClient })
	return mockClient
}

func TestSyncMetricsConfig_NoTopicsArchived_SucceedsWithoutFlag(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Run(withPreviewTopicArchivalsResponse(nil)).
		Return(nil).Once()
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Run(withUpdateMetricsConfigSuccess()).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", false, false)
	assert.NoError(t, err)
	mockBff.AssertExpectations(t)
}

func TestSyncMetricsConfig_TopicsWouldBeArchived_RejectsWithoutFlag(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	previews := []bff.PreviewTopicArchivalsPreviewTopicArchivalsTopicArchivalPreview{
		{TopicName: "old_topic", RowsToBeHidden: 42, ChartCount: 2},
	}
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Run(withPreviewTopicArchivalsResponse(previews)).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "old_topic")
	assert.Contains(t, err.Error(), "--allow-topic-archival")
	// UpdateMetricsConfig must never be called when the archival preview is rejected.
	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything)
}

func TestSyncMetricsConfig_TopicsWouldBeArchived_ProceedsWithFlag(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	// With --allow-topic-archival already set, the user has confirmed the archival, so
	// the preview is intentionally skipped (see previewTopicArchivalImpact's caller in
	// SyncMetricsConfig) — only UpdateMetricsConfig should be invoked.
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Run(withUpdateMetricsConfigSuccess()).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", true, false)
	assert.NoError(t, err)
	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything)
	mockBff.AssertExpectations(t)
}

func TestSyncMetricsConfig_PreviewTransportErrorSoftFails(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	// A non-GraphQL (transport) error on the preview must not block sync — sync proceeds
	// and the BFF's own gate is the backstop, mirroring validateMetricsSetExists's soft-fail.
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Return(fmt.Errorf("bff unavailable")).Once()
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Run(withUpdateMetricsConfigSuccess()).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", false, false)
	assert.NoError(t, err)
	mockBff.AssertExpectations(t)
}

func TestSyncMetricsCmdHasAllowTopicArchivalFlag(t *testing.T) {
	flag := syncMetricsCmd.Flags().Lookup("allow-topic-archival")
	assert.NotNil(t, flag, "--allow-topic-archival flag should exist on syncMetricsCmd")
	assert.Equal(t, "false", flag.DefValue)
}
