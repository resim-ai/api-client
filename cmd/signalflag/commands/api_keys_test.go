package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/resim-ai/api-client/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestAPIKeyCommandsRegistered(t *testing.T) {
	for _, name := range []string{"create", "list", "revoke", "rotate"} {
		command, _, err := rootCmd.Find([]string{"api-keys", name})
		require.NoError(t, err)
		require.Equal(t, name, command.Name())
	}
}

type apiKeyRequest struct {
	Operation string         `json:"operationName"`
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type apiKeyFixture struct {
	t                    *testing.T
	mu                   sync.Mutex
	requests             []apiKeyRequest
	cachePath            string
	stdout, stderr, logs bytes.Buffer
}

func newAPIKeyFixture(t *testing.T, respond func(http.ResponseWriter, apiKeyRequest)) *apiKeyFixture {
	t.Helper()
	fixture := &apiKeyFixture{t: t}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/graphql", r.URL.Path)
		assert.Equal(t, "Bearer human-test-token", r.Header.Get("Authorization"))
		var request apiKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		fixture.mu.Lock()
		fixture.requests = append(fixture.requests, request)
		fixture.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		respond(w, request)
	}))
	t.Cleanup(server.Close)
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("HOME", t.TempDir())
	dir, err := GetConfigDir()
	require.NoError(t, err)
	fixture.cachePath = filepath.Join(dir, auth.CredentialCacheFilename)
	tokens, err := json.Marshal(map[string]oauth2.Token{
		auth.DefaultProdInteractiveClientID: {AccessToken: "human-test-token", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fixture.cachePath, tokens, 0600))
	viper.Set(auth.KeyURL, server.URL+"/v1/")
	viper.Set(auth.KeyAuthURL, auth.ProdAuthURL)
	viper.Set(auth.KeyProdInteractiveClient, auth.DefaultProdInteractiveClientID)
	oldClient, oldBFF, oldLog := Client, BffClient, log.Writer()
	log.SetOutput(&fixture.logs)
	t.Cleanup(func() { Client, BffClient = oldClient, oldBFF; log.SetOutput(oldLog) })
	return fixture
}

func (fixture *apiKeyFixture) run(args ...string) error {
	fixture.t.Helper()
	command := newAPIKeysCommand()
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetOut(&fixture.stdout)
	command.SetErr(&fixture.stderr)
	command.SetArgs(args)
	return command.Execute()
}

func (fixture *apiKeyFixture) recorded() []apiKeyRequest {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]apiKeyRequest(nil), fixture.requests...)
}

func keyMetadata(id, name, access, status string) map[string]any {
	return map[string]any{"id": id, "name": name, "access": access, "status": status, "prefix": "sfk_public", "createdByEmail": "admin@example.test", "createdAt": "2026-10-16T00:00:00Z", "expiresAt": nil, "revokedAt": nil}
}

func TestAPIKeyCreateDefaultsAndOutput(t *testing.T) {
	const secret = "sfk_once_only_test"
	fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
		assert.Equal(t, "CreateApiKey", request.Operation)
		assert.Equal(t, map[string]any{"name": "automation", "access": "READ_ONLY", "expiresInDays": float64(90)}, request.Variables)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"createApiKey": map[string]any{"apiKey": keyMetadata("new-id", "automation", "READ_ONLY", "ACTIVE"), "key": secret}}})
	})
	require.NoError(t, fixture.run("create", "--name", "automation"))
	assert.Equal(t, 1, strings.Count(fixture.stdout.String(), secret))
	var output struct {
		ApiKey struct {
			ID string `json:"id"`
		} `json:"apiKey"`
	}
	require.NoError(t, json.Unmarshal(fixture.stdout.Bytes(), &output))
	assert.Equal(t, "new-id", output.ApiKey.ID)
	assert.NotContains(t, fixture.stderr.String(), secret)
	assert.Contains(t, fixture.stderr.String(), "cannot be shown again")
	assert.NotContains(t, fixture.logs.String(), secret)
	cached, err := os.ReadFile(fixture.cachePath)
	require.NoError(t, err)
	assert.NotContains(t, string(cached), secret)
	assert.Len(t, fixture.recorded(), 1)
}

