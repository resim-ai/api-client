package commands

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	"github.com/resim-ai/api-client/bff"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func isValidateMetricsSetRequest(req *graphql.Request) bool {
	return req.OpName == "ValidateMetricsSet"
}

// withValidateMetricsSetResult configures a mock ValidateMetricsSet call to return the given verdict.
func withValidateMetricsSetResult(valid bool) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.ValidateMetricsSetResponse)
		data.ValidateMetricsSet = valid
	}
}

func withMockBffClient(t *testing.T, mockBff *mockGraphQLClient) {
	t.Helper()
	origBffClient := BffClient
	BffClient = mockBff
	t.Cleanup(func() { BffClient = origBffClient })
}

func TestValidateMetricsSetExists_EmptyNameSkipsLookup(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	// nil and empty-string names are both treated as "unset" and must not call the BFF.
	assert.NoError(t, validateMetricsSetExists(uuid.New(), nil))
	assert.NoError(t, validateMetricsSetExists(uuid.New(), Ptr("")))

	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.Anything, mock.Anything)
}

func TestValidateMetricsSetExists_ValidName(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isValidateMetricsSetRequest), mock.Anything).
		Run(withValidateMetricsSetResult(true)).
		Return(nil).Once()
	withMockBffClient(t, mockBff)

	assert.NoError(t, validateMetricsSetExists(uuid.New(), Ptr("safety")))
	mockBff.AssertExpectations(t)
}

func TestValidateMetricsSetExists_UnknownNameSurfacesBffError(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	message := "Metrics set 'typo' was not found in the latest metrics config (available sets: speed, safety)."
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isValidateMetricsSetRequest), mock.Anything).
		Return(gqlerror.List{{Message: message}}).Once()
	withMockBffClient(t, mockBff)

	// A GraphQL error from the BFF means the set was rejected; surface its message verbatim.
	err := validateMetricsSetExists(uuid.New(), Ptr("typo"))
	assert.EqualError(t, err, message)
	mockBff.AssertExpectations(t)
}

func TestValidateMetricsSetExists_TransportErrorSoftFails(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isValidateMetricsSetRequest), mock.Anything).
		Return(errors.New("bff unavailable")).Once()
	withMockBffClient(t, mockBff)

	// A non-GraphQL (transport) error must not block creation — the server stays the backstop.
	assert.NoError(t, validateMetricsSetExists(uuid.New(), Ptr("safety")))
	mockBff.AssertExpectations(t)
}

// withMockClientForBuild stubs the package-level REST Client with a mock that resolves
// buildID to branchID, and branchID to branchName — the two REST lookups
// syncAndValidateMetricsSet makes before it ever reaches the BFF.
func withMockClientForBuild(t *testing.T, projectID, buildID, branchID uuid.UUID, branchName string) *mockapiclient.ClientWithResponsesInterface {
	t.Helper()
	mockClient := mockapiclient.NewClientWithResponsesInterface(t)
	mockClient.On("GetBuildWithResponse", mock.Anything, projectID, buildID).
		Return(&api.GetBuildResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Build{BranchID: branchID},
		}, nil).Maybe()
	mockClient.On("GetBranchForProjectWithResponse", mock.Anything, projectID, branchID).
		Return(&api.GetBranchForProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Branch{Name: branchName},
		}, nil).Maybe()

	origClient := Client
	Client = mockClient
	t.Cleanup(func() { Client = origClient })
	return mockClient
}

// recordingBffClient answers every BFF operation involved in a sync-then-validate run
// and records the operation names in the order they were issued.
func recordingBffClient(t *testing.T, ops *[]string) *mockGraphQLClient {
	t.Helper()
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*graphql.Request)
			*ops = append(*ops, req.OpName)

			switch data := args.Get(2).(*graphql.Response).Data.(type) {
			case *bff.PreviewTopicRemovalResponse:
				data.PreviewTopicRemoval = nil
			case *bff.UpdateMetricsConfigResponse:
				data.UpdateMetricsConfig = "Success"
			case *bff.FindUnusedMetricsResponse:
				data.FindUnusedMetrics = nil
			case *bff.ValidateMetricsSetResponse:
				data.ValidateMetricsSet = true
			}
		}).
		Return(nil)
	withMockBffClient(t, mockBff)
	return mockBff
}

