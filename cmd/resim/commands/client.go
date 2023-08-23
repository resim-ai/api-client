package commands

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	urlKey          = "url"
	authURLKey      = "auth-url"
	clientIDKey     = "client-id"
	clientSecretKey = "client-secret"
)

const CredentialCacheFilename = "cache.json"

type CredentialCache struct {
	Tokens      map[string]oauth2.Token `json:"tokens"`
	TokenSource oauth2.TokenSource
	ClientID    string
}

func init() {
	rootCmd.PersistentFlags().String(urlKey, "", "The URL of the API.")
	viper.SetDefault(urlKey, "https://api.resim.ai/v1/")
	rootCmd.PersistentFlags().String(authURLKey, "", "The URL of the authentication endpoint.")
	viper.SetDefault(authURLKey, "https://resim.us.auth0.com/")
	rootCmd.PersistentFlags().String(clientIDKey, "", "Authentication credentials client ID")
	rootCmd.PersistentFlags().String(clientSecretKey, "", "Authentication credentials client secret")
}

func GetClient(ctx context.Context) (*api.ClientWithResponses, *CredentialCache, error) {
	var cache CredentialCache
	err := cache.loadCredentialCache()
	if err != nil {
		log.Println("Initializing credential cache")
	}

	clientID := viper.GetString(clientIDKey)
	if clientID == "" {
		return nil, nil, errors.New("client-id must be specified")
	}
	cache.ClientID = clientID
	clientSecret := viper.GetString(clientSecretKey)
	if clientSecret == "" {
		return nil, nil, errors.New("client-secret must be specified")
	}
	tokenURL, err := url.JoinPath(viper.GetString(authURLKey), "/oauth/token")
	if err != nil {
		log.Fatal("unable to create token URL: ", err)
	}
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		EndpointParams: url.Values{
			"audience": []string{"https://api.resim.ai"},
		},
	}

	if token, ok := cache.Tokens[clientID]; ok {
		cache.TokenSource = oauth2.ReuseTokenSource(&token, config.TokenSource(ctx))
	} else {
		cache.TokenSource = config.TokenSource(ctx)
	}

	oauthClient := oauth2.NewClient(ctx, cache.TokenSource)

	client, err := api.NewClientWithResponses(viper.GetString(urlKey), api.WithHTTPClient(oauthClient))
	if err != nil {
		return nil, nil, err
	}
	return client, &cache, nil
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
	c.Tokens[c.ClientID] = *token

	data, err := json.Marshal(c.Tokens)
	if err != nil {
		log.Println("error marshaling credential cache:", err)
		return
	}

	homedir, _ := os.UserHomeDir()
	expectedDir := strings.ReplaceAll(ConfigPath, "$HOME", homedir)
	// Check first if the directory exists, and if it does not, create it:
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		err := os.Mkdir(expectedDir, 0700)
		if err != nil {
			log.Println("error creating directory:", err)
			return
		}
	}
	path := filepath.Join(expectedDir, CredentialCacheFilename)
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		log.Println("error saving credential cache:", err)
	}
}

// Validate Response fails the command if the response is nil, or the
// status code is not what we expect.
func ValidateResponse(expectedStatusCode int, message string, response *http.Response) {
	if response == nil {
		log.Fatal(message, ": ", "no response")
	}
	if response.StatusCode != expectedStatusCode {
		message, readErr := io.ReadAll((response.Body))
		if readErr != nil {
			log.Println("error reading response: ", readErr)
		}
		log.Fatal(message, ": expected status code: ", expectedStatusCode,
			" received: ", response.StatusCode, " status: ", response.Status, " message: ", message)
	}
}