func TestAPIKeyCreateExplicitAccessAndExpiry(t *testing.T) {
	for _, days := range []int{0, 30, 365} {
		t.Run(fmt.Sprint(days), func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
				assert.Equal(t, "READ_WRITE", request.Variables["access"])
				if days == 0 {
					assert.Equal(t, float64(0), request.Variables["expiresInDays"])
				} else {
					assert.Equal(t, float64(days), request.Variables["expiresInDays"])
				}
				fmt.Fprint(w, `{"data":{"createApiKey":{"apiKey":{"id":"new","status":"ACTIVE"},"key":"sfk_once"}}}`)
			})
			require.NoError(t, fixture.run("create", "--name", "automation", "--access", "READ_WRITE", "--expires-in-days", fmt.Sprint(days)))
		})
	}
}

func TestAPIKeyCreateValidationBeforeMutation(t *testing.T) {
	for _, args := range [][]string{
		{"create", "--name", " "},
		{"create", "--name", "automation", "--access", "ADMIN"},
		{"create", "--name", "automation", "--expires-in-days", "2"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) { t.Error("unexpected request") })
			require.Error(t, fixture.run(args...))
			assert.Empty(t, fixture.recorded())
		})
	}
}

func TestAPIKeyCreateErrorsDoNotExposeSecretOrReplay(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"duplicate", 200, `{"errors":[{"message":"name has already been taken"}]}`, "name has already been taken"},
		{"member", 200, `{"errors":[{"message":"admin required"}]}`, "admin required"},
		{"uncertain HTTP result", 502, `{"data":{"createApiKey":{"key":"sfk_lost_secret"}}}`, "HTTP 502"},
		{"truncated response", 200, `{"data":{"createApiKey":{"key":"sfk_lost_secret`, "API-key request failed"},
		{"partial error", 200, `{"data":{"createApiKey":{"key":"sfk_lost_secret"}},"errors":[{"message":"failed sfk_lost_secret"}]}`, "[redacted]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) })
			err := fixture.run("create", "--name", "automation")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Contains(t, err.Error(), "Do not automatically retry")
			assert.NotContains(t, err.Error(), "sfk_lost_secret")
			assert.Empty(t, fixture.stdout.String())
			assert.NotContains(t, fixture.stderr.String()+fixture.logs.String(), "sfk_lost_secret")
			assert.Len(t, fixture.recorded(), 1)
		})
	}
}

func TestAPIKeyListMetadataOnly(t *testing.T) {
	fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
		assert.Equal(t, "ListApiKeys", request.Operation)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"apiKeys": []any{keyMetadata("old-id", "automation", "READ_ONLY", "REVOKING")}}})
	})
	require.NoError(t, fixture.run("list"))
	var keys []map[string]any
	require.NoError(t, json.Unmarshal(fixture.stdout.Bytes(), &keys))
	require.Len(t, keys, 1)
	assert.Equal(t, "old-id", keys[0]["id"])
	assert.Equal(t, "REVOKING", keys[0]["status"])
	assert.NotContains(t, keys[0], "key")
	assert.NotContains(t, fixture.recorded()[0].Query, "createApiKey")
}

func TestAPIKeyManagementRefusesMachineModesBeforeRequests(t *testing.T) {
	for _, mode := range []string{"key", "client"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) { t.Error("unexpected request") })
			if mode == "key" {
				viper.Set(auth.KeyAPIKey, "sfk_machine")
			} else {
				viper.Set(auth.KeyClientID, "client")
				viper.Set(auth.KeyClientSecret, "secret")
			}
			for _, args := range [][]string{{"list"}, {"create", "--name", "key"}, {"revoke", "--id", "old"}, {"rotate", "--id", "old", "--new-name", "replacement"}} {
				err := fixture.run(args...)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "human sign-in")
			}
			assert.Empty(t, fixture.recorded())
		})
	}
}

