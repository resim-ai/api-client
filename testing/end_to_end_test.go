package testing

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	cli "github.com/resim-ai/api-client/cmd/resim/wrapper"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/spf13/viper"
)

// Test Environment Variables
const (
	Deployment   string = "DEPLOYMENT"
	Config       string = "CONFIG"
	Dev          string = "dev"
	Staging      string = "staging"
	Prod         string = "prod"
	ClientID     string = "RESIM_CLIENT_ID"
	ClientSecret string = "RESIM_CLIENT_SECRET"
	Url          string = "RESIM_URL"
	AuthUrl      string = "RESIM_AUTH_URL"
	TestUuid     string = "RESIM_TEST_UUID"
	Home         string = "HOME"
)

// CLI Constant
const (
	CliName       string = "resim"
	TestsLocation string = "testdata/script"
)

// Endpoint Constants
const (
	ProdEndpoint    string = "https://api.resim.ai/v1/"
	StagingEndpoint string = "https://api.resim.io/v1/"
	DevEndpoint     string = "https://$DEPLOYMENT.api.dev.resim.io/v1/"
	DevAuthUrl      string = "https://resim-dev.us.auth0.com/"
	ProdAuthUrl     string = "https://resim.us.auth0.com/"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		CliName: cli.MainWithExitCode,
	}))
}

func TestScript(t *testing.T) {
	viper.AutomaticEnv()
	// Set to the dev config by default:
	viper.SetDefault(Config, Dev)
	var apiEndpoint string
	var authUrl string
	switch viper.GetString(Config) {
	case Dev:
		deployment := viper.GetString(Deployment)
		if deployment == "" {
			fmt.Fprintf(os.Stderr, "error: must set %v for dev", Deployment)
			os.Exit(1)
		}
		apiEndpoint = strings.Replace(DevEndpoint, "$DEPLOYMENT", deployment, 1)
		authUrl = DevAuthUrl
	case Staging:
		apiEndpoint = StagingEndpoint
		authUrl = DevAuthUrl // The same auth0 instance is used for dev and staging
	case Prod:
		apiEndpoint = ProdEndpoint
		authUrl = ProdAuthUrl
	default:
		fmt.Fprintf(os.Stderr, "error: invalid value for %s: %s", Config, viper.GetString(Config))
		os.Exit(1)
	}

	// Validate the client credential environment variables are set:
	if !viper.IsSet(ClientID) {
		fmt.Fprintf(os.Stderr, "error: %v environment variable must be set", ClientID)
		os.Exit(1)
	}
	if !viper.IsSet(ClientSecret) {
		fmt.Fprintf(os.Stderr, "error: %v environment variable must be set", ClientSecret)
		os.Exit(1)
	}

	testscript.Run(t, testscript.Params{
		Dir: TestsLocation,
		Setup: func(env *testscript.Env) error {
			// We pass through the appropriate environment variables to the CLI:
			env.Setenv(ClientID, viper.GetString(ClientID))
			env.Setenv(ClientSecret, viper.GetString(ClientSecret))
			env.Setenv(Url, apiEndpoint)
			env.Setenv(AuthUrl, authUrl)
			// In order to persist a credential cache, we need to set the home directory to the actual home directory in the system:
			env.Setenv(Home, viper.GetString(Home))
			// Finally, we pass in a UUID to act as a random seed for names of objects in tests:
			env.Setenv("RESIM_TEST_UUID", uuid.New().String())
			return nil
		},
	})
}
