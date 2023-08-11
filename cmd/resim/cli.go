package main

import (
	"log"

	"github.com/resim-ai/api-client/cmd/resim/commands"
	"github.com/spf13/viper"
)

const EnvPrefix = "RESIM"

func main() {
	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()
	err := commands.Execute()
	if err != nil && err.Error() != "" {
		log.Fatal(err)
	}
}
