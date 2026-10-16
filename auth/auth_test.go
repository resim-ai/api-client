package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// --- DefaultConfig ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, ProdAPIURL, cfg.APIURL)
	assert.Equal(t, ProdAuthURL, cfg.AuthURL)
	assert.Equal(t, DefaultDevInteractiveClientID, cfg.DevInteractiveClientID)
	assert.Equal(t, DefaultProdInteractiveClientID, cfg.ProdInteractiveClientID)
	assert.Equal(t, DefaultDevNonInteractiveClientID, cfg.DevNonInteractiveClientID)
	assert.Equal(t, DefaultProdNonInteractiveClientID, cfg.ProdNonInteractiveClientID)
	assert.NotEmpty(t, cfg.CacheDir)
}

func TestZeroValueConfig(t *testing.T) {
	cfg := Config{}
	assert.Empty(t, cfg.APIURL)
	assert.Empty(t, cfg.AuthURL)
	assert.Empty(t, cfg.ClientID)
	assert.False(t, cfg.Govcloud)
	assert.Empty(t, cfg.AuthMode)
}

// --- ConfigFromViper ---

func TestConfigFromViper_MapsAllFields(t *testing.T) {
	v := viper.New()
	v.Set(KeyURL, "https://custom-api.example.com/v1/")
	v.Set(KeyAuthURL, "https://custom-auth.example.com/")
	v.Set(KeyClientID, "my-id")
	v.Set(KeyClientSecret, "my-secret")
	v.Set(KeyUsername, "user")
	v.Set(KeyPassword, "pass")
	v.Set("govcloud", true)
	v.Set(KeyDevInteractiveClient, "dev-int")
	v.Set(KeyProdInteractiveClient, "prod-int")
	v.Set(KeyDevNonInteractiveClient, "dev-non")
	v.Set(KeyProdNonInteractiveClient, "prod-non")

	cfg := ConfigFromViper(v, "/tmp/test-cache")

	assert.Equal(t, "https://custom-api.example.com/v1/", cfg.APIURL)
	assert.Equal(t, "https://custom-auth.example.com/", cfg.AuthURL)
	assert.Equal(t, "my-id", cfg.ClientID)
	assert.Equal(t, "my-secret", cfg.ClientSecret)
	assert.Equal(t, "user", cfg.Username)
	assert.Equal(t, "pass", cfg.Password)
	assert.True(t, cfg.Govcloud)
	assert.Equal(t, "/tmp/test-cache", cfg.CacheDir)
	assert.Equal(t, "dev-int", cfg.DevInteractiveClientID)
	assert.Equal(t, "prod-int", cfg.ProdInteractiveClientID)
	assert.Equal(t, "dev-non", cfg.DevNonInteractiveClientID)
	assert.Equal(t, "prod-non", cfg.ProdNonInteractiveClientID)
}

func TestConfigFromViper_SetsClientCredentialsMode(t *testing.T) {
	v := viper.New()
	v.Set(KeyClientID, "id")
	v.Set(KeyClientSecret, "secret")

	cfg := ConfigFromViper(v, "/tmp/cache")
	assert.Equal(t, ModeClientCredentials, cfg.AuthMode)
}

func TestConfigFromViper_SetsPasswordMode(t *testing.T) {
	v := viper.New()
	v.Set(KeyUsername, "user")
	v.Set(KeyPassword, "pass")

	cfg := ConfigFromViper(v, "/tmp/cache")
	assert.Equal(t, ModePassword, cfg.AuthMode)
}

func TestConfigFromViper_NoCredsLeavesAuthModeEmpty(t *testing.T) {
	v := viper.New()
	v.Set(KeyURL, ProdAPIURL)

	cfg := ConfigFromViper(v, "/tmp/cache")
	assert.Empty(t, cfg.AuthMode)
}

// --- Config.Mode ---

func TestDetermineAuthMode_ClientCredentials(t *testing.T) {
	mode, err := (Config{
		ClientID:     "id",
		ClientSecret: "secret",
	}).Mode()
	require.NoError(t, err)
	assert.Equal(t, ModeClientCredentials, mode)
}

