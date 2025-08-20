package commands

import (
	"net/http"
	"testing"
	"time"

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

// waitForBatchCompletion tests
func (s *CommandsSuite) TestWaitForBatchCompletion_Success() {
	projectID := uuid.New()
	batchID := uuid.New()
	batchName := "test-batch"
	timeout := 10 * time.Second
	pollInterval := 100 * time.Millisecond

	// Mock the GetBatchWithResponse call to return a successful batch
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID:      &batchID,
				FriendlyName: &batchName,
				Status:       Ptr(api.BatchStatusSUCCEEDED),
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.NoError(err)
	s.NotNil(batch)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
}

func (s *CommandsSuite) TestWaitForBatchCompletion_ErrorStatus() {
	projectID := uuid.New()
	batchID := uuid.New()
	batchName := "test-batch"
	timeout := 10 * time.Second
	pollInterval := 100 * time.Millisecond

	// Mock the GetBatchWithResponse call to return a batch with ERROR status
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID:      &batchID,
				FriendlyName: &batchName,
				Status:       Ptr(api.BatchStatusERROR),
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.NoError(err)
	s.NotNil(batch)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(api.BatchStatusERROR, *batch.Status)
}

func (s *CommandsSuite) TestWaitForBatchCompletion_CancelledStatus() {
	projectID := uuid.New()
	batchID := uuid.New()
	batchName := "test-batch"
	timeout := 10 * time.Second
	pollInterval := 100 * time.Millisecond

	// Mock the GetBatchWithResponse call to return a batch with CANCELLED status
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID:      &batchID,
				FriendlyName: &batchName,
				Status:       Ptr(api.BatchStatusCANCELLED),
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.NoError(err)
	s.NotNil(batch)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(api.BatchStatusCANCELLED, *batch.Status)
}

func (s *CommandsSuite) TestWaitForBatchCompletion_NoStatusReturned() {
	projectID := uuid.New()
	batchID := uuid.New()
	timeout := 10 * time.Second
	pollInterval := 100 * time.Millisecond

	// Mock the GetBatchWithResponse call to return a batch with no status
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				// Status is nil
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.Error(err)
	s.Nil(batch)
	s.Contains(err.Error(), "no status returned")
}

func (s *CommandsSuite) TestWaitForBatchCompletion_UnknownStatus() {
	projectID := uuid.New()
	batchID := uuid.New()
	timeout := 10 * time.Second
	pollInterval := 100 * time.Millisecond

	// Mock the GetBatchWithResponse call to return a batch with unknown status
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatus("UNKNOWN_STATUS")),
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.Error(err)
	s.Nil(batch)
	s.Contains(err.Error(), "unknown batch status: UNKNOWN_STATUS")
}

func (s *CommandsSuite) TestWaitForBatchCompletion_Timeout() {
	projectID := uuid.New()
	batchID := uuid.New()
	timeout := 100 * time.Millisecond
	pollInterval := 50 * time.Millisecond

	// Mock the GetBatchWithResponse call to return a batch in running state
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING),
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.Error(err)
	s.NotNil(batch) // Should return the last batch state
	s.IsType(&TimeoutError{}, err)
	timeoutErr := err.(*TimeoutError)
	s.Contains(timeoutErr.message, "timeout after")
	s.Contains(timeoutErr.message, "EXPERIENCES_RUNNING")
}

func (s *CommandsSuite) TestWaitForBatchCompletion_StateTransition() {
	projectID := uuid.New()
	batchID := uuid.New()
	timeout := 1 * time.Second
	pollInterval := 50 * time.Millisecond

	// Mock multiple calls to simulate state transition
	// First call: SUBMITTED
	// Second call: EXPERIENCES_RUNNING
	// Third call: SUCCEEDED
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusSUBMITTED),
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING),
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusSUCCEEDED),
			},
		}, nil).Once()

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", &timeout, pollInterval)

	s.NoError(err)
	s.NotNil(batch)
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
}

func (s *CommandsSuite) TestWaitForBatchCompletion_ByName() {
	projectID := uuid.New()
	batchID := uuid.New()
	batchName := "test-batch"
	timeout := 10 * time.Second
	pollInterval := 100 * time.Millisecond

	// Mock the ListBatchesWithResponse call to find the batch by name
	s.mockClient.On("ListBatchesWithResponse", matchContext, projectID, mock.AnythingOfType("*api.ListBatchesParams")).Return(
		&api.ListBatchesResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListBatchesOutput{
				Batches: &[]api.Batch{
					{
						BatchID:      &batchID,
						FriendlyName: &batchName,
						Status:       Ptr(api.BatchStatusSUCCEEDED),
					},
				},
				NextPageToken: nil,
			},
		}, nil)

	batch, err := waitForBatchCompletion(projectID, "", batchName, &timeout, pollInterval)

	s.NoError(err)
	s.NotNil(batch)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(batchName, *batch.FriendlyName)
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
}