func TestAPIKeyRevokeIDRetriesPending(t *testing.T) {
	fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
		assert.Equal(t, "RevokeApiKey", request.Operation)
		assert.Equal(t, "old-id", request.Variables["id"])
		status := "REVOKING"
		fmt.Fprintf(w, `{"data":{"revokeApiKey":{"id":"old-id","status":%q}}}`, status)
	})
	require.NoError(t, fixture.run("revoke", "--id", "old-id"))
	assert.Contains(t, fixture.stdout.String(), "REVOKING")
	assert.Contains(t, fixture.stderr.String(), "pending synchronization")
	assert.Contains(t, fixture.stderr.String(), "revoke --id old-id")
	require.NoError(t, fixture.run("revoke", "--id", "old-id"))
	assert.Len(t, fixture.recorded(), 2)
}

func TestAPIKeyRevokeByNameSelection(t *testing.T) {
	for _, tc := range []struct {
		name    string
		keys    []any
		wantErr string
		wantID  string
	}{
		{"unique live", []any{keyMetadata("revoked", "key", "READ_ONLY", "REVOKED"), keyMetadata("live", "key", "READ_ONLY", "ACTIVE")}, "", "live"},
		{"missing", []any{}, "no matching key", ""},
		{"ambiguous", []any{keyMetadata("a", "key", "READ_ONLY", "ACTIVE"), keyMetadata("b", "key", "READ_ONLY", "REVOKING")}, "ambiguous", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
				if request.Operation == "ListApiKeys" {
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"apiKeys": tc.keys}})
					return
				}
				assert.Equal(t, "RevokeApiKey", request.Operation)
				assert.Equal(t, tc.wantID, request.Variables["id"])
				fmt.Fprintf(w, `{"data":{"revokeApiKey":{"id":%q,"status":"REVOKED"}}}`, tc.wantID)
			})
			err := fixture.run("revoke", "--name", "key")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.Len(t, fixture.recorded(), 1)
			} else {
				require.NoError(t, err)
				assert.Len(t, fixture.recorded(), 2)
				assert.NotContains(t, fixture.stderr.String(), "pending")
			}
		})
	}
}

func TestAPIKeySelectorRequiresExactlyOne(t *testing.T) {
	for _, args := range [][]string{{"revoke"}, {"revoke", "--id", "id", "--name", "name"}, {"rotate", "--new-name", "replacement"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) { t.Error("unexpected request") })
			require.ErrorContains(t, fixture.run(args...), "exactly one")
			assert.Empty(t, fixture.recorded())
		})
	}
}

func TestAPIKeyRotateCreatesReplacementWithoutRevoking(t *testing.T) {
	for _, tc := range []struct {
		status string
		fail   bool
	}{{"ACTIVE", false}, {"ACTIVE", true}, {"EXPIRED", false}} {
		t.Run(fmt.Sprint(tc.status, tc.fail), func(t *testing.T) {
			fail := tc.fail
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
				switch request.Operation {
				case "ListApiKeys":
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"apiKeys": []any{keyMetadata("old-id", "old", "READ_WRITE", tc.status)}}})
				case "CreateApiKey":
					assert.Equal(t, map[string]any{"name": "replacement", "access": "READ_WRITE", "expiresInDays": float64(90)}, request.Variables)
					if fail {
						w.WriteHeader(502)
						fmt.Fprint(w, "lost response")
						return
					}
					fmt.Fprint(w, `{"data":{"createApiKey":{"apiKey":{"id":"new-id","status":"ACTIVE"},"key":"sfk_replacement"}}}`)
				default:
					t.Errorf("unexpected operation %s", request.Operation)
				}
			})
			err := fixture.run("rotate", "--id", "old-id", "--new-name", "replacement")
			if fail {
				require.ErrorContains(t, err, "old key was not changed")
				assert.Empty(t, fixture.stdout.String())
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, strings.Count(fixture.stdout.String(), "sfk_replacement"))
				assert.Contains(t, fixture.stderr.String(), "old key was not changed")
				assert.Contains(t, fixture.stderr.String(), "revoke --id old-id")
			}
			require.Len(t, fixture.recorded(), 2)
			assert.Equal(t, "CreateApiKey", fixture.recorded()[1].Operation)
		})
	}
}

