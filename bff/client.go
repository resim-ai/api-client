package bff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type APIClient interface {
	Do(request *http.Request) (*http.Response, error)
}

// Client manages queries and mutations with the BFF GraphQL API
type Client struct {
	apiURL string
	client APIClient
}

func NewClient(apiURL string, client APIClient) *Client {
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

	return client.makeRequest(query, variables)
}

func (client *Client) makeRequest(query string, variables map[string]any) (*http.Response, error) {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return &http.Response{}, fmt.Errorf("error marshaling request body: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, client.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return &http.Response{}, fmt.Errorf("error creating request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	return client.client.Do(request)
}
