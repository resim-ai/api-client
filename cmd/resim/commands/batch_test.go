package commands

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, batchID.String(), "", timeout, pollInterval)

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

	batch, err := waitForBatchCompletion(projectID, "", batchName, timeout, pollInterval)

	s.NoError(err)
	s.NotNil(batch)
	s.Equal(batchID, *batch.BatchID)
	s.Equal(batchName, *batch.FriendlyName)
	s.Equal(api.BatchStatusSUCCEEDED, *batch.Status)
}

func (s *CommandsSuite) TestGetSuperviseParams_Valid() {
	// Set up viper with valid parameters
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 3)
	viper.Set(batchRerunMaxFailurePercentKey, 50)
	viper.Set(batchRerunOnStatesKey, "Error,Warning")
	viper.Set(batchWaitTimeoutKey, "1h")
	viper.Set(batchWaitPollKey, "30s")
	viper.Set(batchIDKey, "test-batch-id")
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
	projectID := uuid.New()
	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true
	})).Return(
		&api.ListProjectsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListProjectsOutput{
				Projects: &[]api.Project{
					{
						ProjectID: projectID,
						Name:      "test-project",
					},
				},
			},
		}, nil)

	params, err := getSuperviseParams(nil, []string{})

	s.NoError(err)
	s.NotNil(params)
	s.Equal(projectID, params.ProjectID)
	s.Equal(3, params.MaxRerunAttempts)
	s.Equal(50, params.RerunMaxFailurePercent)
	s.Equal("test-batch-id", params.BatchID)
	s.Equal("test-batch-name", params.BatchName)
	s.Equal(1*time.Hour, params.Timeout)
	s.Equal(30*time.Second, params.PollInterval)
	s.Len(params.ConflatedStates, 2)
}

func (s *CommandsSuite) TestGetSuperviseParams_InvalidMaxRerunAttempts() {
	// Set up viper with invalid max rerun attempts
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 0) // Invalid: must be at least 1
	viper.Set(batchRerunMaxFailurePercentKey, 50)
	viper.Set(batchRerunOnStatesKey, "Error")

	// Mock the project lookup - since "test-project" is not a UUID, it will call ListProjectsWithResponse
	projectID := uuid.New()
	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true // Accept any ListProjectsParams
	})).Return(
		&api.ListProjectsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListProjectsOutput{
				Projects: &[]api.Project{
					{
						ProjectID: projectID,
						Name:      "test-project",
					},
				},
			},
		}, nil)

	params, err := getSuperviseParams(nil, []string{})

	s.Error(err)
	s.Nil(params)
	s.Contains(err.Error(), "max-rerun-attempts must be at least 1")
}

func (s *CommandsSuite) TestGetSuperviseParams_InvalidFailurePercent() {
	// Set up viper with invalid failure percent
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 1)
	viper.Set(batchRerunMaxFailurePercentKey, 101) // Invalid: must be 1-100
	viper.Set(batchRerunOnStatesKey, "Error")

	// Mock the project lookup
	projectID := uuid.New()
	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true
	})).Return(
		&api.ListProjectsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListProjectsOutput{
				Projects: &[]api.Project{
					{
						ProjectID: projectID,
						Name:      "test-project",
					},
				},
			},
		}, nil)

	params, err := getSuperviseParams(nil, []string{})

	s.Error(err)
	s.Nil(params)
	s.Contains(err.Error(), "rerun-max-failure-percent must be between 1 and 100")
}

func (s *CommandsSuite) TestGetSuperviseParams_ZeroFailurePercent() {
	// Set up viper with zero failure percent
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 1)
	viper.Set(batchRerunMaxFailurePercentKey, 0) // Invalid: must be 1-100
	viper.Set(batchRerunOnStatesKey, "Error")

	// Mock the project lookup
	projectID := uuid.New()
	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true
	})).Return(
		&api.ListProjectsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListProjectsOutput{
				Projects: &[]api.Project{
					{
						ProjectID: projectID,
						Name:      "test-project",
					},
				},
			},
		}, nil)

	params, err := getSuperviseParams(nil, []string{})

	s.Error(err)
	s.Nil(params)
	s.Contains(err.Error(), "rerun-max-failure-percent must be between 1 and 100")
}

