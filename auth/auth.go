package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/cli/browser"
	"github.com/golang-jwt/jwt/v5"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	audience = "https://api.resim.ai"

	ProdGovcloudURL = "https://api-gov.resim.ai/v1/"
	ProdAPIURL      = "https://api.resim.ai/v1/"
	StagingAPIURL   = "https://api.resim.io/v1/"
	ProdAuthURL     = "https://resim.us.auth0.com/"
	DevAuthURL      = "https://resim-dev.us.auth0.com/"

	CredentialCacheFilename = "cache.json"

	DefaultDevInteractiveClientID     = "Rg1F0ZOCBmVYje4UVrS3BKIh4T2nCW9y"
	DefaultDevNonInteractiveClientID  = "LLNl3xsbNLSd16gQyYsiEn3tbLDZo1gj"
	DefaultProdInteractiveClientID    = "gTp1Y0kOyQ7QzIo2lZm0auGM6FJZZVvy"
	DefaultProdNonInteractiveClientID = "0Ip56H1LLAo6Dc6IfePaNzgpUxbJGyVI"

	// Viper/flag key names used for config binding.
	KeyURL                      = "url"
	KeyAuthURL                  = "auth-url"
	KeyClientID                 = "client-id"
	KeyClientSecret             = "client-secret"
	KeyUsername                 = "username"
	KeyPassword                 = "password"
	KeyDevInteractiveClient     = "dev-interactive-client"
	KeyProdInteractiveClient    = "prod-interactive-client"
	KeyDevNonInteractiveClient  = "dev-non-interactive-client"
	KeyProdNonInteractiveClient = "prod-non-interactive-client"
)

type AuthMode string

const (
	ModeClientCredentials AuthMode = "clientcredentials"
	ModeDeviceCode        AuthMode = "devicecode"
	ModePassword          AuthMode = "password"
)

type Config struct {
	APIURL       string
	AuthURL      string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	Govcloud     bool
	CacheDir     string

	DevInteractiveClientID     string
	ProdInteractiveClientID    string
	DevNonInteractiveClientID  string
	ProdNonInteractiveClientID string

	// AuthMode optionally overrides auth mode inference.
	// If empty, the mode is inferred from which credential fields are populated.
	AuthMode AuthMode

	// DeviceCodeOutput is the writer for device code prompts. Defaults to os.Stdout.
	DeviceCodeOutput io.Writer

	// DeviceCodeBrowserOpen opens a URL in the user's browser. Defaults to browser.OpenURL.
	DeviceCodeBrowserOpen func(string) error
}

type AuthResult struct {
	Cache  *CredentialCache
	APIURL string
}

type CredentialCache struct {
	Tokens      map[string]oauth2.Token `json:"tokens"`
	TokenSource oauth2.TokenSource
	ClientID    string
}

type tokenJSON struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int32  `json:"expires_in"`
}

// DefaultConfig returns a Config pre-filled with production defaults.
func DefaultConfig() Config {
	return Config{
		APIURL:                     ProdAPIURL,
		AuthURL:                    ProdAuthURL,
		CacheDir:                   os.ExpandEnv("$HOME/.resim"),
		DevInteractiveClientID:     DefaultDevInteractiveClientID,
		ProdInteractiveClientID:    DefaultProdInteractiveClientID,
		DevNonInteractiveClientID:  DefaultDevNonInteractiveClientID,
		ProdNonInteractiveClientID: DefaultProdNonInteractiveClientID,
	}
}

// ConfigFromViper builds a Config from a viper instance. The cacheDir parameter
// specifies where credential caches are stored (e.g., os.ExpandEnv("$HOME/.resim")).
// This preserves viper.IsSet() semantics for auth mode detection, so CI environments
// with empty env vars behave correctly.
func ConfigFromViper(v *viper.Viper, cacheDir string) Config {
	cfg := Config{
		APIURL:                     v.GetString(KeyURL),
		AuthURL:                    v.GetString(KeyAuthURL),
		ClientID:                   v.GetString(KeyClientID),
		ClientSecret:               v.GetString(KeyClientSecret),
		Username:                   v.GetString(KeyUsername),
		Password:                   v.GetString(KeyPassword),
		Govcloud:                   v.GetBool("govcloud"),
		CacheDir:                   cacheDir,
		DevInteractiveClientID:     v.GetString(KeyDevInteractiveClient),
		ProdInteractiveClientID:    v.GetString(KeyProdInteractiveClient),
		DevNonInteractiveClientID:  v.GetString(KeyDevNonInteractiveClient),
		ProdNonInteractiveClientID: v.GetString(KeyProdNonInteractiveClient),
	}

	// Preserve viper.IsSet() semantics: set explicit AuthMode when env vars
	// are present (even if empty) to avoid behavior change in CI environments.
	if v.IsSet(KeyUsername) && v.IsSet(KeyPassword) {
		cfg.AuthMode = ModePassword
	}
	if v.IsSet(KeyClientID) && v.IsSet(KeyClientSecret) {
		cfg.AuthMode = ModeClientCredentials
	}

	return cfg
}

