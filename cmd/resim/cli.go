package main

import (
	"log"
	"strings"

	"github.com/resim-ai/api-client/cmd/resim/commands"
	"github.com/spf13/viper"
)

const EnvPrefix = "RESIM"

func main() {
	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()
	// This confusingly-named function defines the mapping from CLI parameter key to environment variable.
	// CLI parameters use kebab-case, and env vars use CAPITAL_SNAKE_CASE.
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	err := commands.Execute()
	if err != nil && err.Error() != "" {
		log.Fatal(err)
	}
}