func (s *CommandsSuite) TestGetSuperviseParams_ValidFailurePercentBoundaries() {
	// Test boundary values for failure percent
	testCases := []int{1, 50, 100}

	for _, failurePercent := range testCases {
		viper.Set(batchProjectKey, "test-project")
		viper.Set(batchMaxRerunAttemptsKey, 1)
		viper.Set(batchRerunMaxFailurePercentKey, failurePercent)
		viper.Set(batchRerunOnStatesKey, "Error")

		// Mock the project lookup
		projectID := uuid.New()
		s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
			return true
		})).Return(
			&api.ListProjectsResponse{
				HTTPResponse: &http.Response{
					StatusCode: http.StatusOK,
				},
				JSON200: &api.ListProjectsOutput{
					Projects: &[]api.Project{
						{
							ProjectID: projectID,
							Name:      "test-project",
						},
					},
				},
			}, nil).Once() // Use Once() to avoid mock conflicts in the loop

		params, err := getSuperviseParams(nil, []string{})

		s.NoError(err, "Should not error for failure percent %d", failurePercent)
		s.NotNil(params)
		s.Equal(failurePercent, params.RerunMaxFailurePercent)
	}
}

func (s *CommandsSuite) TestGetSuperviseParams_ValidMaxRerunAttemptsBoundaries() {
	// Test boundary values for max rerun attempts
	testCases := []int{1, 5, 10}

	for _, maxAttempts := range testCases {
		viper.Set(batchProjectKey, "test-project")
		viper.Set(batchMaxRerunAttemptsKey, maxAttempts)
		viper.Set(batchRerunMaxFailurePercentKey, 50)
		viper.Set(batchRerunOnStatesKey, "Error")

		// Mock the project lookup
		projectID := uuid.New()
		s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
			return true
		})).Return(
			&api.ListProjectsResponse{
				HTTPResponse: &http.Response{
					StatusCode: http.StatusOK,
				},
				JSON200: &api.ListProjectsOutput{
					Projects: &[]api.Project{
						{
							ProjectID: projectID,
							Name:      "test-project",
						},
					},
				},
			}, nil).Once() // Use Once() to avoid mock conflicts in the loop

		params, err := getSuperviseParams(nil, []string{})

		s.NoError(err, "Should not error for max attempts %d", maxAttempts)
		s.NotNil(params)
		s.Equal(maxAttempts, params.MaxRerunAttempts)
	}
}

func (s *CommandsSuite) TestGetSuperviseParams_DefaultTimeouts() {
	// Test that default timeouts are parsed correctly
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 1)
	viper.Set(batchRerunMaxFailurePercentKey, 50)
	viper.Set(batchRerunOnStatesKey, "Error")
	// Don't set timeout and poll interval to test defaults

	// Mock the project lookup
	projectID := uuid.New()
	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true
	})).Return(
		&api.ListProjectsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListProjectsOutput{
				Projects: &[]api.Project{
					{
						ProjectID: projectID,
						Name:      "test-project",
					},
				},
			},
		}, nil)

	params, err := getSuperviseParams(nil, []string{})

	s.NoError(err)
	s.NotNil(params)
	// Default values should be parsed (these depend on your viper defaults)
	s.NotZero(params.Timeout)
	s.NotZero(params.PollInterval)
}

func (s *CommandsSuite) TestGetSuperviseParams_CustomTimeouts() {
	// Test custom timeout values
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 1)
	viper.Set(batchRerunMaxFailurePercentKey, 50)
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "2h")
	viper.Set(batchWaitPollKey, "45s")

	// Mock the project lookup
	projectID := uuid.New()
	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true
	})).Return(
		&api.ListProjectsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListProjectsOutput{
				Projects: &[]api.Project{
					{
						ProjectID: projectID,
						Name:      "test-project",
					},
				},
			},
		}, nil)

	params, err := getSuperviseParams(nil, []string{})

	s.NoError(err)
	s.NotNil(params)
	s.Equal(2*time.Hour, params.Timeout)
	s.Equal(45*time.Second, params.PollInterval)
}

