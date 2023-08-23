package main

import (
	"os"

	cli "github.com/resim-ai/api-client/cmd/resim/wrapper"
)

// This is the entry point for the CLI.
func main() {
	status := cli.MainWithExitCode()
	os.Exit(status)
}
