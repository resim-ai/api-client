package commands

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func TestCommandsSuite(t *testing.T) {
	suite.Run(t, new(CommandsSuite))
}

type CommandsSuite struct {
	suite.Suite
	mockClient *mockapiclient.ClientWithResponsesInterface
}

func (s *CommandsSuite) SetupTest() {
	// Note that since the api client is referred to as the global variable `Client`,
	// these tests cannot be run in parallel.
	s.mockClient = mockapiclient.NewClientWithResponsesInterface(s.T())
	Client = s.mockClient
}

func (s *CommandsSuite) TearDownTest() {
	s.mockClient.AssertExpectations(s.T())
}

func (s *CommandsSuite) TestActualGetBatchByID() {
	projectID := uuid.New()
	batchID := uuid.New()
	friendlyName := "resim-black-sparrow"

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID:      &batchID,
				FriendlyName: &friendlyName,
			},
		}, nil)

	batch := actualGetBatch(projectID, batchID.String(), "")
	s.Equal(batchID, *batch.BatchID)
	s.Equal(friendlyName, *batch.FriendlyName)
}

func (s *CommandsSuite) TestActualGetBatchByName() {
	projectID := uuid.New()
	batchID := uuid.New()
	friendlyName := "resim-black-sparrow"

	// Client calls the List call with no search terms, and finds a match in the results.
	s.mockClient.On("ListBatchesWithResponse", matchContext, projectID, mock.AnythingOfType("*api.ListBatchesParams")).Return(
		&api.ListBatchesResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListBatchesOutput{
				Batches: &[]api.Batch{
					{
						BatchID:      Ptr(uuid.New()),
						FriendlyName: Ptr("some-other-batch"),
					},
					{
						BatchID:      &batchID,
						FriendlyName: &friendlyName,
					},
					{
						BatchID:      Ptr(uuid.New()),
						FriendlyName: Ptr("yet-another-batch"),
					},
				},
				NextPageToken: nil,
			},
		}, nil)

	batch := actualGetBatch(projectID, "", friendlyName)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(friendlyName, *batch.FriendlyName)
}