// Authenticate performs OAuth2 authentication using the provided config.
// It returns an AuthResult containing the credential cache and the resolved API URL
// (which may differ from Config.APIURL if govcloud mode is enabled).
func Authenticate(ctx context.Context, cfg Config) (*AuthResult, error) {
	var cache CredentialCache
	cache.Load(cfg.CacheDir)

	authMode, err := determineAuthMode(cfg)
	if err != nil {
		return nil, err
	}

	authURL, tokenURL, err := getAuthURL(cfg)
	if err != nil {
		return nil, err
	}

	resolvedAPIURL := cfg.APIURL

	if cfg.Govcloud {
		switch cfg.AuthURL {
		case DevAuthURL:
			if cfg.APIURL == ProdAPIURL {
				return nil, fmt.Errorf("GovCloud dev mode enabled, set --url to the deployment you wish to use")
			}
		case ProdAuthURL:
			resolvedAPIURL = ProdGovcloudURL
		}
	}

	var clientID string
	var clientSecret string
	var token oauth2.Token
	var tokenSource oauth2.TokenSource

	if authMode == ModeDeviceCode {
		switch authURL {
		case DevAuthURL:
			clientID = cfg.DevInteractiveClientID
		case ProdAuthURL:
			clientID = cfg.ProdInteractiveClientID
		default:
			return nil, fmt.Errorf("couldn't find interactive auth client ID for auth-url %q", authURL)
		}
	}

	if authMode == ModePassword {
		switch authURL {
		case DevAuthURL:
			clientID = cfg.DevNonInteractiveClientID
		case ProdAuthURL:
			clientID = cfg.ProdNonInteractiveClientID
		default:
			return nil, fmt.Errorf("couldn't find non-interactive auth client ID for auth-url %q", authURL)
		}
	}

	if authMode == ModeClientCredentials {
		clientID = cfg.ClientID
		clientSecret = cfg.ClientSecret
	}

	cache.ClientID = clientID
	token, ok := cache.Tokens[clientID]
	if !(ok && token.Valid()) {
		token, err = doAuth(ctx, authMode, authURL, tokenURL, clientID, clientSecret, cfg)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}

		permissionsPresent, err := tokenPermissionsPresent(token.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("error checking token permissions: %w", err)
		}
		if !permissionsPresent {
			time.Sleep(1 * time.Second)
			token, err = doAuth(ctx, authMode, authURL, tokenURL, clientID, clientSecret, cfg)
			if err != nil {
				return nil, fmt.Errorf("re-authentication for permissions failed: %w", err)
			}
		}
	}

	tokenSource = oauth2.ReuseTokenSource(&token, tokenSource)
	cache.TokenSource = tokenSource

	return &AuthResult{
		Cache:  &cache,
		APIURL: resolvedAPIURL,
	}, nil
}

func doAuth(ctx context.Context, mode AuthMode, authURL, tokenURL, clientID, clientSecret string, cfg Config) (oauth2.Token, error) {
	switch mode {
	case ModePassword:
		return doPasswordAuth(tokenURL, clientID, cfg.Username, cfg.Password)
	case ModeDeviceCode:
		return doDeviceCodeAuth(ctx, authURL, tokenURL, clientID, cfg.DeviceCodeOutput, cfg.DeviceCodeBrowserOpen)
	case ModeClientCredentials:
		return doClientCredentialsAuth(ctx, tokenURL, clientID, clientSecret)
	default:
		return oauth2.Token{}, fmt.Errorf("unknown auth mode: %s", mode)
	}
}

func determineAuthMode(cfg Config) (AuthMode, error) {
	if cfg.AuthMode != "" {
		return cfg.AuthMode, nil
	}

	hasPassword := cfg.Username != "" && cfg.Password != ""
	hasClientCreds := cfg.ClientID != "" && cfg.ClientSecret != ""

	if hasPassword && hasClientCreds {
		return "", fmt.Errorf("ambiguous authentication arguments provided - please provide username and password OR client ID and client secret")
	}

	if hasPassword {
		return ModePassword, nil
	}

	if hasClientCreds {
		return ModeClientCredentials, nil
	}

	return ModeDeviceCode, nil
}

func doPasswordAuth(tokenURL, clientID, username, password string) (oauth2.Token, error) {
	payloadVals := url.Values{
		"grant_type": []string{"http://auth0.com/oauth/grant-type/password-realm"},
		"realm":      []string{"cli-users"},
		"username":   []string{username},
		"password":   []string{password},
		"audience":   []string{audience},
		"client_id":  []string{clientID},
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(payloadVals.Encode()))
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("error creating password auth request: %w", err)
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("error in password auth: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("error reading password auth response: %w", err)
	}

	var tj tokenJSON
	if err := json.Unmarshal(body, &tj); err != nil {
		return oauth2.Token{}, fmt.Errorf("error parsing password auth response: %w", err)
	}

	return oauth2.Token{
		AccessToken:  tj.AccessToken,
		TokenType:    tj.TokenType,
		RefreshToken: tj.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tj.ExpiresIn) * time.Second),
	}, nil
}

