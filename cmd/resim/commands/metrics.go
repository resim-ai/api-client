package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	metricsCmd = &cobra.Command{
		Use:   "metrics",
		Short: "metrics contains commands for managing your metrics configuration",
		Long:  ``,
	}
	syncMetricsCmd = &cobra.Command{
		Use:   "sync",
		Short: "sync - syncs your metrics config files with ReSim",
		Long:  ``,
		Run:   syncMetrics,
	}
	debugMetricsCmd = &cobra.Command{
		Use:   "debug",
		Short: "debug - creates a debug dashboard from an emissions file and metrics config",
		Long:  "Creates an ephemeral debug dashboard by uploading an emissions file and metrics config. Polls until the dashboard is ready, then prints its URL. Note: debug dashboards may be cleaned up after 24 hours.",
		Run:   debugMetrics,
	}
)

const (
	metricsProjectKey         = "project"
	metricsBranchNameKey      = "branch"
	metricsConfigPathKey      = "config-path"
	metricsConfigPathAliasKey = "metrics-config-path"
	metricsTemplatesPathKey   = "templates-path"
	metricsEmissionsFileKey   = "emissions-file"
	metricsTimeoutKey         = "timeout"
	metricsPollIntervalKey    = "poll-interval"
	metricsSetNameKey         = "metrics-set"
)