func TestDetermineAuthMode_Password(t *testing.T) {
	mode, err := (Config{
		Username: "user",
		Password: "pass",
	}).Mode()
	require.NoError(t, err)
	assert.Equal(t, ModePassword, mode)
}

func TestDetermineAuthMode_DeviceCode(t *testing.T) {
	mode, err := (Config{}).Mode()
	require.NoError(t, err)
	assert.Equal(t, ModeDeviceCode, mode)
}

func TestDetermineAuthMode_Ambiguous(t *testing.T) {
	_, err := (Config{
		Username:     "user",
		Password:     "pass",
		ClientID:     "id",
		ClientSecret: "secret",
	}).Mode()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestDetermineAuthMode_ExplicitOverride(t *testing.T) {
	mode, err := (Config{
		AuthMode: ModePassword,
	}).Mode()
	require.NoError(t, err)
	assert.Equal(t, ModePassword, mode)
}

func TestDetermineAuthMode_OnlyUsernameNoPassword(t *testing.T) {
	mode, err := (Config{
		Username: "user",
	}).Mode()
	require.NoError(t, err)
	assert.Equal(t, ModeDeviceCode, mode)
}

// --- getAuthURL ---

func TestGetAuthURL_AddsTrailingSlash(t *testing.T) {
	authURL, tokenURL, err := getAuthURL(Config{AuthURL: "https://auth.example.com"})
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/", authURL)
	assert.Equal(t, "https://auth.example.com/oauth/token", tokenURL)
}

func TestGetAuthURL_PreservesTrailingSlash(t *testing.T) {
	authURL, tokenURL, err := getAuthURL(Config{AuthURL: "https://auth.example.com/"})
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/", authURL)
	assert.Equal(t, "https://auth.example.com/oauth/token", tokenURL)
}

// --- tokenPermissionsPresent ---

func makeTestJWT(t *testing.T, permissions []string) string {
	t.Helper()
	claims := jwt.MapClaims{}
	if permissions != nil {
		perms := make([]interface{}, len(permissions))
		for i, p := range permissions {
			perms[i] = p
		}
		claims["permissions"] = perms
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return signed
}

func TestTokenPermissionsPresent_WithPermissions(t *testing.T) {
	tokenStr := makeTestJWT(t, []string{"read:tests", "write:tests"})
	present, err := tokenPermissionsPresent(tokenStr)
	require.NoError(t, err)
	assert.True(t, present)
}

func TestTokenPermissionsPresent_NoPermissions(t *testing.T) {
	tokenStr := makeTestJWT(t, nil)
	present, err := tokenPermissionsPresent(tokenStr)
	require.NoError(t, err)
	assert.False(t, present)
}

func TestTokenPermissionsPresent_EmptyPermissions(t *testing.T) {
	tokenStr := makeTestJWT(t, []string{})
	present, err := tokenPermissionsPresent(tokenStr)
	require.NoError(t, err)
	assert.False(t, present)
}

func TestTokenPermissionsPresent_InvalidToken(t *testing.T) {
	_, err := tokenPermissionsPresent("not-a-jwt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing token")
}

// --- inferGraphqlAPI ---

func TestInferGraphqlAPI_ProdURL(t *testing.T) {
	result, err := inferGraphqlAPI("https://api.resim.ai/v1/")
	require.NoError(t, err)
	assert.Equal(t, "https://bff.resim.ai/graphql", result)
}

func TestInferGraphqlAPI_Localhost(t *testing.T) {
	result, err := inferGraphqlAPI("http://localhost:8080/v1/")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4000/graphql", result)
}

func TestInferGraphqlAPI_StagingURL(t *testing.T) {
	result, err := inferGraphqlAPI("https://api.resim.io/v1/")
	require.NoError(t, err)
	assert.Equal(t, "https://bff.resim.io/graphql", result)
}

func TestInferGraphqlAPI_InvalidURL(t *testing.T) {
	_, err := inferGraphqlAPI("://bad-url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing API url")
}

// --- CredentialCache Load/Save ---

func TestCacheLoad_NonExistentFile(t *testing.T) {
	cache := CredentialCache{}
	err := cache.Load(t.TempDir())
	assert.NoError(t, err)
	assert.NotNil(t, cache.Tokens)
	assert.Empty(t, cache.Tokens)
}

func TestCacheLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	tokens := map[string]oauth2.Token{
		"test-client": {
			AccessToken: "test-token",
			Expiry:      time.Now().Add(1 * time.Hour),
		},
	}
	data, err := json.Marshal(tokens)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, CredentialCacheFilename), data, 0600))

	cache := CredentialCache{}
	err = cache.Load(dir)
	require.NoError(t, err)
	assert.Contains(t, cache.Tokens, "test-client")
	assert.Equal(t, "test-token", cache.Tokens["test-client"].AccessToken)
}

func TestCacheLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, CredentialCacheFilename), []byte("{bad json"), 0600))

	cache := CredentialCache{}
	err := cache.Load(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing credential cache")
}

func TestCacheSave_CreatesDirectoryAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "cache")

	token := oauth2.Token{
		AccessToken: "saved-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	cache := CredentialCache{
		Tokens:      map[string]oauth2.Token{"my-client": token},
		TokenSource: oauth2.StaticTokenSource(&token),
		ClientID:    "my-client",
	}

	err := cache.Save(dir)
	require.NoError(t, err)

	// Verify file was created
	data, err := os.ReadFile(filepath.Join(dir, CredentialCacheFilename))
	require.NoError(t, err)

	var loaded map[string]oauth2.Token
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Contains(t, loaded, "my-client")
}

func TestCacheSave_ExistingDirectory(t *testing.T) {
	dir := t.TempDir()

	token := oauth2.Token{
		AccessToken: "saved-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	cache := CredentialCache{
		Tokens:      map[string]oauth2.Token{"my-client": token},
		TokenSource: oauth2.StaticTokenSource(&token),
		ClientID:    "my-client",
	}

	err := cache.Save(dir)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, CredentialCacheFilename))
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestCacheLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()

	token := oauth2.Token{
		AccessToken:  "roundtrip-token",
		TokenType:    "Bearer",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(1 * time.Hour).Truncate(time.Second),
	}
	original := CredentialCache{
		Tokens:      map[string]oauth2.Token{"client-a": token},
		TokenSource: oauth2.StaticTokenSource(&token),
		ClientID:    "client-a",
	}

	require.NoError(t, original.Save(dir))

	loaded := CredentialCache{}
	require.NoError(t, loaded.Load(dir))

	assert.Contains(t, loaded.Tokens, "client-a")
	assert.Equal(t, "roundtrip-token", loaded.Tokens["client-a"].AccessToken)
	assert.Equal(t, "Bearer", loaded.Tokens["client-a"].TokenType)
}

// --- Govcloud URL resolution ---

func TestGovcloud_ProdAuthURL_ResolvesToGovcloudURL(t *testing.T) {
	// We test this through the Authenticate function with a cached valid token
	// to avoid hitting real auth endpoints.
	dir := t.TempDir()
	token := oauth2.Token{
		AccessToken: makeTestJWT(t, []string{"read:all"}),
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	tokens := map[string]oauth2.Token{
		DefaultProdInteractiveClientID: token,
	}
	data, _ := json.Marshal(tokens)
	os.WriteFile(filepath.Join(dir, CredentialCacheFilename), data, 0600)

	cfg := DefaultConfig()
	cfg.CacheDir = dir
	cfg.Govcloud = true

	result, err := Authenticate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, ProdGovcloudURL, result.APIURL)
}

