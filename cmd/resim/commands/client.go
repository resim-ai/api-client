package commands

import (
	"context"
	"errors"
	"net/url"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

func GetClient(ctx context.Context) (*api.ClientWithResponses, error) {
	clientID := viper.GetString(ClientIDKey)
	if clientID == "" {
		return nil, errors.New("client_id must be specified")
	}
	clientSecret := viper.GetString(ClientSecretKey)
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
	url := viper.GetString(URLKey)
	return api.NewClientWithResponses(url, api.WithHTTPClient(oauthClient))
}