func (s *CommandsSuite) TestCheckRerunNeeded_RerunAttemptsExceeded() {
	// Create test data
	projectID := uuid.New()
	batchID := uuid.New()
	jobID1 := uuid.New()
	jobID2 := uuid.New()

	// Create a batch with ERROR status
	batch := &api.Batch{
		BatchID: &batchID,
		Status:  Ptr(api.BatchStatusERROR),
	}

	// Create params with max rerun attempts = 3
	params := &SuperviseParams{
		ProjectID:              projectID,
		MaxRerunAttempts:       3,
		RerunMaxFailurePercent: 50,
		ConflatedStates:        []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
	}

	// Mock the ListJobsWithResponse call that getAllJobs makes
	// Use Maybe() to make the mock optional - this avoids tight coupling
	// The test will pass whether the API call is made or not
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR),
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR),
					},
				},
			},
		}, nil).Maybe()

	// Test with attempt = 3 (equal to max attempts)
	rerunNeeded, matchingJobIDs := checkRerunNeeded(batch, params, 3)

	// Should not need rerun because max attempts reached
	s.False(rerunNeeded)
	s.Nil(matchingJobIDs)
}

func (s *CommandsSuite) TestCheckRerunNeeded_BatchStatusCancelled() {
	// Create test data
	projectID := uuid.New()
	batchID := uuid.New()

	// Create a batch with CANCELLED status
	batch := &api.Batch{
		BatchID: &batchID,
		Status:  Ptr(api.BatchStatusCANCELLED),
	}

	// Create params
	params := &SuperviseParams{
		ProjectID:              projectID,
		MaxRerunAttempts:       3,
		RerunMaxFailurePercent: 50,
		ConflatedStates:        []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
	}

	// Test with attempt = 0 (well within max attempts)
	rerunNeeded, matchingJobIDs := checkRerunNeeded(batch, params, 0)

	// Should not need rerun because batch is cancelled
	s.False(rerunNeeded)
	s.Nil(matchingJobIDs)

	// Verify that no API calls were made since we return early for cancelled batches
	s.mockClient.AssertNotCalled(s.T(), "ListJobsWithResponse")
}

func (s *CommandsSuite) TestCheckRerunNeeded_ValidRerunScenario() {
	// Create test data
	projectID := uuid.New()
	batchID := uuid.New()
	jobID1 := uuid.New()
	jobID2 := uuid.New()

	// Create a batch with ERROR status
	batch := &api.Batch{
		BatchID: &batchID,
		Status:  Ptr(api.BatchStatusERROR),
	}

	// Create params with max rerun attempts = 3
	params := &SuperviseParams{
		ProjectID:              projectID,
		MaxRerunAttempts:       3,
		RerunMaxFailurePercent: 50,
		ConflatedStates:        []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
	}

	// Mock the ListJobsWithResponse call that getAllJobs makes
	s.mockClient.On("ListJobsWithResponse", matchContext, projectID, batchID, mock.MatchedBy(func(params *api.ListJobsParams) bool {
		return true
	})).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR),
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // This one passed
					},
				},
			},
		}, nil)

	// Test with attempt = 0 (within max attempts)
	rerunNeeded, matchingJobIDs := checkRerunNeeded(batch, params, 0)

	// Should need rerun because there are failed jobs and we're within max attempts
	s.True(rerunNeeded)
	s.Len(matchingJobIDs, 1)
	s.Equal(jobID1, matchingJobIDs[0])
}

func (s *CommandsSuite) TestCheckRerunNeeded_TooManyFailedJobs() {
	// Create test data
	projectID := uuid.New()
	batchID := uuid.New()
	jobID1 := uuid.New()
	jobID2 := uuid.New()

	// Create a batch with ERROR status
	batch := &api.Batch{
		BatchID: &batchID,
		Status:  Ptr(api.BatchStatusERROR),
	}

	// Create params with max rerun attempts = 3
	params := &SuperviseParams{
		ProjectID:              projectID,
		MaxRerunAttempts:       3,
		RerunMaxFailurePercent: 50,
		ConflatedStates:        []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
	}

	// Mock the ListJobsWithResponse call that getAllJobs makes
	s.mockClient.On("ListJobsWithResponse", matchContext, projectID, batchID, mock.MatchedBy(func(params *api.ListJobsParams) bool {
		return true
	})).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR),
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR), // Both failed so more than 50%
					},
				},
			},
		}, nil)

	// Test with attempt = 0 (within max attempts)
	rerunNeeded, matchingJobIDs := checkRerunNeeded(batch, params, 0)

	// Should need rerun because there are failed jobs and we're within max attempts
	s.False(rerunNeeded)
	s.Nil(matchingJobIDs)

}
