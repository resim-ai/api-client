package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSchemaFile(t *testing.T) {
	source := "schema { query: RootQueryType }\ntype RootQueryType { hello: String }\n"
	path := filepath.Join(t.TempDir(), "schema.graphql")
	require.NoError(t, os.WriteFile(path, []byte(source), 0600))
	schema, err := loadSchema(context.Background(), path, "://unused")
	require.NoError(t, err)
	assert.Equal(t, source, schema)
}

func TestLoadSchemaFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.graphql")
	_, err := loadSchema(context.Background(), path, "://unused")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read GraphQL schema")
	require.NoError(t, os.WriteFile(path, []byte("not a schema"), 0600))
	_, err = loadSchema(context.Background(), path, "://unused")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid GraphQL schema")
}

func TestLoadSchemaEndpointAddsRootDeclaration(t *testing.T) {
	types := []any{}
	for _, name := range []string{"RootQueryType", "RootMutationType", "RootSubscriptionType"} {
		types = append(types, map[string]any{"kind": "OBJECT", "name": name, "fields": []any{map[string]any{"name": "hello", "type": map[string]any{"kind": "SCALAR", "name": "String"}}}})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"__schema": map[string]any{"types": types}}})
	}))
	defer server.Close()
	schema, err := loadSchema(context.Background(), "", server.URL)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(schema, "schema {"))
	assert.Contains(t, schema, "query: RootQueryType")
	assert.Contains(t, schema, "type RootMutationType")
}
