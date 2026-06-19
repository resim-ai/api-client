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
