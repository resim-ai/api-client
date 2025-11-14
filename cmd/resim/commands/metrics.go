package commands

import (
	"encoding/base64"
	"log"
	"os"

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
)

const (
	metricsProjectKey       = "project"
	metricsBranchNameKey    = "branch"
	metricsConfigPathKey    = "config-path"
	metricsTemplatesPathKey = "templates-path"
)

func init() {
	syncMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project to sync metrics to")
	syncMetricsCmd.Flags().String(metricsBranchNameKey, "main", "The name of the branch to associate the config with. The default is main")
	syncMetricsCmd.Flags().String(metricsConfigPathKey, ".resim/metrics/config.yml", "The path to the metrics config file. Default is .resim/metrics/config.yml")
	syncMetricsCmd.Flags().String(metricsTemplatesPathKey, ".resim/metrics/templates", "The path to the metrics templates directory. Default is .resim/metrics/templates")
	syncMetricsCmd.MarkFlagRequired(metricsProjectKey)
	metricsCmd.AddCommand(syncMetricsCmd)
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
	configPath := viper.GetString(metricsConfigPathKey)
	templatesPath := viper.GetString(metricsTemplatesPathKey)

	if err := SyncMetricsConfig(projectID, branchID, configPath, templatesPath, verboseMode); err != nil {
		log.Fatal(err)
	}
}
