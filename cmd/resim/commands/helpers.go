package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/bff"
)

func SyncMetricsConfig(projectID uuid.UUID, branchName string, verbose bool) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	configFilePath := path.Join(workDir, ".resim/metrics/config.yml")
	if verbose {
		fmt.Printf("Looking for metrics config at %s\n", configFilePath)
	}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return fmt.Errorf("failed to find ReSim metrics config at %s", configFilePath)
	}
	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	configB64 := base64.StdEncoding.EncodeToString(configData)

	templateDir := path.Join(workDir, ".resim/metrics/templates")
	if verbose {
		fmt.Printf("Looking for templates in %s\n", templateDir)
	}
	files, err := os.ReadDir(templateDir)
	if err != nil {
		return fmt.Errorf("failed to read templates dir: %w", err)
	}
	templates := []bff.MetricsTemplate{}
	for _, f := range files {
		if f.IsDir() {
			if verbose {
				fmt.Printf("Skipping directory %s\n", f.Name())
			}
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Name()), ".liquid") {
			if verbose {
				fmt.Printf("Skipping non .liquid file %s\n", f.Name())
			}
			continue
		}
		if verbose {
			fmt.Printf("Found template %s\n", f.Name())
		}
		fullPath := path.Join(templateDir, f.Name())
		contents, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", fullPath, err)
		}
		templates = append(templates, bff.MetricsTemplate{
			Name:     f.Name(),
			Contents: base64.StdEncoding.EncodeToString(contents),
		})
	}

	_, err = bff.UpdateMetricsConfig(context.Background(), BffClient, projectID.String(), branchName, configB64, templates)
	if err != nil {
		return fmt.Errorf("failed to sync metrics config: %w", err)
	}

	if verbose {
		fmt.Println("Successfully synced metrics config with templates:")
		for _, t := range templates {
			fmt.Printf("\t%s\n", t.Name)
		}
	}
	return nil
}
