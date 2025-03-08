package commands

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/cobra"
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
	file, _ := os.ReadFile(path)
	// TODO: handle error
	return base64.StdEncoding.EncodeToString(file)
}

func syncMetrics(cmd *cobra.Command, args []string) {
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %s", err)
	}

	configFilePath := path.Join(workDir, ".resim/metrics/config.yml")
	log.Println("Looking for metrics config at .resim/metrics/config.yml")

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Fatalf("failed to find ReSim metrics config at %s\nAre you in the right folder?\n", configFilePath)
	}

	configFile := readFile(configFilePath)

	log.Println("Looking for templates in .resim/metrics/templates/")
	templates := []bff.MetricsTemplate{}
	templateDir := path.Join(workDir, ".resim/metrics/templates")
	files, err := os.ReadDir(templateDir)
	if err != nil {
		log.Fatal(err)
	}
	if len(files) == 0 {
		log.Printf("Found 0 template files at %s\n", templateDir)
	}
	for _, file := range files {
		if file.IsDir() {
			log.Printf("Skipping directory %s\n", file.Name())
			continue
		}
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".heex") {
			log.Printf("Skipping non .heex file %s\n", file.Name())
			continue
		}
		log.Printf("Found template %s", file.Name())
		contents := readFile(path.Join(workDir, ".resim/metrics/templates/", file.Name()))
		if len(contents) == 0 {
			log.Printf("Template %s is empty!\n", file.Name())
		} else {
			templates = append(templates, bff.MetricsTemplate{Name: file.Name(), Contents: contents})
		}
	}

	response, err := BffClient.UpdateMetricsConfig(configFile, templates)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("failed to read body: %s", err)
	}

	if response.StatusCode != http.StatusOK {
		log.Fatalf("Got non-200 response: %d", response.StatusCode)
	}

	var graphqlResponse GraphQLResponse
	if err := json.Unmarshal(body, &graphqlResponse); err != nil {
		log.Fatalf("Failed to read response body: %s", err)
	}

	if len(graphqlResponse.Errors) > 0 {
		errors := []string{}
		for _, e := range graphqlResponse.Errors {
			errors = append(errors, e.Message)
		}
		log.Fatalf("Request failed:\n%s", strings.Join(errors, ", "))
	} else {
		log.Println("Successfully synced metrics config, and the following templates:")
		for _, t := range templates {
			log.Printf("\t%s\n", t.Name)
		}
	}

	/*
		- Check if workdir is a git repo, err if not
		- Check if git repo contains .resim/metrics/metrics.y[a]ml file, err if not
		- Base64 encode the metrics.yml file
		- Base64 encode all .resim/metrics/templates/*.heex files
		- bffClient.updateMetricsConfig???
		- print success/failure
	*/
}

type GraphQLResponse struct {
	Data   any `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}