func init() {
	syncMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project to sync metrics to")
	syncMetricsCmd.Flags().String(metricsBranchNameKey, "main", "The name of the branch to associate the config with. The default is main")
	syncMetricsCmd.Flags().StringSlice(metricsConfigPathAliasKey, []string{".resim/metrics/config.yml"}, "The path(s) to the metrics config file(s). Supports glob patterns (e.g. \"metrics/*.yml\"). Can be specified multiple times or comma-separated. Files are merged in order. Default is .resim/metrics/config.yml")
	syncMetricsCmd.Flags().StringSlice(metricsConfigPathKey, []string{".resim/metrics/config.yml"}, "Deprecated: use --metrics-config-path instead")
	syncMetricsCmd.Flags().MarkDeprecated(metricsConfigPathKey, "use --metrics-config-path instead")
	syncMetricsCmd.Flags().String(metricsTemplatesPathKey, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates")
	syncMetricsCmd.MarkFlagRequired(metricsProjectKey)
	metricsCmd.AddCommand(syncMetricsCmd)

	debugMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project")
	debugMetricsCmd.Flags().String(metricsEmissionsFileKey, "", "The path to the emissions file")
	debugMetricsCmd.Flags().String(metricsBranchNameKey, "", "The name of the branch to associate the debug dashboard with (optional)")
	debugMetricsCmd.Flags().StringSlice(metricsConfigPathAliasKey, []string{".resim/metrics/config.yml"}, "The path(s) to the metrics config file(s). Supports glob patterns. Default is .resim/metrics/config.yml")
	debugMetricsCmd.Flags().String(metricsTemplatesPathKey, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates")
	debugMetricsCmd.Flags().String(metricsSetNameKey, "", "The name of the metrics set to use")
	debugMetricsCmd.Flags().Duration(metricsTimeoutKey, 10*time.Minute, "Maximum time to wait for the dashboard to be ready. Default is 10m")
	debugMetricsCmd.Flags().Duration(metricsPollIntervalKey, 10*time.Second, "How often to poll for dashboard readiness. Default is 10s")
	debugMetricsCmd.MarkFlagRequired(metricsProjectKey)
	debugMetricsCmd.MarkFlagRequired(metricsEmissionsFileKey)
	debugMetricsCmd.MarkFlagRequired(metricsSetNameKey)
	metricsCmd.AddCommand(debugMetricsCmd)

	rootCmd.AddCommand(metricsCmd)
}

// Read the given file and return a base64 encoded string of the file contents
func readFile(path string) string {
	file, err := os.ReadFile(path)

	if err != nil {
		log.Fatalf("Failed to read file %s: %s", path, err)
	}

	return base64.StdEncoding.EncodeToString(file)
}

func syncMetrics(cmd *cobra.Command, args []string) {
	verboseMode := viper.GetBool(verboseKey)
	projectID := getProjectID(Client, viper.GetString(metricsProjectKey))
	branchName := viper.GetString(metricsBranchNameKey)
	branchID := getBranchID(Client, projectID, branchName, true)

	// Prefer --metrics-config-path if explicitly set; fall back to deprecated --config-path
	configPaths := viper.GetStringSlice(metricsConfigPathAliasKey)
	if cmd.Flags().Changed(metricsConfigPathKey) && !cmd.Flags().Changed(metricsConfigPathAliasKey) {
		configPaths = viper.GetStringSlice(metricsConfigPathKey)
	}
	templatesPath := viper.GetString(metricsTemplatesPathKey)

	if err := SyncMetricsConfig(projectID, branchID, configPaths, templatesPath, verboseMode); err != nil {
		log.Fatal(err)
	}
}

func debugMetrics(cmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(metricsProjectKey))
	emissionsFilePath := viper.GetString(metricsEmissionsFileKey)
	configPaths := viper.GetStringSlice(metricsConfigPathAliasKey)
	templatesPath := viper.GetString(metricsTemplatesPathKey)
	timeout := viper.GetDuration(metricsTimeoutKey)
	pollInterval := viper.GetDuration(metricsPollIntervalKey)

	// Resolve optional branch
	branchID := ""
	if cmd.Flags().Changed(metricsBranchNameKey) {
		branchName := viper.GetString(metricsBranchNameKey)
		resolvedBranchID := getBranchID(Client, projectID, branchName, true)
		branchID = resolvedBranchID.String()
	}

	metricsSetName := viper.GetString(metricsSetNameKey)

	// Reuse shared helpers for config and templates
	configB64, err := prepareMetricsConfig(configPaths, false)
	if err != nil {
		log.Fatal(err)
	}

	templates, err := readTemplates(templatesPath, false)
	if err != nil {
		log.Fatal(err)
	}

	// Read and base64-encode emissions file
	emissionsB64 := readFile(emissionsFilePath)

	// Create the debug dashboard
	fmt.Println("Creating debug dashboard...")
	resp, err := bff.CreateDebugDashboard(
		context.Background(),
		BffClient,
		projectID.String(),
		configB64,
		templates,
		emissionsB64,
		branchID,
		metricsSetName,
	)
	if err != nil {
		log.Fatalf("failed to create debug dashboard: %v", err)
	}

	dashboardID := resp.CreateDebugDashboard.Id
	fmt.Printf("Dashboard created: %s\n", dashboardID)

	appURL := inferAppURL(viper.GetString(urlKey))
	dashboardURL := fmt.Sprintf("%s/projects/%s/debug/%s", appURL, projectID.String(), dashboardID)

	// Poll until ready
	s := NewSpinner(cmd)
	err = waitForDashboardReady(context.Background(), BffClient, dashboardID, timeout, pollInterval, s)
	if err != nil {
		if _, ok := err.(*TimeoutError); ok {
			log.Fatalf("Timed out waiting for dashboard to be ready: %v\nDashboard URL: %s", err, dashboardURL)
		}
		log.Fatalf("Error waiting for dashboard: %v\nDashboard URL: %s", err, dashboardURL)
	}

	fmt.Println(dashboardURL)
}

func waitForDashboardReady(ctx context.Context, client graphql.Client, dashboardID string, timeout time.Duration, pollInterval time.Duration, s *Spinner) error {
	startTime := time.Now()
	s.Start("Waiting for dashboard to be ready...")

	for {
		resp, err := bff.GetDashboard(ctx, client, dashboardID)
		if err != nil {
			s.Stop(nil)
			return fmt.Errorf("failed to get dashboard: %w", err)
		}

		if resp.Dashboard.LastRanAt != "" {
			msg := "Dashboard is ready!\n"
			s.Stop(&msg)
			return nil
		}

		elapsed := time.Since(startTime)
		if elapsed >= timeout {
			s.Stop(nil)
			return &TimeoutError{message: fmt.Sprintf("timeout after %v waiting for dashboard %s to be ready", timeout, dashboardID)}
		}

		s.Update(fmt.Sprintf("Waiting for dashboard to be ready (%s elapsed)...", elapsed.Round(time.Second)))
		time.Sleep(pollInterval)
	}
}

func inferAppURL(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalf("error parsing API url: %v", err)
	}

	u.Path = ""
	if strings.Contains(u.Host, "localhost") {
		u.Host = "localhost:3000"
	} else {
		u.Host = strings.Replace(u.Host, "api.", "app.", 1)
	}
	return u.String()
}
