package bff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client manages queries and mutations with the BFF GraphQL API
type Client struct {
	// apiKey string
	apiURL string
	client *http.Client
}

func NewBffClient(apiURL string, client *http.Client) *Client {

	return &Client{
		apiURL: apiURL,
		client: client,
	}
}

type MetricsTemplate struct {
	Name     string
	Contents string
}

func (client *Client) UpdateMetricsConfig(metricsFile string, templates []MetricsTemplate) (*http.Response, error) {
	query := `
		mutation UpdateMetricsConfig($config: String!, $templateFiles: [MetricsTemplate!]!) {
			updateMetricsConfig(config: $config, templateFiles: $templateFiles)
		}
	`

	variables := map[string]any{
		"config":        metricsFile,
		"templateFiles": templates,
	}

	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return &http.Response{}, fmt.Errorf("error marshaling request body: %w", err)
	}

	request, err := http.NewRequest("POST", client.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return &http.Response{}, fmt.Errorf("error creating request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	// request.Header.Set("Authorization", "Bearer "+client.apiKey)

	return client.client.Do(request)
}
