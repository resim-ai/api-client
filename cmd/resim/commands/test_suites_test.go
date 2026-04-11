package commands

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

// setupRunTestSuiteMocks sets up the common mocks needed for runTestSuite tests.
// It configures viper flags and mocks for project lookup and test suite retrieval.
// Returns the projectID, testSuiteID, buildID, and the test suite's metricsBuildID.
func (s *CommandsSuite) setupRunTestSuiteMocks(metricsBuildID *uuid.UUID) (uuid.UUID, uuid.UUID, uuid.UUID) {
	viper.Reset()
	projectID := uuid.New()
	testSuiteID := uuid.New()
	buildID := uuid.New()
	batchID := uuid.New()
	batchStatus := api.BatchStatusSUBMITTED
	friendlyName := "test-batch"

	viper.Set(testSuiteProjectKey, projectID.String())
	viper.Set(testSuiteKey, testSuiteID.String())
	viper.Set(testSuiteBuildIDKey, buildID.String())

	// Mock the project lookup (by UUID)
	s.mockClient.On("GetProjectWithResponse", matchContext, projectID).Return(
		&api.GetProjectResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.Project{
				ProjectID: projectID,
				Name:      "test-project",
			},
		}, nil)

	// Mock the test suite lookup (by UUID)
	s.mockClient.On("GetTestSuiteWithResponse", matchContext, projectID, testSuiteID).Return(
		&api.GetTestSuiteResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
			JSON200: &api.TestSuite{
				TestSuiteID:       testSuiteID,
				TestSuiteRevision: 1,
				MetricsBuildID:    metricsBuildID,
				Experiences:       []uuid.UUID{uuid.New()},
			},
		}, nil)

	// Mock the batch creation response
	s.mockClient.On("CreateBatchWithResponse", matchContext, projectID, mock.AnythingOfType("api.BatchInput")).Return(
		&api.CreateBatchResponse{
			HTTPResponse: &http.Response{
				StatusCode: http.StatusCreated,
			},
			JSON201: &api.Batch{
				BatchID:      &batchID,
				FriendlyName: &friendlyName,
				Status:       &batchStatus,
			},
		}, nil)

	return projectID, testSuiteID, buildID
}

func (s *CommandsSuite) TestRunTestSuiteWithMetricsSetOverridePreservesBuildID() {
	originalMetricsBuildID := uuid.New()
	metricsSetOverride := "my-override-metrics-set"

	s.setupRunTestSuiteMocks(&originalMetricsBuildID)
	viper.Set(testSuiteMetricsSetOverrideKey, metricsSetOverride)

	runTestSuite(nil, nil)

	// Assert CreateBatchWithResponse was called with the correct body
	calls := s.mockClient.Calls
	for _, call := range calls {
		if call.Method == "CreateBatchWithResponse" {
			body := call.Arguments.Get(2).(api.BatchInput)
			s.NotNil(body.MetricsBuildID, "MetricsBuildID should not be nil when test suite has one")
			s.Equal(originalMetricsBuildID, *body.MetricsBuildID, "MetricsBuildID should be preserved from the test suite")
			s.NotNil(body.MetricsSetName, "MetricsSetName should be set")
			s.Equal(metricsSetOverride, *body.MetricsSetName, "MetricsSetName should be the override value")
			return
		}
	}
	s.Fail("CreateBatchWithResponse was not called")
}

func (s *CommandsSuite) TestRunTestSuiteWithBothOverrides() {
	originalMetricsBuildID := uuid.New()
	overrideMetricsBuildID := uuid.New()
	metricsSetOverride := "my-override-metrics-set"

	s.setupRunTestSuiteMocks(&originalMetricsBuildID)
	viper.Set(testSuiteMetricsSetOverrideKey, metricsSetOverride)
	viper.Set(testSuiteMetricsBuildOverrideKey, overrideMetricsBuildID.String())

	runTestSuite(nil, nil)

	// Assert CreateBatchWithResponse was called with both overrides
	calls := s.mockClient.Calls
	for _, call := range calls {
		if call.Method == "CreateBatchWithResponse" {
			body := call.Arguments.Get(2).(api.BatchInput)
			s.NotNil(body.MetricsBuildID, "MetricsBuildID should not be nil")
			s.Equal(overrideMetricsBuildID, *body.MetricsBuildID, "MetricsBuildID should be the override value, not the original")
			s.NotNil(body.MetricsSetName, "MetricsSetName should be set")
			s.Equal(metricsSetOverride, *body.MetricsSetName, "MetricsSetName should be the override value")
			return
		}
	}
	s.Fail("CreateBatchWithResponse was not called")
}

func (s *CommandsSuite) TestRunTestSuiteWithMetricsSetOverrideNilBuildID() {
	metricsSetOverride := "my-override-metrics-set"

	s.setupRunTestSuiteMocks(nil) // test suite has no MetricsBuildID
	viper.Set(testSuiteMetricsSetOverrideKey, metricsSetOverride)

	runTestSuite(nil, nil)

	// Assert CreateBatchWithResponse was called with nil MetricsBuildID
	calls := s.mockClient.Calls
	for _, call := range calls {
		if call.Method == "CreateBatchWithResponse" {
			body := call.Arguments.Get(2).(api.BatchInput)
			s.Nil(body.MetricsBuildID, "MetricsBuildID should be nil when test suite has no MetricsBuildID")
			s.NotNil(body.MetricsSetName, "MetricsSetName should be set")
			s.Equal(metricsSetOverride, *body.MetricsSetName, "MetricsSetName should be the override value")
			return
		}
	}
	s.Fail("CreateBatchWithResponse was not called")
}