func doDeviceCodeAuth(ctx context.Context, authURL, tokenURL, clientID string, output io.Writer, browserOpen func(string) error) (oauth2.Token, error) {
	if output == nil {
		output = os.Stdout
	}
	if browserOpen == nil {
		browserOpen = browser.OpenURL
	}

	deviceAuthURL, err := url.JoinPath(authURL, "/oauth/device/code")
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("error creating deviceAuthURL: %w", err)
	}

	config := &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: deviceAuthURL,
			TokenURL:      tokenURL,
		},
		Scopes: []string{
			"offline_access",
		},
	}

	response, err := config.DeviceAuth(ctx, oauth2.SetAuthURLParam("audience", audience))
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("unable to initiate device auth: %w", err)
	}

	browserOpen(response.VerificationURIComplete)
	fmt.Fprintf(output, "If your browser hasn't opened automatically, please open\n%s\n", response.VerificationURIComplete)
	fmt.Fprintf(output, "and enter code\n%s\n", response.UserCode)

	token, err := config.DeviceAccessToken(ctx, response)
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("unable to complete device auth: %w", err)
	}

	return *token, nil
}

func doClientCredentialsAuth(ctx context.Context, tokenURL, clientID, clientSecret string) (oauth2.Token, error) {
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		EndpointParams: url.Values{
			"audience": []string{audience},
		},
	}

	token, err := config.TokenSource(ctx).Token()
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("error in client credentials exchange: %w", err)
	}

	return *token, nil
}

func getAuthURL(cfg Config) (string, string, error) {
	authURL := cfg.AuthURL
	if !strings.HasSuffix(authURL, "/") {
		authURL += "/"
	}
	tokenURL, err := url.JoinPath(cfg.AuthURL, "/oauth/token")
	if err != nil {
		return "", "", fmt.Errorf("unable to create token URL: %w", err)
	}

	return authURL, tokenURL, nil
}

func tokenPermissionsPresent(tokenString string) (bool, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return false, fmt.Errorf("error parsing token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, fmt.Errorf("error getting token claims")
	}

	permissionsCount := 0
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		permissionsCount = len(permissions)
	}

	return permissionsCount > 0, nil
}

// Load reads the credential cache from disk. Returns nil if the file does not exist.
// Returns an error if the file exists but is malformed or unreadable.
func (c *CredentialCache) Load(cacheDir string) error {
	if c.Tokens == nil {
		c.Tokens = map[string]oauth2.Token{}
	}
	path := filepath.Join(cacheDir, CredentialCacheFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("error reading credential cache: %w", err)
	}

	if err := json.Unmarshal(data, &c.Tokens); err != nil {
		return fmt.Errorf("error parsing credential cache: %w", err)
	}
	return nil
}

// Save writes the credential cache to disk. Creates the cache directory if needed.
func (c *CredentialCache) Save(cacheDir string) error {
	token, err := c.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("error getting token: %w", err)
	}
	if token != nil {
		c.Tokens[c.ClientID] = *token
	}

	data, err := json.Marshal(c.Tokens)
	if err != nil {
		return fmt.Errorf("error marshaling credential cache: %w", err)
	}

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("error creating cache directory: %w", err)
	}

	path := filepath.Join(cacheDir, CredentialCacheFilename)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("error saving credential cache: %w", err)
	}
	return nil
}

// NewAPIClient creates an API client authenticated with the given credential cache.
func NewAPIClient(ctx context.Context, cache CredentialCache, apiURL string) (*api.ClientWithResponses, error) {
	oauthClient := oauth2.NewClient(ctx, cache.TokenSource)
	return api.NewClientWithResponses(apiURL, api.WithHTTPClient(oauthClient))
}

// NewBFFClient creates a GraphQL BFF client authenticated with the given credential cache.
func NewBFFClient(ctx context.Context, cache CredentialCache, apiURL string) (graphql.Client, error) {
	oauthClient := oauth2.NewClient(ctx, cache.TokenSource)
	graphqlURL, err := inferGraphqlAPI(apiURL)
	if err != nil {
		return nil, err
	}
	return graphql.NewClient(graphqlURL, oauthClient), nil
}

func inferGraphqlAPI(rerunAPIurl string) (string, error) {
	u, err := url.Parse(rerunAPIurl)
	if err != nil {
		return "", fmt.Errorf("error parsing API url: %w", err)
	}

	u.Path = "/graphql"
	if strings.Contains(u.Host, "localhost") {
		u.Host = "localhost:4000"
	} else {
		u.Host = strings.Replace(u.Host, "api.", "bff.", 1)
	}
	return u.String(), nil
}
