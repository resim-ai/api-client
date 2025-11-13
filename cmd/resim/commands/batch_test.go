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
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
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
	s.Equal(50.0, params.RerunMaxFailurePercent)
	s.Equal("test-batch-id", params.BatchID)
	s.Equal("test-batch-name", params.BatchName)
	s.Equal(1*time.Hour, params.Timeout)
	s.Equal(30*time.Second, params.PollInterval)
	s.Len(params.UndesiredConflatedStates, 2)
}

func (s *CommandsSuite) TestGetSuperviseParams_InvalidMaxRerunAttempts() {
	// Set up viper with invalid max rerun attempts
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 0) // Invalid: must be at least 1
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
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
	viper.Set(batchRerunMaxFailurePercentKey, 101.0) // Invalid: must be 1-100
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
	s.Contains(err.Error(), "rerun-max-failure-percent must be greater than 0 and less than 100")
}

func (s *CommandsSuite) TestGetSuperviseParams_ZeroFailurePercent() {
	// Set up viper with zero failure percent
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 1)
	viper.Set(batchRerunMaxFailurePercentKey, 0.0) // Invalid: must be > 0 and less than or equal to 100
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
	s.Contains(err.Error(), "rerun-max-failure-percent must be greater than 0 and less than 100")
}

func (s *CommandsSuite) TestGetSuperviseParams_ValidFailurePercentBoundaries() {
	// Test boundary values for failure percent
	testCases := []int{1, 50, 100}

	for _, failurePercent := range testCases {
		viper.Set(batchProjectKey, "test-project")
		viper.Set(batchMaxRerunAttemptsKey, 1)
		viper.Set(batchRerunMaxFailurePercentKey, float64(failurePercent))
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
		s.Equal(float64(failurePercent), params.RerunMaxFailurePercent)
	}
}

func (s *CommandsSuite) TestGetSuperviseParams_ValidMaxRerunAttemptsBoundaries() {
	// Test boundary values for max rerun attempts
	testCases := []int{1, 5, 10}

	for _, maxAttempts := range testCases {
		viper.Set(batchProjectKey, "test-project")
		viper.Set(batchMaxRerunAttemptsKey, maxAttempts)
		viper.Set(batchRerunMaxFailurePercentKey, 50.0)
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
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
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
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
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
		ProjectID:                projectID,
		MaxRerunAttempts:         3,
		RerunMaxFailurePercent:   50.0,
		UndesiredConflatedStates: []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
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
	matchingJobIDs := getMatchingJobIDs(batch, params, 3)

	// Should not need rerun because max attempts reached
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
		ProjectID:                projectID,
		MaxRerunAttempts:         3,
		RerunMaxFailurePercent:   50.0,
		UndesiredConflatedStates: []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
	}

	// Test with attempt = 0 (well within max attempts)
	matchingJobIDs := getMatchingJobIDs(batch, params, 0)

	// Should not need rerun because batch is cancelled
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
		ProjectID:                projectID,
		MaxRerunAttempts:         3,
		RerunMaxFailurePercent:   50,
		UndesiredConflatedStates: []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
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
	matchingJobIDs := getMatchingJobIDs(batch, params, 0)

	// Should need rerun because there are failed jobs and we're within max attempts
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
		ProjectID:                projectID,
		MaxRerunAttempts:         3,
		RerunMaxFailurePercent:   50.0,
		UndesiredConflatedStates: []api.ConflatedJobStatus{api.ConflatedJobStatusERROR},
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
	matchingJobIDs := getMatchingJobIDs(batch, params, 0)

	// Should not need rerun because there are more than 50% failed jobs
	s.Nil(matchingJobIDs)

}

func (s *CommandsSuite) TestSuperviseBatch_Success() {
	// Create test data first
	projectID := uuid.New()
	batchID := uuid.New()

	// Set up viper with valid parameters
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 3)
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "1h")
	viper.Set(batchWaitPollKey, "100ms")
	viper.Set(batchIDKey, batchID.String()) // Use actual UUID
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
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

	// Mock the batch status transitions: RUNNING -> SUCCEEDED
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusSUCCEEDED), // Second call: succeeded
			},
		}, nil).Once()

	// Mock the jobs list call (for checkRerunNeeded)
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{}, // No failed jobs
			},
		}, nil).Maybe()

	// Call the supervise function
	result := actualSuperviseBatch(nil, []string{})

	// Should succeed with no error
	s.NoError(result.Error)
	s.NotNil(result.Batch)
	s.Equal(api.BatchStatusSUCCEEDED, *result.Batch.Status)
}