func TestGovcloud_DevAuthURL_ProdAPIURL_ReturnsError(t *testing.T) {
	cfg := Config{
		APIURL:   ProdAPIURL,
		AuthURL:  DevAuthURL,
		Govcloud: true,
	}

	_, err := Authenticate(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GovCloud dev mode enabled")
}

func TestGovcloud_Disabled_PreservesAPIURL(t *testing.T) {
	dir := t.TempDir()
	token := oauth2.Token{
		AccessToken: makeTestJWT(t, []string{"read:all"}),
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	tokens := map[string]oauth2.Token{
		DefaultProdInteractiveClientID: token,
	}
	data, _ := json.Marshal(tokens)
	os.WriteFile(filepath.Join(dir, CredentialCacheFilename), data, 0600)

	cfg := DefaultConfig()
	cfg.CacheDir = dir
	cfg.Govcloud = false

	result, err := Authenticate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, ProdAPIURL, result.APIURL)
}

// --- Authenticate with cached token ---

func TestAuthenticate_UsesCachedValidToken(t *testing.T) {
	dir := t.TempDir()
	token := oauth2.Token{
		AccessToken: makeTestJWT(t, []string{"read:all"}),
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	tokens := map[string]oauth2.Token{
		"test-client-id": token,
	}
	data, _ := json.Marshal(tokens)
	os.WriteFile(filepath.Join(dir, CredentialCacheFilename), data, 0600)

	cfg := Config{
		APIURL:       ProdAPIURL,
		AuthURL:      ProdAuthURL,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		CacheDir:     dir,
	}

	result, err := Authenticate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, ProdAPIURL, result.APIURL)
	assert.NotNil(t, result.Cache)
	assert.Equal(t, "test-client-id", result.Cache.ClientID)
}

// --- Authenticate with client credentials against test server ---

func TestAuthenticate_ClientCredentials_Success(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": makeTestJWT(t, []string{"read:tests"}),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	cfg := Config{
		APIURL:       ProdAPIURL,
		AuthURL:      tokenServer.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		CacheDir:     t.TempDir(),
	}

	result, err := Authenticate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, ProdAPIURL, result.APIURL)
	assert.NotNil(t, result.Cache.TokenSource)
}

// --- doPasswordAuth against test server ---

func TestDoPasswordAuth_Success(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("content-type"))
		r.ParseForm()
		assert.Equal(t, "testuser", r.FormValue("username"))
		assert.Equal(t, "testpass", r.FormValue("password"))
		assert.Equal(t, audience, r.FormValue("audience"))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token": "%s", "token_type": "Bearer", "expires_in": 3600}`,
			makeTestJWT(t, []string{"read:tests"}))
	}))
	defer tokenServer.Close()

	token, err := doPasswordAuth(tokenServer.URL, "test-client", "testuser", "testpass")
	require.NoError(t, err)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.NotEmpty(t, token.AccessToken)
	assert.False(t, token.Expiry.IsZero())
}

func TestDoPasswordAuth_ServerError(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer tokenServer.Close()

	_, err := doPasswordAuth(tokenServer.URL, "test-client", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing password auth response")
}

// --- NewAPIClient ---

func TestNewAPIClient_ReturnsClient(t *testing.T) {
	token := oauth2.Token{AccessToken: "test", Expiry: time.Now().Add(1 * time.Hour)}
	cache := CredentialCache{
		TokenSource: oauth2.StaticTokenSource(&token),
	}

	client, err := NewAPIClient(context.Background(), cache, "https://api.resim.ai/v1/")
	require.NoError(t, err)
	assert.NotNil(t, client)
}

// --- NewBFFClient ---

func TestNewBFFClient_ReturnsClient(t *testing.T) {
	token := oauth2.Token{AccessToken: "test", Expiry: time.Now().Add(1 * time.Hour)}
	cache := CredentialCache{
		TokenSource: oauth2.StaticTokenSource(&token),
	}

	client, err := NewBFFClient(context.Background(), cache, "https://api.resim.ai/v1/")
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewBFFClient_InvalidURL(t *testing.T) {
	token := oauth2.Token{AccessToken: "test", Expiry: time.Now().Add(1 * time.Hour)}
	cache := CredentialCache{
		TokenSource: oauth2.StaticTokenSource(&token),
	}

	_, err := NewBFFClient(context.Background(), cache, "://bad-url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing API url")
}

func TestAuthenticate_APIKeyBypassesOAuthAndCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, CredentialCacheFilename)
	original := []byte("not a credential cache")
	require.NoError(t, os.WriteFile(path, original, 0600))
	v := viper.New()
	v.Set("api-key", "  sfk_test_only  ")
	v.Set(KeyURL, StagingAPIURL)
	v.Set(KeyAuthURL, "://invalid-auth-url")
	cfg := ConfigFromViper(v, dir)
	result, err := Authenticate(context.Background(), cfg)
	require.NoError(t, err)
	token, err := result.Cache.TokenSource.Token()
	require.NoError(t, err)
	assert.Equal(t, "sfk_test_only", token.AccessToken)
	require.NoError(t, result.Cache.Save(dir))
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, actual)
}

