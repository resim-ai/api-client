package main

import (
	"github.com/danielgtaylor/openapi-cli-generator/auth0"
	"github.com/danielgtaylor/openapi-cli-generator/cli"
	"github.com/spf13/viper"
)

const (
	Auth0ClientIDKey = "auth0_client_id"
	Auth0IssuerKey   = "auth0_issuer"
)

func main() {
	cli.Init(&cli.Config{
		AppName:   "resim",
		EnvPrefix: "RESIM",
		Version:   "1.0.0",
	})

	cli.AddGlobalFlag(Auth0ClientIDKey, "", "Client ID for user authentication", "CsHFNGLgehRwRYTXMKTqoDDWEgVbtxGR")
	cli.AddGlobalFlag(Auth0IssuerKey, "", "Issuer for Auth0", "https://resim.us.auth0.com/")

	clientID := viper.GetString(Auth0ClientIDKey)
	issuer := viper.GetString(Auth0IssuerKey)

	auth0.InitClientCredentials(issuer, auth0.Type("client"))
	auth0.InitAuthCode(clientID, issuer, auth0.Type("user"))

	apiRegister(false)

	cli.Root.Execute()
}
