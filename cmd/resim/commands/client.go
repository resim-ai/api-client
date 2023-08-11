package commands

import (
	"context"
	"errors"
	"net/url"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

var (
	URL          string
	clientID     string
	clientSecret string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&URL, "url", "", "The URL of the API.")
	rootCmd.PersistentFlags().StringVar(&clientID, "client_id", "", "Authentication credentials client ID")
	rootCmd.PersistentFlags().StringVar(&clientSecret, "client_secret", "", "Authentication credentials client secret")

	viper.BindPFlags(rootCmd.PersistentFlags())
}

func GetClient(ctx context.Context) (*api.ClientWithResponses, error) {
	if clientID == "" {
		return nil, errors.New("client_id must be specified")
	}
	if clientSecret == "" {
		return nil, errors.New("client_secret must be specified")
	}
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     "https://resim.us.auth0.com/oauth/token",
		EndpointParams: url.Values{
			"audience": []string{"https://api.resim.ai"},
		},
	}
	oauthClient := config.Client(ctx)
	url := viper.GetString(URL)
	return api.NewClientWithResponses(url, api.WithHTTPClient(oauthClient))
}
