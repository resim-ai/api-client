package bff

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockClient struct {
	request *http.Request
}

func (mc *mockClient) Do(request *http.Request) (*http.Response, error) {
	mc.request = request
	return &http.Response{StatusCode: 200}, nil
}

func TestUpdateMetricsConfig(t *testing.T) {
	client := mockClient{}

	bffClient := NewClient("http://localhost:4000/graphql", &client)

	response, err := bffClient.UpdateMetricsConfig("metricsFile", []MetricsTemplate{
		{Name: "hi.json.heex", Contents: "hi, it's me"},
		{Name: "problem.json.heex", Contents: "I'm the problem it's me"},
	})

	assert.NoError(t, err)
	assert.Equal(t, 200, response.StatusCode)

	assert.Equal(t, "http://localhost:4000/graphql", client.request.URL.String())
	assert.Equal(t, http.MethodPost, client.request.Method)
	requestBody, err := io.ReadAll(client.request.Body)
	assert.NoError(t, err)

	type requestGraphqlQuery struct {
		Query     string `json:"query"`
		Variables struct {
			Config        string            `json:"config"`
			TemplateFiles []MetricsTemplate `json:"templateFiles"`
		} `json:"variables"`
	}
	var request requestGraphqlQuery
	err = json.Unmarshal(requestBody, &request)
	assert.NoError(t, err)
	assert.Contains(t, request.Query, "mutation UpdateMetricsConfig")
	assert.Equal(t, "metricsFile", request.Variables.Config)

	assert.Equal(t, "hi.json.heex", request.Variables.TemplateFiles[0].Name)
	assert.Equal(t, "hi, it's me", request.Variables.TemplateFiles[0].Contents)
	assert.Equal(t, "problem.json.heex", request.Variables.TemplateFiles[1].Name)
	assert.Equal(t, "I'm the problem it's me", request.Variables.TemplateFiles[1].Contents)
}