func (s *CommandsSuite) TestSuperviseBatch_Cancelled() {
	// Create test data first
	projectID := uuid.New()
	batchID := uuid.New()

	// Set up viper with valid parameters
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 3)
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "1h")
	viper.Set(batchWaitPollKey, "100ms")
	viper.Set(batchIDKey, batchID.String()) // Use actual UUID
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
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

	// Mock the batch status transitions: RUNNING -> CANCELLED
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusCANCELLED), // Second call: cancelled
			},
		}, nil).Once()

	// Mock the jobs list call (for checkRerunNeeded)
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{}, // No failed jobs
			},
		}, nil).Maybe()

	// Call the supervise function
	result := actualSuperviseBatch(nil, []string{})

	// Should succeed with cancelled batch (no error, but batch is cancelled)
	s.NoError(result.Error)
	s.NotNil(result.Batch)
	s.Equal(api.BatchStatusCANCELLED, *result.Batch.Status)
}

func (s *CommandsSuite) TestSuperviseBatch_Timeout() {
	// Create test data first
	projectID := uuid.New()
	batchID := uuid.New()

	// Set up viper with valid parameters
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 3)
	viper.Set(batchRerunMaxFailurePercentKey, 50.0)
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "1s") // Short timeout for testing
	viper.Set(batchWaitPollKey, "500ms")
	viper.Set(batchIDKey, batchID.String()) // Use actual UUID
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
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

	// Mock the batch status to always be running (will cause timeout)
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // Always running
			},
		}, nil)

	// Call the supervise function
	result := actualSuperviseBatch(nil, []string{})

	// Should fail with timeout error
	s.Error(result.Error)
	s.Nil(result.Batch)
	s.Contains(result.Error.Error(), "timeout")
}

func (s *CommandsSuite) TestSuperviseBatch_TooManyFailedJobs_NoRerun() {
	// Create test data first
	projectID := uuid.New()
	batchID := uuid.New()
	jobID1 := uuid.New()
	jobID2 := uuid.New()
	jobID3 := uuid.New()

	// Set up viper with valid parameters
	// Set failure threshold to 33% - with 2 out of 3 jobs failing (66.7%), no rerun should happen
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 3)
	viper.Set(batchRerunMaxFailurePercentKey, 34.0) // 33% threshold
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "1h")
	viper.Set(batchWaitPollKey, "100ms")
	viper.Set(batchIDKey, batchID.String()) // Use actual UUID
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
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

	// Mock the batch status transitions: RUNNING -> ERROR
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusERROR), // Second call: error status
			},
		}, nil).Once()

	// Mock the jobs list call with 3 jobs: 2 failed (Error status), 1 succeeded
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR), // Failed job 1
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR), // Failed job 2
					},
					{
						JobID:           &jobID3,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job
					},
				},
			},
		}, nil).Once()

	// Call the supervise function
	result := actualSuperviseBatch(nil, []string{})

	// Should succeed with no error, but batch has ERROR status
	// No rerun should be attempted because failure rate (66.7%) exceeds threshold (33%)
	s.NoError(result.Error)
	s.NotNil(result.Batch)
	s.Equal(api.BatchStatusERROR, *result.Batch.Status)
}

func (s *CommandsSuite) TestSuperviseBatch_RerunSuccess() {
	// Create test data first
	projectID := uuid.New()
	batchID := uuid.New()
	rerunBatchID := uuid.New()
	jobID1 := uuid.New()
	jobID2 := uuid.New()
	jobID3 := uuid.New()

	// Set up viper with valid parameters
	// Set failure threshold to 33% - with 1 out of 3 jobs failing (33.3%), rerun should happen
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 3)
	viper.Set(batchRerunMaxFailurePercentKey, 34.0) // 33% threshold
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "1h")
	viper.Set(batchWaitPollKey, "100ms")
	viper.Set(batchIDKey, batchID.String()) // Use actual UUID
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
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

	// Mock the initial batch status transitions: RUNNING -> ERROR
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusERROR), // Second call: error status
			},
		}, nil).Once()

	// Mock the jobs list call for initial batch with 3 jobs: 1 failed (Error status), 2 succeeded
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR), // Failed job 1
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job 2
					},
					{
						JobID:           &jobID3,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job 3
					},
				},
			},
		}, nil).Once()

	// Mock the rerun batch submission
	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.RerunBatchOutput{
				BatchID: &rerunBatchID,
			},
		}, nil).Once()

	// Mock the rerun batch status transitions: RUNNING -> SUCCEEDED
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, rerunBatchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &rerunBatchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, rerunBatchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &rerunBatchID,
				Status:  Ptr(api.BatchStatusSUCCEEDED), // Second call: succeeded
			},
		}, nil).Once()

	// Mock the jobs list call for rerun batch with all jobs succeeded
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Now succeeded
					},
				},
			},
		}, nil).Once()

	// Call the supervise function
	result := actualSuperviseBatch(nil, []string{})

	// Should succeed with no error, and batch has SUCCEEDED status
	s.NoError(result.Error)
	s.NotNil(result.Batch)
	s.Equal(api.BatchStatusSUCCEEDED, *result.Batch.Status)
}

