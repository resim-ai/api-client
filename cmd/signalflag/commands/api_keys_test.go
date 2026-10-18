package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/resim-ai/api-client/auth"
	"github.com/resim-ai/api-client/bff"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const retryMessage = "Could not finish the change on every service. Nothing was changed; try again."

func apiKeyFields(id, name string, access bff.ApiKeyAccess, status bff.ApiKeyStatus, expiresAt string) bff.ApiKeyFields {
	return bff.ApiKeyFields{
		Id:             id,
		Name:           name,
		Prefix:         "sf_" + id,
		Access:         access,
		CreatedByEmail: "admin@example.com",
		CreatedAt:      "2026-10-01T12:00:00Z",
		ExpiresAt:      expiresAt,
		Status:         status,
	}
}

func isOp(name string) func(*graphql.Request) bool {
	return func(req *graphql.Request) bool { return req.OpName == name }
}

func requestVars(req *graphql.Request) map[string]any {
	vars := map[string]any{}
	data, _ := json.Marshal(req.Variables)
	json.Unmarshal(data, &vars)
	return vars
}

func withListApiKeys(keys ...bff.ApiKeyFields) func(mock.Arguments) {
	return func(args mock.Arguments) {
		data := args.Get(2).(*graphql.Response).Data.(*bff.ListApiKeysResponse)
		for _, k := range keys {
			data.ApiKeys = append(data.ApiKeys, bff.ListApiKeysApiKeysApiKey{ApiKeyFields: k})
		}
	}
}

func bffErrors(messages ...string) error {
	var list gqlerror.List
	for _, m := range messages {
		list = append(list, &gqlerror.Error{Message: m, Path: ast.Path{ast.PathName("createApiKey")}})
	}
	return list
}

// --- auth guard ---

func TestApiKeysAuthGuard(t *testing.T) {
	cases := []struct {
		name    string
		set     map[string]string
		allowed bool
	}{
		{name: "device code", set: map[string]string{}, allowed: true},
		{name: "password", set: map[string]string{auth.KeyUsername: "u", auth.KeyPassword: "p"}, allowed: true},
		{name: "client credentials", set: map[string]string{auth.KeyClientID: "id", auth.KeyClientSecret: "secret"}},
		{name: "api key", set: map[string]string{auth.KeyAPIKey: "sf_live_abc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			for k, val := range tc.set {
				v.Set(k, val)
			}
			err := requirePersonSignIn(v)
			if tc.allowed {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, "Managing API keys needs a person signed in as an org admin.", err.Error())
		})
	}
}

func TestApiKeysPreRunRefusesAPIKeyBeforeAnyRequest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	defer viper.Reset()
	viper.Set(auth.KeyAPIKey, "sf_live_abc")

	origClient, origBff := Client, BffClient
	defer func() { Client, BffClient = origClient, origBff }()
	mockBff := new(mockGraphQLClient)
	BffClient = mockBff
	Client = nil

	err := apiKeysCmd.PersistentPreRunE(listApiKeysCmd, nil)
	require.Error(t, err)
	assert.Equal(t, "Managing API keys needs a person signed in as an org admin.", err.Error())
	assert.Same(t, mockBff, BffClient)
	assert.Nil(t, Client)
	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.Anything, mock.Anything)
}

func TestAPIKeyFromEnvironment(t *testing.T) {
	t.Setenv(EnvPrefix+"_API_KEY", "sf_live_env")
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	cfg := auth.ConfigFromViper(v, t.TempDir())
	assert.Equal(t, "sf_live_env", cfg.APIKey)
	assert.Equal(t, auth.ModeAPIKey, cfg.AuthMode)
}

func TestRootHasAPIKeyFlag(t *testing.T) {
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup(auth.KeyAPIKey))
}

// --- create ---

func TestCreateApiKey_PrintsKeyOnce(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(func(req *graphql.Request) bool {
		vars := requestVars(req)
		return req.OpName == "CreateApiKey" && vars["name"] == "ci-runner" && vars["access"] == "READ_ONLY" && vars["expiresInDays"] == float64(90)
	}), mock.Anything).Run(func(args mock.Arguments) {
		data := args.Get(2).(*graphql.Response).Data.(*bff.CreateApiKeyResponse)
		data.CreateApiKey.Key = "sf_live_secretvalue"
		data.CreateApiKey.ApiKey.ApiKeyFields = apiKeyFields("k1", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusActive, "2027-01-14T12:00:00Z")
	}).Return(nil).Once()

	var out bytes.Buffer
	err := actualCreateApiKey(context.Background(), mockBff, &out, "ci-runner", "read-only", 90)
	require.NoError(t, err)

	text := out.String()
	assert.Equal(t, 1, strings.Count(text, "sf_live_secretvalue"))
	assert.Contains(t, text, "won't be shown again")
	assert.Contains(t, text, "ci-runner")
	mockBff.AssertExpectations(t)
}

