package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/auth"
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
	validateMetricsCmd = &cobra.Command{
		Use:   "validate",
		Short: "validate - validates your metrics config files against a branch without syncing",
		Long:  "Runs the same validations as `sync` (schema, query building, and backwards-compatibility against the branch's current config) but does not persist anything. The branch must already exist.",
		Run:   validateMetrics,
	}
	debugMetricsCmd = &cobra.Command{
		Use:   "debug",
		Short: "debug - creates a debug dashboard from an emissions file and metrics config",
		Long:  "Creates an ephemeral debug dashboard by uploading an emissions file and metrics config. Polls until the dashboard is ready, then prints its URL. Note: debug dashboards may be cleaned up after 24 hours.",
		Run:   debugMetrics,
	}
	configSchemaMetricsCmd = &cobra.Command{
		Use:   "config-schema",
		Short: "config-schema - prints the JSON schema for the metrics config file",
		Long:  "Fetches and prints the JSON Schema describing the metrics configuration file format. Pipe to a file to use it for editor validation and autocomplete.",
		Run:   getMetricsConfigSchema,
	}
)

const (
	metricsProjectKey            = "project"
	metricsBranchNameKey         = "branch"
	metricsConfigPathKey         = "config-path"
	metricsConfigPathAliasKey    = "metrics-config-path"
	metricsTemplatesPathKey      = "templates-path"
	metricsEmissionsFileKey      = "emissions-file"
	metricsTimeoutKey            = "timeout"
	metricsPollIntervalKey       = "poll-interval"
	metricsSetNameKey            = "metrics-set"
	metricsMediaFilesKey         = "media-file"
	metricsAllowTopicArchivalKey = "allow-topic-archival"
)

