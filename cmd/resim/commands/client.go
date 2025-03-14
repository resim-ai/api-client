package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	audience                    = "https://api.resim.ai"
	urlKey                      = "url"
	usernameKey                 = "username"
	passwordKey                 = "password"
	authURLKey                  = "auth-url"
	clientIDKey                 = "client-id"
	clientSecretKey             = "client-secret"
	devInteractiveClientKey     = "dev-interactive-client"
	prodInteractiveClientKey    = "prod-interactive-client"
	devNonInteractiveClientKey  = "dev-non-interactive-client"
	prodNonInteractiveClientKey = "prod-non-interactive-client"
	verboseKey                  = "verbose"
	prodGovcloudURL             = "https://api-gov.resim.ai/v1/"
	prodAPIURL                  = "https://api.resim.ai/v1/"
	stagingAPIURL               = "https://api.resim.io/v1/"
	prodAuthURL                 = "https://resim.us.auth0.com/"
	devAuthURL                  = "https://resim-dev.us.auth0.com/"
)

const CredentialCacheFilename = "cache.json"

type AuthMode string

const (
	ClientCredentials AuthMode = "clientcredentials"
	DeviceCode        AuthMode = "devicecode"
	Password          AuthMode = "password"
)

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

func init() {
	rootCmd.PersistentFlags().String(urlKey, "", "The URL of the API.")
	viper.SetDefault(urlKey, prodAPIURL)
	rootCmd.PersistentFlags().String(authURLKey, "", "The URL of the authentication endpoint.")
	viper.SetDefault(authURLKey, prodAuthURL)
	rootCmd.PersistentFlags().String(clientIDKey, "", "Authentication credentials client ID")
	rootCmd.PersistentFlags().String(clientSecretKey, "", "Authentication credentials client secret")
	rootCmd.PersistentFlags().String(devInteractiveClientKey, "", "Client ID for dev interactive login")
	viper.SetDefault(devInteractiveClientKey, "Rg1F0ZOCBmVYje4UVrS3BKIh4T2nCW9y")
	rootCmd.PersistentFlags().String(devNonInteractiveClientKey, "", "Client ID for dev non-interactive login")
	viper.SetDefault(devNonInteractiveClientKey, "LLNl3xsbNLSd16gQyYsiEn3tbLDZo1gj")
	rootCmd.PersistentFlags().String(prodInteractiveClientKey, "", "Client ID for prod interactive login")
	viper.SetDefault(prodInteractiveClientKey, "gTp1Y0kOyQ7QzIo2lZm0auGM6FJZZVvy")
	rootCmd.PersistentFlags().String(prodNonInteractiveClientKey, "", "Client ID for prod non-interactive login")
	viper.SetDefault(prodNonInteractiveClientKey, "0Ip56H1LLAo6Dc6IfePaNzgpUxbJGyVI")
	rootCmd.PersistentFlags().String(usernameKey, "", "username for non-interactive login")
	rootCmd.PersistentFlags().String(passwordKey, "", "password for non-interactive login")
	rootCmd.PersistentFlags().Bool(verboseKey, false, "Verbose mode")
}