func TestAuthenticate_APIKeyRejectsOtherCredentials(t *testing.T) {
	for _, key := range []string{KeyClientID, KeyClientSecret, KeyUsername, KeyPassword} {
		t.Run(key, func(t *testing.T) {
			v := viper.New()
			v.Set("api-key", "sfk_test_only")
			v.Set(key, "other")
			_, err := Authenticate(context.Background(), ConfigFromViper(v, t.TempDir()))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ambiguous")
			assert.NotContains(t, err.Error(), "sfk_test_only")
		})
	}
}

type apiKeyTransport struct {
	t    *testing.T
	urls []string
}

func (transport *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport.urls = append(transport.urls, req.URL.String())
	assert.Equal(transport.t, "Bearer sfk_transport_test", req.Header.Get("Authorization"))
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":{},"projects":[]}`))}, nil
}

func TestAuthenticate_APIKeyBothClients(t *testing.T) {
	transport := &apiKeyTransport{t: t}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: transport})
	cfg := DefaultConfig()
	cfg.APIKey = "  sfk_transport_test  "
	cfg.APIURL = "https://api.example.test/v1/"
	cfg.AuthURL = "://unused"
	cfg.CacheDir = filepath.Join(t.TempDir(), "unused")
	result, err := Authenticate(ctx, cfg)
	require.NoError(t, err)
	client, err := NewAPIClient(ctx, *result.Cache, result.APIURL)
	require.NoError(t, err)
	_, err = client.ListProjectsWithResponse(ctx, nil)
	require.NoError(t, err)
	bffClient, err := NewBFFClient(ctx, *result.Cache, result.APIURL)
	require.NoError(t, err)
	err = bffClient.MakeRequest(ctx, &graphql.Request{Query: "query { __typename }"}, &graphql.Response{})
	require.NoError(t, err)
	assert.Equal(t, []string{"https://api.example.test/v1/projects", "https://bff.example.test/graphql"}, transport.urls)
	require.NoError(t, result.Cache.Save(cfg.CacheDir))
	assert.NoDirExists(t, cfg.CacheDir)
}

func TestAPIKeyConfigAndGovcloud(t *testing.T) {
	t.Setenv("SIGNALFLAG_API_KEY", "sfk_environment_test")
	v := viper.New()
	v.SetEnvPrefix("SIGNALFLAG")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	assert.Equal(t, "sfk_environment_test", ConfigFromViper(v, "").APIKey)
	for _, tc := range []struct {
		name, authURL, apiURL, want string
		fails                       bool
	}{
		{"production", ProdAuthURL, ProdAPIURL, ProdGovcloudURL, false},
		{"development explicit", DevAuthURL, StagingAPIURL, StagingAPIURL, false},
		{"development missing target", DevAuthURL, ProdAPIURL, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Authenticate(context.Background(), Config{APIKey: "sfk_test", AuthURL: tc.authURL, APIURL: tc.apiURL, Govcloud: true})
			if tc.fails {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.APIURL)
		})
	}
}

func TestAPIKeyRejectsEmptyAndExplicitOtherMode(t *testing.T) {
	for _, cfg := range []Config{
		{APIKey: "   "},
		{AuthMode: ModeAPIKey},
		{APIKey: "sfk_test", AuthMode: ModePassword},
		{APIKey: "sfk_test", AuthMode: ModeClientCredentials},
	} {
		_, err := Authenticate(context.Background(), cfg)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "sfk_test")
	}
}