func TestAPIKeyRotateInvalidSourceDoesNotCreate(t *testing.T) {
	for _, tc := range []struct {
		name, status, newName string
		keys                  bool
		want                  string
	}{
		{"missing", "ACTIVE", "new", false, "no matching key"},
		{"same name", "ACTIVE", "old", true, "must differ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, request apiKeyRequest) {
				assert.Equal(t, "ListApiKeys", request.Operation)
				keys := []any{}
				if tc.keys {
					keys = append(keys, keyMetadata("old-id", "old", "READ_ONLY", tc.status))
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"apiKeys": keys}})
			})
			require.ErrorContains(t, fixture.run("rotate", "--id", "old-id", "--new-name", tc.newName), tc.want)
			assert.Len(t, fixture.recorded(), 1)
		})
	}
}

func TestAPIKeySetupNeverWritesCredentialCache(t *testing.T) {
	fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) { t.Error("unexpected request") })
	const original = "invalid existing cache"
	require.NoError(t, os.WriteFile(fixture.cachePath, []byte(original), 0600))
	viper.Set(auth.KeyAPIKey, "sfk_machine")
	viper.Set(auth.KeyAuthURL, "://unused")
	command := newAPIKeysCommand()
	command.SetContext(context.Background())
	command.SetErr(&fixture.stderr)
	require.NoError(t, SetClient(command, nil))
	cached, err := os.ReadFile(fixture.cachePath)
	require.NoError(t, err)
	assert.Equal(t, original, string(cached))
	assert.Empty(t, fixture.logs.String())
	assert.Empty(t, fixture.recorded())
}

type failedAPIKeyOutput struct{}

func (failedAPIKeyOutput) Write(_ []byte) (int, error) { return 0, fmt.Errorf("output unavailable") }

func TestAPIKeyLostOutputDoesNotReplayCreation(t *testing.T) {
	fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) {
		fmt.Fprint(w, `{"data":{"createApiKey":{"apiKey":{"id":"new-id"},"key":"sfk_output_lost"}}}`)
	})
	command := newAPIKeysCommand()
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetOut(failedAPIKeyOutput{})
	command.SetErr(&fixture.stderr)
	command.SetArgs([]string{"create", "--name", "automation"})
	err := command.Execute()
	require.ErrorContains(t, err, "could not write the created key")
	assert.Contains(t, err.Error(), "Do not automatically retry")
	assert.NotContains(t, err.Error(), "sfk_output_lost")
	assert.Len(t, fixture.recorded(), 1)
}

func TestAPIKeyGlobalFlagRefusesManagementBeforeRequests(t *testing.T) {
	fixture := newAPIKeyFixture(t, func(w http.ResponseWriter, _ apiKeyRequest) { t.Error("unexpected request") })
	command := &cobra.Command{Use: "signalflag", SilenceErrors: true, SilenceUsage: true}
	command.PersistentFlags().String(auth.KeyAPIKey, "", "Organization API key")
	command.AddCommand(newAPIKeysCommand())
	command.SetOut(&fixture.stdout)
	command.SetErr(&fixture.stderr)
	command.SetArgs([]string{"--api-key", "sfk_machine", "api-keys", "list"})
	require.ErrorContains(t, command.Execute(), "human sign-in")
	assert.Empty(t, fixture.recorded())
}