func Authenticate(ctx context.Context) (*CredentialCache, error) {
	var cache CredentialCache
	err := cache.loadCredentialCache()
	if err != nil {
		log.Println("Initializing credential cache")
	}

	authMode := determineAuthMode()

	authURL, tokenURL := getAuthURL()

	// If the user has run `resim govcloud enable` or if RESIM_GOVCLOUD=true, govcloud is enabled
	// If govcloud is enabled and we're authenticating against prod,
	// automatically set the url to prod govcloud
	if viper.GetBool("govcloud") {
		switch viper.GetString(authURLKey) {
		case devAuthURL:
			if viper.GetString(urlKey) == prodAPIURL {
				log.Fatal("GovCloud dev mode enabled, set --url to the deployment you wish to use")
			}
		case prodAuthURL:
			viper.Set(urlKey, prodGovcloudURL)
		}
	}

	var clientID string
	var clientSecret string
	var token oauth2.Token
	var tokenSource oauth2.TokenSource

	if authMode == DeviceCode {
		switch authURL {
		case devAuthURL:
			clientID = viper.GetString(devInteractiveClientKey)
		case prodAuthURL:
			clientID = viper.GetString(prodInteractiveClientKey)
		default:
			log.Fatal("couldn't find interactive auth client ID for auth-url")
		}
	}

	if authMode == Password {
		switch authURL {
		case devAuthURL:
			clientID = viper.GetString(devNonInteractiveClientKey)
		case prodAuthURL:
			clientID = viper.GetString(prodNonInteractiveClientKey)
		default:
			log.Fatal("couldn't find non-interactive auth client ID for auth-url")
		}
	}

	if authMode == ClientCredentials {
		clientID = viper.GetString(clientIDKey)
		clientSecret = viper.GetString(clientSecretKey)
	}

	cache.ClientID = clientID
	token, ok := cache.Tokens[clientID]
	if !(ok && token.Valid()) {
		switch authMode {
		case Password:
			token = doPasswordAuth(tokenURL, clientID)
		case DeviceCode:
			token = doDeviceCodeAuth(ctx, authURL, tokenURL, clientID)
		case ClientCredentials:
			token = doClientCredentialsAuth(ctx, tokenURL, clientID, clientSecret)
		}

		// on first auth the permissions are not present - check for them and re-auth if they aren't there
		if !tokenPermissionsPresent(token.AccessToken) {
			time.Sleep(1 * time.Second)
			switch authMode {
			case Password:
				token = doPasswordAuth(tokenURL, clientID)
			case DeviceCode:
				token = doDeviceCodeAuth(ctx, authURL, tokenURL, clientID)
			case ClientCredentials:
				token = doClientCredentialsAuth(ctx, tokenURL, clientID, clientSecret)
			}
		}
	}

	tokenSource = oauth2.ReuseTokenSource(&token, tokenSource)
	cache.TokenSource = tokenSource

	return &cache, nil
}

func GetClient(ctx context.Context, cache CredentialCache) (*api.ClientWithResponses, error) {
	oauthClient := oauth2.NewClient(ctx, cache.TokenSource)

	client, err := api.NewClientWithResponses(viper.GetString(urlKey), api.WithHTTPClient(oauthClient))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func GetBffClient(ctx context.Context, cache CredentialCache) graphql.Client {
	oauthClient := oauth2.NewClient(ctx, cache.TokenSource)
	return graphql.NewClient(inferGraphqlAPI(viper.GetString(urlKey)), oauthClient)
}

// Infer the URL of the BFF's GraphQL, instead of making users
// awkwardly specify it like `resim --url api.resim.ai/v1 --bff-url bff.resim.ai/graphql projects list`,
// which is bound to cause mistakes.
func inferGraphqlAPI(rerunAPIurl string) string {
	url, err := url.Parse(rerunAPIurl)
	if err != nil {
		log.Fatal("error parsing API url: ", err)
	}

	url.Path = "/graphql"
	if strings.Contains(url.Host, "localhost") {
		url.Host = "localhost:4000"
	} else {
		url.Host = strings.Replace(url.Host, "api.", "bff.", 1)
	}
	return url.String()
}

func (c *CredentialCache) loadCredentialCache() error {
	homedir, _ := os.UserHomeDir()
	path := strings.ReplaceAll(filepath.Join(ConfigPath, CredentialCacheFilename), "$HOME", homedir)
	data, err := os.ReadFile(path)
	if err != nil {
		c.Tokens = map[string]oauth2.Token{}
		return err
	}

	return json.Unmarshal(data, &c.Tokens)
}

func (c *CredentialCache) SaveCredentialCache() {
	token, err := c.TokenSource.Token()
	if err != nil {
		log.Println("error getting token:", err)
	}
	if token != nil {
		c.Tokens[c.ClientID] = *token
	}

	data, err := json.Marshal(c.Tokens)
	if err != nil {
		log.Println("error marshaling credential cache:", err)
		return
	}

	expectedDir, err := GetConfigDir()
	if err != nil {
		return
	}

	path := filepath.Join(expectedDir, CredentialCacheFilename)
	err = os.WriteFile(path, data, 0600)
	if err != nil {
		log.Println("error saving credential cache:", err)
	}
}

// Validate Response fails the command if the response is nil, or the
// status code is not what we expect.
func ValidateResponse(expectedStatusCode int, message string, response *http.Response, body []byte) {
	if response == nil {
		log.Fatal(message, ": ", "no response")
	}
	if response.StatusCode != expectedStatusCode {
		// Unmarshal response as JSON:
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			log.Fatal(message, ": expected status code: ", expectedStatusCode,
				" received: ", response.StatusCode, " status: ", response.Status)
		}
		// Pretty print the response map
		prettyJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatal(message, ": expected status code: ", expectedStatusCode,
				" received: ", response.StatusCode, " status: ", response.Status)
		}
		// Handle the unmarshalled data
		log.Fatal(message, ": expected status code: ", expectedStatusCode,
			" received: ", response.StatusCode, " status: ", response.Status, "\n message:\n", string(prettyJSON))
	}

}

