package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// The BFF accepts organization API keys only on its REST API, so with a key
// the generated operations are sent there instead of to GraphQL.
type restRoute struct {
	method string
	path   func(variables map[string]any) string
	// The field the operation's result is under in the GraphQL response.
	field string
	// Turns the REST response body into that field's value.
	result func(body []byte) (any, error)
}

func fixed(path string) func(map[string]any) string {
	return func(map[string]any) string { return path }
}

func asIs(body []byte) (any, error) { return json.RawMessage(body), nil }

func constant(value any) func([]byte) (any, error) {
	return func([]byte) (any, error) { return value, nil }
}

func member(name string) func([]byte) (any, error) {
	return func(body []byte) (any, error) {
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(body, &decoded); err != nil {
			return nil, err
		}
		return decoded[name], nil
	}
}

var restRoutes = map[string]restRoute{
	"GetConfigFileSchema": {
		method: http.MethodGet, path: fixed("/metrics-config/schema"), field: "configFileSchema",
		result: func(body []byte) (any, error) { return string(body), nil },
	},
	"ValidateMetricsConfig": {
		method: http.MethodPost, path: fixed("/metrics-config/validate"), field: "validateMetricsConfig",
		result: constant(true),
	},
	"ValidateMetricsSet": {
		method: http.MethodPost, path: fixed("/metrics-config/validate-metrics-set"), field: "validateMetricsSet",
		result: constant(true),
	},
	"FindUnusedMetrics": {
		method: http.MethodPost, path: fixed("/metrics-config/unused-metrics"), field: "findUnusedMetrics",
		result: member("unusedMetrics"),
	},
	"PreviewTopicRemoval": {
		method: http.MethodPost, path: fixed("/metrics-config/topic-removal-preview"), field: "previewTopicRemoval",
		result: member("topics"),
	},
	"UpdateMetricsConfig": {
		method: http.MethodPut, path: fixed("/metrics-config"), field: "updateMetricsConfig",
		result: constant("Success"),
	},
	"CreateDashboard": {
		method: http.MethodPost, path: fixed("/dashboards"), field: "createDashboard", result: asIs,
	},
	"GetDashboard": {
		method: http.MethodGet, field: "dashboard", result: asIs,
		path: func(variables map[string]any) string {
			id, _ := variables["id"].(string)
			return "/dashboards/" + url.PathEscape(id)
		},
	},
	"CreateDebugDashboard": {
		method: http.MethodPost, path: fixed("/debug-dashboards"), field: "createDebugDashboard", result: asIs,
	},
}

type restClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRESTClient returns a client for the BFF's REST API at baseURL (ending in
// /api/v1) that serves the generated operations in this package.
func NewRESTClient(baseURL string, httpClient *http.Client) graphql.Client {
	return &restClient{baseURL: strings.TrimSuffix(baseURL, "/"), httpClient: httpClient}
}

type restError struct {
	Error struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Details []string `json:"details"`
	} `json:"error"`
}

func (c *restClient) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	route, ok := restRoutes[req.OpName]
	if !ok {
		return fmt.Errorf("%s is not available when signed in with an API key", req.OpName)
	}

	variables := map[string]any{}
	var payload []byte
	if req.Variables != nil {
		encoded, err := json.Marshal(req.Variables)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(encoded, &variables); err != nil {
			return err
		}
		if route.method != http.MethodGet {
			payload = encoded
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, route.method, c.baseURL+route.path(variables), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 300 {
		return errorFromResponse(httpResp.StatusCode, body)
	}

	value, err := route.result(body)
	if err != nil {
		return fmt.Errorf("reading %s response: %w", req.OpName, err)
	}
	data, err := json.Marshal(map[string]any{route.field: value})
	if err != nil {
		return err
	}
	return json.Unmarshal(data, resp.Data)
}

func errorFromResponse(status int, body []byte) error {
	var decoded restError
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Error.Message == "" {
		return fmt.Errorf("returned error %d: %s", status, strings.TrimSpace(string(body)))
	}

	extensions := map[string]any{"code": decoded.Error.Code}
	messages := decoded.Error.Details
	if len(messages) == 0 {
		messages = []string{decoded.Error.Message}
	}

	errs := make(gqlerror.List, 0, len(messages))
	for _, message := range messages {
		errs = append(errs, &gqlerror.Error{Message: message, Extensions: extensions})
	}
	return errs
}
