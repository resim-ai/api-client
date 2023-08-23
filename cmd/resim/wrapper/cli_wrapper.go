package cli

// We provide this wrapper for the Cobra CLI so that we can run CLI tests using the go testscript
// package. The testscript package requires that the CLI entry point be a function that returns an
// exit code, but the Cobra CLI entry point is a function that returns void. This wrapper provides
// the necessary translation.

import (
	"log"
	"strings"

	"github.com/resim-ai/api-client/cmd/resim/commands"
	"github.com/spf13/viper"
)

const EnvPrefix = "RESIM"

func MainWithExitCode() int {
	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()
	// This confusingly-named function defines the mapping from CLI parameter key to environment variable.
	// CLI parameters use kebab-case, and env vars use CAPITAL_SNAKE_CASE.
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	err := commands.Execute()
	if err != nil && err.Error() != "" {
		log.Fatal(err)
	}
	// Return 0 to indicate success.
	return 0
}