func determineAuthMode() AuthMode {
	if viper.IsSet(usernameKey) && viper.IsSet(passwordKey) && viper.IsSet(clientIDKey) && viper.IsSet(clientSecretKey) {
		log.Fatal("ambiguous authentication arguments provided - please provide username and password OR client ID and client secret.")
	}

	if viper.IsSet(usernameKey) && viper.IsSet(passwordKey) {
		return Password
	}

	if viper.IsSet(clientIDKey) && viper.IsSet(clientSecretKey) {
		return ClientCredentials
	}

	return DeviceCode
}

func doPasswordAuth(tokenURL string, clientID string) oauth2.Token {
	var token oauth2.Token

	payloadVals := url.Values{
		"grant_type": []string{"http://auth0.com/oauth/grant-type/password-realm"},
		"realm":      []string{"cli-users"},
		"username":   []string{viper.GetString(usernameKey)},
		"password":   []string{viper.GetString(passwordKey)},
		"audience":   []string{audience},
		"client_id":  []string{clientID},
	}

	req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(payloadVals.Encode()))

	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("error in password auth: ", err)
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var tj tokenJSON
	err = json.Unmarshal(body, &tj)
	if err != nil {
		log.Fatal(err)
	}
	token = oauth2.Token{
		AccessToken:  tj.AccessToken,
		TokenType:    tj.TokenType,
		RefreshToken: tj.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tj.ExpiresIn) * time.Second),
	}

	return token
}

func doDeviceCodeAuth(ctx context.Context, authURL string, tokenURL string, clientID string) oauth2.Token {
	var token *oauth2.Token

	deviceAuthURL, err := url.JoinPath(authURL, "/oauth/device/code")
	if err != nil {
		log.Fatal("error creating deviceAuthURL", err)
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
		log.Fatal("unable to initiate device auth: ", err)
	}

	browser.OpenURL(response.VerificationURIComplete)
	fmt.Printf("If your browser hasn't opened automatically, please open\n%s\n", response.VerificationURIComplete)
	fmt.Printf("and enter code\n%s\n", response.UserCode)
	token, err = config.DeviceAccessToken(ctx, response)
	if err != nil {
		log.Fatal("unable to complete device auth: ", err)
	}

	return *token
}

func doClientCredentialsAuth(ctx context.Context, tokenURL string, clientID string, clientSecret string) oauth2.Token {
	var token *oauth2.Token

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
		log.Fatal("error in client credentials exchange", err)
	}

	return *token
}

func getAuthURL() (string, string) {
	authURL := viper.GetString(authURLKey)
	if !strings.HasSuffix(authURL, "/") {
		authURL += "/"
	}
	tokenURL, err := url.JoinPath(viper.GetString(authURLKey), "/oauth/token")
	if err != nil {
		log.Fatal("unable to create token URL: ", err)
	}

	return authURL, tokenURL
}

func tokenPermissionsPresent(tokenString string) bool {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		log.Fatal(err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Fatal("error getting token claims")
	}

	permissionsCount := 0

	if permissions, ok := claims["permissions"].([]interface{}); ok {
		permissionsCount = len(permissions)
	}

	return permissionsCount > 0
}