func init() {
	syncMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project to sync metrics to")
	syncMetricsCmd.Flags().String(metricsBranchNameKey, "main", "The name of the branch to associate the config with. The default is main")
	syncMetricsCmd.Flags().StringSlice(metricsConfigPathAliasKey, []string{".resim/metrics/config.resim.yml"}, "The path(s) to the metrics config file(s). Supports glob patterns (e.g. \"metrics/*.yml\"). Can be specified multiple times or comma-separated. Files are merged in order. Default is .resim/metrics/config.resim.yml")
	syncMetricsCmd.Flags().StringSlice(metricsConfigPathKey, []string{".resim/metrics/config.resim.yml"}, "Deprecated: use --metrics-config-path instead")
	syncMetricsCmd.Flags().MarkDeprecated(metricsConfigPathKey, "use --metrics-config-path instead")
	syncMetricsCmd.Flags().String(metricsTemplatesPathKey, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates")
	syncMetricsCmd.Flags().Bool(metricsAllowTopicArchivalKey, false, "Confirm archiving any topics this sync would drop. Without this flag, a sync that drops a topic is rejected after previewing the impact.")
	syncMetricsCmd.MarkFlagRequired(metricsProjectKey)
	metricsCmd.AddCommand(syncMetricsCmd)

	validateMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project")
	validateMetricsCmd.Flags().String(metricsBranchNameKey, "main", "The name of the branch to validate against. The default is main")
	validateMetricsCmd.Flags().StringSlice(metricsConfigPathAliasKey, []string{".resim/metrics/config.resim.yml"}, "The path(s) to the metrics config file(s). Supports glob patterns (e.g. \"metrics/*.yml\"). Can be specified multiple times or comma-separated. Files are merged in order. Default is .resim/metrics/config.resim.yml")
	validateMetricsCmd.Flags().String(metricsTemplatesPathKey, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates")
	validateMetricsCmd.MarkFlagRequired(metricsProjectKey)
	metricsCmd.AddCommand(validateMetricsCmd)

	debugMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project")
	debugMetricsCmd.Flags().String(metricsEmissionsFileKey, "", "The path to the emissions file")
	debugMetricsCmd.Flags().String(metricsBranchNameKey, "", "The name of the branch to associate the debug dashboard with (optional)")
	debugMetricsCmd.Flags().StringSlice(metricsConfigPathAliasKey, []string{".resim/metrics/config.resim.yml"}, "The path(s) to the metrics config file(s). Supports glob patterns. Default is .resim/metrics/config.resim.yml")
	debugMetricsCmd.Flags().String(metricsTemplatesPathKey, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates")
	debugMetricsCmd.Flags().String(metricsSetNameKey, "", "The name of the metrics set to use")
	debugMetricsCmd.Flags().Duration(metricsTimeoutKey, 10*time.Minute, "Maximum time to wait for the dashboard to be ready. Default is 10m")
	debugMetricsCmd.Flags().Duration(metricsPollIntervalKey, 10*time.Second, "How often to poll for dashboard readiness. Default is 10s")
	debugMetricsCmd.Flags().StringSlice(metricsMediaFilesKey, []string{}, "Path(s) to media files (images/videos) referenced by the emissions file. Can be specified multiple times.")
	debugMetricsCmd.MarkFlagRequired(metricsProjectKey)
	debugMetricsCmd.MarkFlagRequired(metricsEmissionsFileKey)
	debugMetricsCmd.MarkFlagRequired(metricsSetNameKey)
	metricsCmd.AddCommand(debugMetricsCmd)

	metricsCmd.AddCommand(configSchemaMetricsCmd)

	rootCmd.AddCommand(metricsCmd)
}

func getMetricsConfigSchema(cmd *cobra.Command, args []string) {
	resp, err := bff.GetConfigFileSchema(context.Background(), BffClient)
	if err != nil {
		log.Fatal("failed to fetch metrics config schema: ", err)
	}

	fmt.Println(resp.ConfigFileSchema)
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
	allowTopicArchival := viper.GetBool(metricsAllowTopicArchivalKey)

	// Prefer --metrics-config-path if explicitly set; fall back to deprecated --config-path
	configPaths := viper.GetStringSlice(metricsConfigPathAliasKey)
	if cmd.Flags().Changed(metricsConfigPathKey) && !cmd.Flags().Changed(metricsConfigPathAliasKey) {
		configPaths = viper.GetStringSlice(metricsConfigPathKey)
	}
	templatesPath := viper.GetString(metricsTemplatesPathKey)

	if err := SyncMetricsConfig(projectID, branchID, configPaths, templatesPath, allowTopicArchival, verboseMode); err != nil {
		log.Fatal(err)
	}
}

func validateMetrics(cmd *cobra.Command, args []string) {
	verboseMode := viper.GetBool(verboseKey)
	projectID := getProjectID(Client, viper.GetString(metricsProjectKey))
	branchName := viper.GetString(metricsBranchNameKey)
	branchID := getBranchID(Client, projectID, branchName, true)

	configPaths := viper.GetStringSlice(metricsConfigPathAliasKey)
	templatesPath := viper.GetString(metricsTemplatesPathKey)

	if err := ValidateMetricsConfig(branchID, configPaths, templatesPath, verboseMode); err != nil {
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

	// Read and base64-encode any referenced media files
	mediaFilePaths := viper.GetStringSlice(metricsMediaFilesKey)
	mediaFiles := make([]bff.MediaFileInput, 0, len(mediaFilePaths))
	for _, path := range mediaFilePaths {
		mediaFiles = append(mediaFiles, bff.MediaFileInput{
			Name:     filepath.Base(path),
			Contents: readFile(path),
		})
	}

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
		mediaFiles,
	)
	if err != nil {
		log.Fatalf("failed to create debug dashboard: %v", err)
	}

	dashboardID := resp.CreateDebugDashboard.Id
	fmt.Printf("Dashboard created: %s\n", dashboardID)

	appURL := inferAppURL(viper.GetString(auth.KeyURL))
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
