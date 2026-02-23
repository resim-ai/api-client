package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
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
	}
	for _, name := range flags {
		flag := debugMetricsCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on debugMetricsCmd", name)
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

func TestWaitForDashboardReady_IsStale(t *testing.T) {
	mockClient := new(mockGraphQLClient)

	mockClient.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetDashboardRequest), mock.Anything).
		Run(withGetDashboardResponseFull("", true)).
		Return(nil).Once()

	err := waitForDashboardReady(context.Background(), mockClient, "test-dashboard-id", 5*time.Second, 50*time.Millisecond, testSpinner())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process")
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
		"project-id", "config", []bff.MetricsTemplate{}, "emissions", "", "",
	)
	assert.NoError(t, err)
	assert.Equal(t, "abc-123", result.CreateDebugDashboard.Id)
	mockClient.AssertExpectations(t)
}
