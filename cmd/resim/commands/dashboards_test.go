package commands

import (
	"net/http"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateDashboardCmdHasRequiredFlags(t *testing.T) {
	requiredFlags := []string{
		dashboardProjectKey,
		dashboardBranchKey,
		dashboardNameKey,
		dashboardDayRangeKey,
	}
	for _, name := range requiredFlags {
		flag := createDashboardCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on createDashboardCmd", name)
		assert.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should be required", name)
	}
}

func TestCreateDashboardCmdMetricsSetIsOptional(t *testing.T) {
	flag := createDashboardCmd.Flags().Lookup(dashboardMetricsSetKey)
	assert.NotNil(t, flag, "--metrics-set flag should exist on createDashboardCmd")
	assert.Nil(t, flag.Annotations[cobra.BashCompOneRequiredFlag], "--metrics-set should not be required")
}

func (s *CommandsSuite) TestCreateDashboard_Success() {
	projectID := uuid.New()
	branchID := uuid.New()

	viper.Set(dashboardProjectKey, "test-project")
	viper.Set(dashboardBranchKey, "main")
	viper.Set(dashboardNameKey, "From CLI")
	viper.Set(dashboardDayRangeKey, 180)
	viper.Set(dashboardMetricsSetKey, "my dashboard")

	s.mockClient.On("ListProjectsWithResponse", matchContext, mock.MatchedBy(func(params *api.ListProjectsParams) bool {
		return true
	})).Return(&api.ListProjectsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListProjectsOutput{
			Projects: &[]api.Project{{ProjectID: projectID, Name: "test-project"}},
		},
	}, nil)

	s.mockClient.On("ListBranchesForProjectWithResponse", matchContext, projectID, mock.Anything).Return(&api.ListBranchesForProjectResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListBranchesOutput{
			Branches: &[]api.Branch{{BranchID: branchID, Name: "main"}},
		},
	}, nil)

	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(func(req *graphql.Request) bool {
		return req.OpName == "CreateDashboard"
	}), mock.Anything).Run(func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.CreateDashboardResponse)
		data.CreateDashboard = bff.CreateDashboardCreateDashboard{
			Id:   "dash-abc-123",
			Name: "From CLI",
		}
	}).Return(nil).Once()

	origBffClient := BffClient
	BffClient = mockBff
	defer func() { BffClient = origBffClient }()

	createDashboard(nil, []string{})

	mockBff.AssertExpectations(s.T())
}