func (s *CommandsSuite) TestSuperviseBatch_RerunFails_MaxAttemptsReached() {
	// Create test data first
	projectID := uuid.New()
	batchID := uuid.New()
	rerunBatchID := uuid.New()
	jobID1 := uuid.New()
	jobID2 := uuid.New()
	jobID3 := uuid.New()

	// Set up viper with valid parameters
	// Set max rerun attempts to 1 - after first rerun fails, no more reruns should happen
	viper.Set(batchProjectKey, "test-project")
	viper.Set(batchMaxRerunAttemptsKey, 1)          // Only 1 rerun attempt allowed
	viper.Set(batchRerunMaxFailurePercentKey, 34.0) // 33% threshold
	viper.Set(batchRerunOnStatesKey, "Error")
	viper.Set(batchWaitTimeoutKey, "1h")
	viper.Set(batchWaitPollKey, "100ms")
	viper.Set(batchIDKey, batchID.String()) // Use actual UUID
	viper.Set(batchNameKey, "test-batch-name")

	// Mock the project lookup
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

	// Mock the initial batch status transitions: RUNNING -> ERROR
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, batchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &batchID,
				Status:  Ptr(api.BatchStatusERROR), // Second call: error status
			},
		}, nil).Once()

	// Mock the jobs list call for initial batch with 3 jobs: 1 failed (Error status), 2 succeeded
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR), // Failed job 1
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job 2
					},
					{
						JobID:           &jobID3,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job 3
					},
				},
			},
		}, nil).Once()

	// Mock the rerun batch submission
	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.RerunBatchOutput{
				BatchID: &rerunBatchID,
			},
		}, nil).Once()

	// Mock the rerun batch status transitions: RUNNING -> ERROR
	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, rerunBatchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &rerunBatchID,
				Status:  Ptr(api.BatchStatusEXPERIENCESRUNNING), // First call: still running
			},
		}, nil).Once()

	s.mockClient.On("GetBatchWithResponse", matchContext, projectID, rerunBatchID).Return(
		&api.GetBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Batch{
				BatchID: &rerunBatchID,
				Status:  Ptr(api.BatchStatusERROR), // Second call: error status again
			},
		}, nil).Once()

	// Mock the jobs list call for rerun batch with 1 job still failed
	s.mockClient.On("ListJobsWithResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.ListJobsResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.ListJobsOutput{
				Jobs: &[]api.Job{
					{
						JobID:           &jobID1,
						ConflatedStatus: Ptr(api.ConflatedJobStatusERROR), // Still failed
					},
					{
						JobID:           &jobID2,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job 2
					},
					{
						JobID:           &jobID3,
						ConflatedStatus: Ptr(api.ConflatedJobStatusPASSED), // Successful job 3
					},
				},
			},
		}, nil).Maybe() // this shouldnt be hit because we have already exhausted max rerun attempts

	// Call the supervise function
	result := actualSuperviseBatch(nil, []string{})

	// Should succeed with no error, but batch has ERROR status
	// No more reruns should be attempted because max attempts (1) reached
	s.NoError(result.Error)
	s.NotNil(result.Batch)
	s.Equal(api.BatchStatusERROR, *result.Batch.Status)
}

func (s *CommandsSuite) TestBatchRerun_ConflictShouldRetry() {
	projectID := uuid.New()
	batchID := uuid.New()
	rerunBatchID := uuid.New()
	jobID := uuid.New()

	// Mock the rerun batch submission - first call returns 409 Conflict
	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusConflict, // First attempt: conflict
			},
		}, nil).Once()

	// Mock the rerun batch submission - second call returns 200 OK
	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK, // Second attempt: success
			},
			JSON200: &api.RerunBatchOutput{
				BatchID: &rerunBatchID,
			},
		}, nil).Once()

	// Call submitBatchRerun directly
	response, err := submitBatchRerun(projectID, batchID, []uuid.UUID{jobID}, 1*time.Second, false)

	// Should succeed with no error after retrying
	s.NoError(err)
	s.NotNil(response)
	s.NotNil(response.JSON200)
	s.Equal(rerunBatchID, *response.JSON200.BatchID)
}

func (s *CommandsSuite) TestBatchRerun_ConflictMaxRetriesReached() {
	projectID := uuid.New()
	batchID := uuid.New()
	jobID := uuid.New()

	// Mock the rerun batch submission - all 3 attempts return 409 Conflict
	// This will exhaust all retries and return an error
	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusConflict, // First attempt: conflict
			},
		}, nil).Once()

	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusConflict, // Second attempt: conflict
			},
		}, nil).Once()

	s.mockClient.On("RerunBatchWithResponse", matchContext, projectID, batchID, mock.Anything).Return(
		&api.RerunBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusConflict, // Third attempt: conflict (max retries reached)
			},
		}, nil).Once()

	// Call submitBatchRerun directly
	response, err := submitBatchRerun(projectID, batchID, []uuid.UUID{jobID}, 1*time.Second, false)

	// Should return an error after max retries are reached
	s.Error(err)
	s.Nil(response)
	s.Contains(err.Error(), "max retries reached")
}
