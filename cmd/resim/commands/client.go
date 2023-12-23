package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cli/browser"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	urlKey                   = "url"
	authURLKey               = "auth-url"
	clientIDKey              = "client-id"
	clientSecretKey          = "client-secret"
	devInteractiveClientKey  = "dev-interactive-client"
	prodInteractiveClientKey = "prod-interactive-client"
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
	rootCmd.PersistentFlags().String(devInteractiveClientKey, "", "Client ID for dev interactive login")
	viper.SetDefault(devInteractiveClientKey, "Rg1F0ZOCBmVYje4UVrS3BKIh4T2nCW9y")
	rootCmd.PersistentFlags().String(prodInteractiveClientKey, "", "Client ID for prod interactive login")
	viper.SetDefault(prodInteractiveClientKey, "gTp1Y0kOyQ7QzIo2lZm0auGM6FJZZVvy")
}

func GetClient(ctx context.Context) (*api.ClientWithResponses, *CredentialCache, error) {
	var cache CredentialCache
	err := cache.loadCredentialCache()
	if err != nil {
		log.Println("Initializing credential cache")
	}

	tokenURL, err := url.JoinPath(viper.GetString(authURLKey), "/oauth/token")
	if err != nil {
		log.Fatal("unable to create token URL: ", err)
	}
	authURL, err := url.JoinPath(viper.GetString(authURLKey), "/oauth/device/code")
	if err != nil {
		log.Fatal("unable to create authURL: ", err)
	}

	var tokenSource oauth2.TokenSource

	if viper.GetString(clientIDKey) == "" {
		var clientID string
		switch viper.GetString(authURLKey) {
		case "https://resim-dev.us.auth0.com/":
			clientID = viper.GetString(devInteractiveClientKey)
		case "https://resim.us.auth0.com/":
			clientID = viper.GetString(prodInteractiveClientKey)
		default:
			log.Fatal("couldn't find CLI client ID for auth-url")
		}

		config := &oauth2.Config{
			ClientID: clientID,
			Endpoint: oauth2.Endpoint{
				DeviceAuthURL: authURL,
				TokenURL:      tokenURL,
			},
			Scopes: []string{
				"offline_access",
			},
		}

		cache.ClientID = clientID
		token, ok := cache.Tokens[clientID]
		if ok && token.Valid() {
			cache.TokenSource = config.TokenSource(ctx, &token)
		} else {
			response, err := config.DeviceAuth(ctx, oauth2.SetAuthURLParam("audience", "https://api.resim.ai"))
			if err != nil {
				log.Fatal("unable to initiate device auth: ", err)
			}

			browser.OpenURL(response.VerificationURIComplete)
			fmt.Printf("If your browser hasn't opened automatically, please open\n%s\n", response.VerificationURIComplete)
			fmt.Printf("and enter code\n%s\n", response.UserCode)
			token, err := config.DeviceAccessToken(ctx, response)
			if err != nil {
				log.Fatal("unable to complete device auth: ", err)
			}

			tokenSource = config.TokenSource(ctx, token)
			cache.TokenSource = tokenSource
		}
	} else {
		clientID := viper.GetString(clientIDKey)
		if clientID == "" {
			return nil, nil, errors.New("client-id must be specified")
		}

		clientSecret := viper.GetString(clientSecretKey)
		if clientSecret == "" {
			return nil, nil, errors.New("client-secret must be specified for non-interactive login")
		}

		config := clientcredentials.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			TokenURL:     tokenURL,
			EndpointParams: url.Values{
				"audience": []string{"https://api.resim.ai"},
			},
		}
		tokenSource = config.TokenSource(ctx)

		cache.ClientID = clientID
		if token, ok := cache.Tokens[clientID]; ok {
			cache.TokenSource = oauth2.ReuseTokenSource(&token, tokenSource)
		} else {
			cache.TokenSource = tokenSource
		}
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
	if token != nil {
		c.Tokens[c.ClientID] = *token
	}

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
