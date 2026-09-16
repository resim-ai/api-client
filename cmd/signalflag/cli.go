package main

import (
	"log"
	"os"
	"strings"

	"github.com/resim-ai/api-client/cmd/signalflag/commands"
	"github.com/spf13/viper"
)

func main() {
	commands.PromoteLegacyEnv(os.Stderr)
	viper.SetEnvPrefix(commands.EnvPrefix)
	viper.AutomaticEnv()
	// This confusingly-named function defines the mapping from CLI parameter key to environment variable.
	// CLI parameters use kebab-case, and env vars use CAPITAL_SNAKE_CASE.
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	err := commands.Execute()
	if err != nil && err.Error() != "" {
		log.Fatal(err)
	}
}