func TestCreateApiKey_NoExpiryAndReadWrite(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(func(req *graphql.Request) bool {
		vars := requestVars(req)
		return req.OpName == "CreateApiKey" && vars["access"] == "READ_WRITE" && vars["expiresInDays"] == float64(0)
	}), mock.Anything).Run(func(args mock.Arguments) {
		data := args.Get(2).(*graphql.Response).Data.(*bff.CreateApiKeyResponse)
		data.CreateApiKey.Key = "sf_live_other"
		data.CreateApiKey.ApiKey.ApiKeyFields = apiKeyFields("k2", "deploy", bff.ApiKeyAccessReadWrite, bff.ApiKeyStatusActive, "")
	}).Return(nil).Once()

	var out bytes.Buffer
	require.NoError(t, actualCreateApiKey(context.Background(), mockBff, &out, "deploy", "read-write", 0))
	assert.Contains(t, out.String(), "never expires")
	mockBff.AssertExpectations(t)
}

func TestCreateApiKey_RejectsBadInputWithoutRequest(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	var out bytes.Buffer

	err := actualCreateApiKey(context.Background(), mockBff, &out, "x", "admin", 90)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--access")

	err = actualCreateApiKey(context.Background(), mockBff, &out, "x", "read-only", 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--expires-in-days")

	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateApiKey_BFFErrorsPassThroughUnchanged(t *testing.T) {
	messages := []string{
		"An active API key named ci-runner already exists.",
		"Only org admins can manage API keys.",
		retryMessage,
	}
	for _, msg := range messages {
		t.Run(msg, func(t *testing.T) {
			mockBff := new(mockGraphQLClient)
			mockBff.On("MakeRequest", matchContext, mock.MatchedBy(isOp("CreateApiKey")), mock.Anything).
				Return(bffErrors(msg)).Once()

			var out bytes.Buffer
			err := actualCreateApiKey(context.Background(), mockBff, &out, "ci-runner", "read-only", 90)
			require.Error(t, err)
			assert.Equal(t, msg, err.Error())
			assert.Empty(t, out.String())
		})
	}
}

func TestCreateApiKey_HTTPErrorMessagePassesThrough(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.Anything, mock.Anything).Return(&graphql.HTTPError{
		StatusCode: 403,
		Response:   graphql.Response{Errors: gqlerror.List{{Message: "Only org admins can manage API keys."}}},
	}).Once()

	err := actualCreateApiKey(context.Background(), mockBff, &bytes.Buffer{}, "ci-runner", "read-only", 90)
	require.Error(t, err)
	assert.Equal(t, "Only org admins can manage API keys.", err.Error())
}

// --- list ---

func TestListApiKeys(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(isOp("ListApiKeys")), mock.Anything).
		Run(withListApiKeys(
			apiKeyFields("k1", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusActive, "2027-01-14T12:00:00Z"),
			apiKeyFields("k2", "deploy", bff.ApiKeyAccessReadWrite, bff.ApiKeyStatusRevoked, ""),
			apiKeyFields("k3", "laptop", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusExpired, "2026-09-30T12:00:00Z"),
		)).Return(nil).Once()

	var out bytes.Buffer
	require.NoError(t, actualListApiKeys(context.Background(), mockBff, &out))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 4)
	for _, col := range []string{"NAME", "PREFIX", "ACCESS", "CREATED BY", "EXPIRES", "STATUS"} {
		assert.Contains(t, lines[0], col)
	}
	assert.Contains(t, lines[1], "ci-runner")
	assert.Contains(t, lines[1], "sf_k1")
	assert.Contains(t, lines[1], "read-only")
	assert.Contains(t, lines[1], "admin@example.com")
	assert.Contains(t, lines[1], "2027-01-14")
	assert.True(t, strings.HasSuffix(lines[1], "active"))

	assert.Contains(t, lines[2], "deploy")
	assert.Contains(t, lines[2], "never")
	assert.True(t, strings.HasSuffix(lines[2], "revoked"))

	assert.Contains(t, lines[3], "2026-09-30")
	assert.True(t, strings.HasSuffix(lines[3], "expired"))
	mockBff.AssertExpectations(t)
}