func indexOfOp(ops []string, name string) int {
	for i, op := range ops {
		if op == name {
			return i
		}
	}
	return -1
}

// The regression guard for WOB-4358: when both flags are supplied, the config must be
// synced before the metrics set is validated. Validating first makes --sync-metrics-config
// and --metrics-set mutually unusable on a branch that has no metrics config yet, because
// the set being validated is defined in the very config the sync is about to upload.
func TestSyncAndValidateMetricsSet_SyncsBeforeValidating(t *testing.T) {
	projectID, buildID, branchID := uuid.New(), uuid.New(), uuid.New()
	withMockClientForBuild(t, projectID, buildID, branchID, "main")

	var ops []string
	recordingBffClient(t, &ops)

	err := syncAndValidateMetricsSet(projectID, buildID, Ptr("woot"), metricsConfigSync{
		Enabled:       true,
		ConfigPaths:   []string{"testdata/config.yml"},
		TemplatesPath: "testdata/templates",
	})
	assert.NoError(t, err)

	syncedAt := indexOfOp(ops, "UpdateMetricsConfig")
	validatedAt := indexOfOp(ops, "ValidateMetricsSet")
	assert.NotEqual(t, -1, syncedAt, "expected the metrics config to be synced, got ops %v", ops)
	assert.NotEqual(t, -1, validatedAt, "expected the metrics set to be validated, got ops %v", ops)
	assert.Less(t, syncedAt, validatedAt,
		"the metrics config must be synced before the metrics set is validated, got ops %v", ops)
}

// A sync failure must stop the run before the set is validated: validating against a
// half-synced branch would report a confusing "set not found" instead of the sync error.
func TestSyncAndValidateMetricsSet_SyncFailureSkipsValidation(t *testing.T) {
	projectID, buildID, branchID := uuid.New(), uuid.New(), uuid.New()
	withMockClientForBuild(t, projectID, buildID, branchID, "main")

	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicRemovalRequest), mock.Anything).
		Run(withPreviewTopicRemovalResponse(nil)).
		Return(nil).Once()
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Return(gqlerror.List{{Message: "config is invalid"}}).Once()
	withMockBffClient(t, mockBff)

	err := syncAndValidateMetricsSet(projectID, buildID, Ptr("woot"), metricsConfigSync{
		Enabled:       true,
		ConfigPaths:   []string{"testdata/config.yml"},
		TemplatesPath: "testdata/templates",
	})
	assert.ErrorContains(t, err, "config is invalid")
	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.MatchedBy(isValidateMetricsSetRequest), mock.Anything)
}

// Without --sync-metrics-config the precheck still runs on its own, unchanged.
func TestSyncAndValidateMetricsSet_ValidatesWithoutSync(t *testing.T) {
	projectID, buildID, branchID := uuid.New(), uuid.New(), uuid.New()
	withMockClientForBuild(t, projectID, buildID, branchID, "main")

	var ops []string
	recordingBffClient(t, &ops)

	err := syncAndValidateMetricsSet(projectID, buildID, Ptr("woot"), metricsConfigSync{Enabled: false})
	assert.NoError(t, err)
	assert.Equal(t, []string{"ValidateMetricsSet"}, ops)
}

// With neither a metrics set nor a sync requested there is nothing to do, and in
// particular no build lookup to pay for.
func TestSyncAndValidateMetricsSet_NoWorkSkipsBuildLookup(t *testing.T) {
	projectID, buildID, branchID := uuid.New(), uuid.New(), uuid.New()
	mockClient := withMockClientForBuild(t, projectID, buildID, branchID, "main")

	var ops []string
	recordingBffClient(t, &ops)

	assert.NoError(t, syncAndValidateMetricsSet(projectID, buildID, nil, metricsConfigSync{Enabled: false}))
	assert.Empty(t, ops)
	mockClient.AssertNotCalled(t, "GetBuildWithResponse", mock.Anything, mock.Anything, mock.Anything)
}
