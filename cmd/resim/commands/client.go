package commands

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	urlKey          = "url"
	authURLKey      = "auth-url"
	clientIDKey     = "client-id"
	clientSecretKey = "client-secret"
)

func init() {
	rootCmd.PersistentFlags().String(urlKey, "", "The URL of the API.")
	viper.SetDefault(urlKey, "https://api.resim.ai/v1/")
	rootCmd.PersistentFlags().String(authURLKey, "", "The URL of the authentication endpoint.")
	viper.SetDefault(authURLKey, "https://resim.us.auth0.com/")
	rootCmd.PersistentFlags().String(clientIDKey, "", "Authentication credentials client ID")
	rootCmd.PersistentFlags().String(clientSecretKey, "", "Authentication credentials client secret")
}

func GetClient(ctx context.Context) (*api.ClientWithResponses, error) {
	clientID := viper.GetString(clientIDKey)
	if clientID == "" {
		return nil, errors.New("client-id must be specified")
	}
	clientSecret := viper.GetString(clientSecretKey)
	if clientSecret == "" {
		return nil, errors.New("client-secret must be specified")
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
	oauthClient := config.Client(ctx)

	return api.NewClientWithResponses(viper.GetString(urlKey), api.WithHTTPClient(oauthClient))
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
