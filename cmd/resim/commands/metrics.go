package commands

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"path"
	"strings"

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
)

func init() {
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

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %s", err)
	}

	configFilePath := path.Join(workDir, ".resim/metrics/config.yml")
	if verboseMode {
		log.Println("Looking for metrics config at .resim/metrics/config.yml")
	}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Fatalf("failed to find ReSim metrics config at %s\nAre you in the right folder?\n", configFilePath)
	}

	configFile := readFile(configFilePath)

	if verboseMode {
		log.Println("Looking for templates in .resim/metrics/templates/")
	}
	templates := []bff.MetricsTemplate{}
	templateDir := path.Join(workDir, ".resim/metrics/templates")
	files, err := os.ReadDir(templateDir)
	if err != nil {
		log.Fatal(err)
	}
	if len(files) == 0 {
		if verboseMode {
			log.Printf("Found 0 template files at %s\n", templateDir)
		}
	}
	for _, file := range files {
		if file.IsDir() {
			if verboseMode {
				log.Printf("Skipping directory %s\n", file.Name())
			}
			continue
		}
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".heex") {
			if verboseMode {
				log.Printf("Skipping non .heex file %s\n", file.Name())
			}
			continue
		}
		if verboseMode {
			log.Printf("Found template %s", file.Name())
		}
		contents := readFile(path.Join(workDir, ".resim/metrics/templates/", file.Name()))
		if len(contents) == 0 {
			if verboseMode {
				log.Printf("Template %s is empty!\n", file.Name())
			}
		} else {
			templates = append(templates, bff.MetricsTemplate{Name: file.Name(), Contents: contents})
		}
	}

	_, err = bff.UpdateMetricsConfig(context.Background(), BffClient, configFile, templates)
	if err != nil {
		log.Fatalf("Failed to sync metrics config: %s", err)
	}

	if verboseMode {
		log.Println("Successfully synced metrics config, and the following templates:")
		for _, t := range templates {
			log.Printf("\t%s\n", t.Name)
		}
	}
}
