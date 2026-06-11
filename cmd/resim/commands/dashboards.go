package commands

import (
	"context"
	"fmt"
	"log"

	"github.com/resim-ai/api-client/auth"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	dashboardCmd = &cobra.Command{
		Use:     "dashboards",
		Short:   "dashboards contains commands for managing dashboards",
		Long:    ``,
		Aliases: []string{"dashboard"},
	}
	createDashboardCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new dashboard",
		Long:  ``,
		Run:   createDashboard,
	}
)

const (
	dashboardProjectKey    = "project"
	dashboardBranchKey     = "branch"
	dashboardNameKey       = "name"
	dashboardDayRangeKey   = "day-range"
	dashboardMetricsSetKey = "metrics-set"
)

func init() {
	createDashboardCmd.Flags().String(dashboardProjectKey, "", "The name or ID of the project")
	createDashboardCmd.MarkFlagRequired(dashboardProjectKey)
	createDashboardCmd.Flags().String(dashboardBranchKey, "", "The name or ID of the branch")
	createDashboardCmd.MarkFlagRequired(dashboardBranchKey)
	createDashboardCmd.Flags().String(dashboardNameKey, "", "The display name for the dashboard")
	createDashboardCmd.MarkFlagRequired(dashboardNameKey)
	createDashboardCmd.Flags().Int(dashboardDayRangeKey, 31, "Number of days of data to include")
	createDashboardCmd.MarkFlagRequired(dashboardDayRangeKey)
	createDashboardCmd.Flags().String(dashboardMetricsSetKey, "", "The name of the metrics set to use")

	dashboardCmd.AddCommand(createDashboardCmd)
	rootCmd.AddCommand(dashboardCmd)
}

func createDashboard(cmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(dashboardProjectKey))
	branchID := getBranchID(Client, projectID, viper.GetString(dashboardBranchKey), true)

	resp, err := bff.CreateDashboard(
		context.Background(),
		BffClient,
		projectID.String(),
		branchID.String(),
		viper.GetString(dashboardNameKey),
		viper.GetInt(dashboardDayRangeKey),
		viper.GetString(dashboardMetricsSetKey),
	)
	if err != nil {
		log.Fatalf("failed to create dashboard: %v", err)
	}

	fmt.Println("Created dashboard successfully!")
	fmt.Printf("Dashboard ID: %s\n", resp.CreateDashboard.Id)
	appURL := inferAppURL(viper.GetString(auth.KeyURL))
	fmt.Printf("%s/projects/%s/dashboards/%s\n", appURL, projectID.String(), resp.CreateDashboard.Id)
}
