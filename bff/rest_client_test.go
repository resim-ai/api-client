package bff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type recorded struct {
	method string
	path   string
	body   map[string]any
}

func restServer(t *testing.T, status int, response string) (*httptest.Server, *recorded) {
	t.Helper()
	rec := &recorded{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			require.NoError(t, json.Unmarshal(raw, &rec.body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server, rec
}

func TestRESTClientCreateDashboard(t *testing.T) {
	server, rec := restServer(t, http.StatusCreated,
		`{"id":"d1","name":"Nightly","isStale":false,"lastRanAt":null}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	resp, err := CreateDashboard(context.Background(), client, "p1", "b1", "Nightly", 7, "nightly")

	require.NoError(t, err)
	assert.Equal(t, "d1", resp.CreateDashboard.Id)
	assert.Equal(t, "Nightly", resp.CreateDashboard.Name)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api/v1/dashboards", rec.path)
	assert.Equal(t, map[string]any{
		"projectId": "p1", "branchId": "b1", "name": "Nightly", "dayRange": float64(7), "metricsSet": "nightly",
	}, rec.body)
}

func TestRESTClientGetDashboard(t *testing.T) {
	server, rec := restServer(t, http.StatusOK,
		`{"id":"d1","name":"Nightly","isStale":true,"lastRanAt":"2026-10-22T10:00:00Z"}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	resp, err := GetDashboard(context.Background(), client, "d1")

	require.NoError(t, err)
	assert.True(t, resp.Dashboard.IsStale)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api/v1/dashboards/d1", rec.path)
}

func TestRESTClientConfigFileSchema(t *testing.T) {
	server, rec := restServer(t, http.StatusOK, `{"$schema":"http://json-schema.org/draft-07/schema#"}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	resp, err := GetConfigFileSchema(context.Background(), client)

	require.NoError(t, err)
	assert.JSONEq(t, `{"$schema":"http://json-schema.org/draft-07/schema#"}`, resp.ConfigFileSchema)
	assert.Equal(t, "/api/v1/metrics-config/schema", rec.path)
}

func TestRESTClientUpdateMetricsConfig(t *testing.T) {
	server, rec := restServer(t, http.StatusOK, `{"synced":true}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	_, err := UpdateMetricsConfig(context.Background(), client, "p1", "Y29uZmln",
		[]MetricsTemplate{{Name: "line.j2", Contents: "e30="}}, "main", true)

	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/api/v1/metrics-config", rec.path)
	assert.Equal(t, true, rec.body["allowTopicRemoval"])
	assert.Equal(t, []any{map[string]any{"name": "line.j2", "contents": "e30="}}, rec.body["templateFiles"])
}

func TestRESTClientUnusedMetricsAndTopicRemoval(t *testing.T) {
	server, rec := restServer(t, http.StatusOK, `{"unusedMetrics":["speed_p99"]}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	unused, err := FindUnusedMetrics(context.Background(), client, "Y29uZmln")
	require.NoError(t, err)
	assert.Equal(t, []string{"speed_p99"}, unused.FindUnusedMetrics)
	assert.Equal(t, "/api/v1/metrics-config/unused-metrics", rec.path)

	server, rec = restServer(t, http.StatusOK,
		`{"topics":[{"topicName":"speed","chartCount":2,"dashboards":[{"id":"d1","name":"Nightly"}]}]}`)
	client = NewRESTClient(server.URL+"/api/v1", server.Client())

	preview, err := PreviewTopicRemoval(context.Background(), client, "b1", "Y29uZmln")
	require.NoError(t, err)
	require.Len(t, preview.PreviewTopicRemoval, 1)
	assert.Equal(t, "speed", preview.PreviewTopicRemoval[0].TopicName)
	assert.Equal(t, 2, preview.PreviewTopicRemoval[0].ChartCount)
	assert.Equal(t, "/api/v1/metrics-config/topic-removal-preview", rec.path)
}

func TestRESTClientValidationErrorsBecomeGraphQLErrors(t *testing.T) {
	server, _ := restServer(t, http.StatusBadRequest,
		`{"error":{"code":"invalid_argument","message":"a; b","details":["a","b"]}}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	_, err := ValidateMetricsConfig(context.Background(), client, "b1", "Y29uZmln", nil)

	var gqlErrs gqlerror.List
	require.True(t, errors.As(err, &gqlErrs))
	require.Len(t, gqlErrs, 2)
	assert.Equal(t, "a", gqlErrs[0].Message)
	assert.Equal(t, "b", gqlErrs[1].Message)
}

func TestRESTClientErrorCarriesCode(t *testing.T) {
	server, _ := restServer(t, http.StatusForbidden,
		`{"error":{"code":"API_KEY_READ_ONLY","message":"This API key is read-only."}}`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	_, err := ValidateMetricsSet(context.Background(), client, "b1", "nightly")

	var gqlErrs gqlerror.List
	require.True(t, errors.As(err, &gqlErrs))
	require.Len(t, gqlErrs, 1)
	assert.Equal(t, "This API key is read-only.", gqlErrs[0].Message)
	assert.Equal(t, "API_KEY_READ_ONLY", gqlErrs[0].Extensions["code"])
}

func TestRESTClientNonJSONError(t *testing.T) {
	server, _ := restServer(t, http.StatusBadGateway, `<html>bad gateway</html>`)
	client := NewRESTClient(server.URL+"/api/v1", server.Client())

	_, err := GetDashboard(context.Background(), client, "d1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}