func TestListApiKeys_Empty(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(isOp("ListApiKeys")), mock.Anything).
		Return(nil).Once()

	var out bytes.Buffer
	require.NoError(t, actualListApiKeys(context.Background(), mockBff, &out))
	assert.Equal(t, "no API keys\n", out.String())
}

// --- revoke ---

func TestRevokeApiKey_ByID(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(func(req *graphql.Request) bool {
		return req.OpName == "RevokeApiKey" && requestVars(req)["id"] == "k1"
	}), mock.Anything).Run(func(args mock.Arguments) {
		data := args.Get(2).(*graphql.Response).Data.(*bff.RevokeApiKeyResponse)
		data.RevokeApiKey.ApiKeyFields = apiKeyFields("k1", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusRevoked, "")
	}).Return(nil).Once()

	var out bytes.Buffer
	require.NoError(t, actualRevokeApiKey(context.Background(), mockBff, &out, "", "k1"))
	assert.Contains(t, out.String(), `Revoked API key "ci-runner"`)
	mockBff.AssertExpectations(t)
}

func TestRevokeApiKey_ByNameFindsActiveKey(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(isOp("ListApiKeys")), mock.Anything).
		Run(withListApiKeys(
			apiKeyFields("old", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusRevoked, ""),
			apiKeyFields("other", "deploy", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusActive, ""),
			apiKeyFields("current", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusActive, ""),
		)).Return(nil).Once()
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(func(req *graphql.Request) bool {
		return req.OpName == "RevokeApiKey" && requestVars(req)["id"] == "current"
	}), mock.Anything).Run(func(args mock.Arguments) {
		data := args.Get(2).(*graphql.Response).Data.(*bff.RevokeApiKeyResponse)
		data.RevokeApiKey.ApiKeyFields = apiKeyFields("current", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusRevoked, "")
	}).Return(nil).Once()

	var out bytes.Buffer
	require.NoError(t, actualRevokeApiKey(context.Background(), mockBff, &out, "ci-runner", ""))
	mockBff.AssertExpectations(t)
}

func TestRevokeApiKey_ByNameOnlyMatchesActiveKey(t *testing.T) {
	mockBff := new(mockGraphQLClient)
	mockBff.On("MakeRequest", matchContext, mock.MatchedBy(isOp("ListApiKeys")), mock.Anything).
		Run(withListApiKeys(
			apiKeyFields("old", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusRevoked, ""),
			apiKeyFields("lapsed", "ci-runner", bff.ApiKeyAccessReadOnly, bff.ApiKeyStatusExpired, ""),
		)).Return(nil).Once()

	err := actualRevokeApiKey(context.Background(), mockBff, &bytes.Buffer{}, "ci-runner", "")
	require.Error(t, err)
	assert.Equal(t, `no active API key named "ci-runner"`, err.Error())
	mockBff.AssertExpectations(t)
}

func TestRevokeApiKey_BFFErrorsPassThroughUnchanged(t *testing.T) {
	for _, msg := range []string{"Only org admins can manage API keys.", retryMessage} {
		t.Run(msg, func(t *testing.T) {
			mockBff := new(mockGraphQLClient)
			mockBff.On("MakeRequest", matchContext, mock.MatchedBy(isOp("RevokeApiKey")), mock.Anything).
				Return(bffErrors(msg)).Once()

			var out bytes.Buffer
			err := actualRevokeApiKey(context.Background(), mockBff, &out, "", "k1")
			require.Error(t, err)
			assert.Equal(t, msg, err.Error())
			assert.Empty(t, out.String())
		})
	}
}

func TestApiKeysCmdFlags(t *testing.T) {
	for _, name := range []string{apiKeyNameKey, apiKeyAccessKey, apiKeyExpiresInDaysKey} {
		assert.NotNil(t, createApiKeyCmd.Flags().Lookup(name), "--%s should exist on create", name)
	}
	expires := createApiKeyCmd.Flags().Lookup(apiKeyExpiresInDaysKey)
	assert.Equal(t, "90", expires.DefValue)
	for _, name := range []string{apiKeyNameKey, apiKeyIDKey} {
		assert.NotNil(t, revokeApiKeyCmd.Flags().Lookup(name), "--%s should exist on revoke", name)
	}
}
