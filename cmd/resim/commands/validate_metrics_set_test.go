package commands

import (
	"errors"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/bff"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func isGetBranchMetricsSetsRequest(req *graphql.Request) bool {
	return req.OpName == "GetBranchMetricsSets"
}

// withBranchMetricsSets configures a mock GetBranchMetricsSets call to return the given set names.
func withBranchMetricsSets(names ...string) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.GetBranchMetricsSetsResponse)
		sets := make([]bff.GetBranchMetricsSetsBranchConfigVersionMetricsSetsBranchMetricsSet, 0, len(names))
		for _, name := range names {
			sets = append(sets, bff.GetBranchMetricsSetsBranchConfigVersionMetricsSetsBranchMetricsSet{Name: name})
		}
		data.BranchConfigVersion.MetricsSets = sets
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
	assert.NoError(t, validateMetricsSetExists(uuid.New(), uuid.New(), nil))
	assert.NoError(t, validateMetricsSetExists(uuid.New(), uuid.New(), Ptr("")))

	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.Anything, mock.Anything)
}

func TestValidateMetricsSetExists_ValidName(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetBranchMetricsSetsRequest), mock.Anything).
		Run(withBranchMetricsSets("speed", "safety")).
		Return(nil).Once()
	withMockBffClient(t, mockBff)

	assert.NoError(t, validateMetricsSetExists(uuid.New(), uuid.New(), Ptr("safety")))
	mockBff.AssertExpectations(t)
}

func TestValidateMetricsSetExists_UnknownNameListsAvailable(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetBranchMetricsSetsRequest), mock.Anything).
		Run(withBranchMetricsSets("speed", "safety")).
		Return(nil).Once()
	withMockBffClient(t, mockBff)

	err := validateMetricsSetExists(uuid.New(), uuid.New(), Ptr("typo"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `"typo"`)
	assert.Contains(t, err.Error(), "speed")
	assert.Contains(t, err.Error(), "safety")
	mockBff.AssertExpectations(t)
}

func TestValidateMetricsSetExists_NoSetsDefined(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetBranchMetricsSetsRequest), mock.Anything).
		Run(withBranchMetricsSets()).
		Return(nil).Once()
	withMockBffClient(t, mockBff)

	err := validateMetricsSetExists(uuid.New(), uuid.New(), Ptr("anything"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no metrics sets defined")
	mockBff.AssertExpectations(t)
}

func TestValidateMetricsSetExists_LookupErrorSoftFails(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isGetBranchMetricsSetsRequest), mock.Anything).
		Return(errors.New("bff unavailable")).Once()
	withMockBffClient(t, mockBff)

	// A failed lookup must not block creation — the server stays the backstop.
	assert.NoError(t, validateMetricsSetExists(uuid.New(), uuid.New(), Ptr("safety")))
	mockBff.AssertExpectations(t)
}
