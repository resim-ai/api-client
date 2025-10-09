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
	metricsProjectKey    = "project"
	metricsBranchNameKey = "branch"
)

func init() {
	syncMetricsCmd.Flags().String(metricsProjectKey, "", "The name or ID of the project to sync metrics to")
	syncMetricsCmd.Flags().String(metricsBranchNameKey, "", "The name of the branch to associate the config with")
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

	if err := SyncMetricsConfig(projectID, branchName, verboseMode); err != nil {
		log.Fatal(err)
	}
}
