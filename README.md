# ReSim CLI Tool

This repository contains the ReSim API command-line interface (CLI).  It is written in Go and produced via code generation with [openapi-cli-generator](https://github.com/danielgtaylor/openapi-cli-generator) from the publicly-available [API spec](https://api.resim.ai).

## Installation

Pre-built binaries are available for linux-amd64, linux-arm64, darwin-amd64 (Mac OS) and darwin-arm64 (Mac OS on Apple Silicon/ARM):

For Linux on AMD64:

    curl -L https://github.com/resim-ai/api-client/releases/latest/download/signalflag-linux-amd64 -o signalflag
    chmod +x signalflag

For Linux on ARM64:

    curl -L https://github.com/resim-ai/api-client/releases/latest/download/signalflag-linux-arm64 -o signalflag
    chmod +x signalflag

For Mac OS on Apple Silicon/ARM:

    curl -L https://github.com/resim-ai/api-client/releases/latest/download/signalflag-darwin-arm64 -o signalflag
    chmod +x signalflag

For Mac OS on Intel:

    curl -L https://github.com/resim-ai/api-client/releases/latest/download/signalflag-darwin-amd64 -o signalflag
    chmod +x signalflag

Or you can install using `go install`:

    go install github.com/resim-ai/api-client/cmd/signalflag@latest

## Authentication

When you run any command, if you don't have a cached authentication token, the CLI will prompt you to log in using a web browser.

### Non-Interactive Auth

If you would like to use the CLI in a non-interactive setting (e.g. CI), it can also be configured to authenticate using a username and password or client credentials (a client ID and a client secret). These credentials are obtained by contacting ReSim. We will provide the most appropriate type for your environment.

The username and password can be specified using the `--username` and `--password` flags, or in the environment as `SIGNALFLAG_USERNAME` and `SIGNALFLAG_PASSWORD`.

Client credentials can be specified on the commandline with the `--client-id` and `--client-secret` flags, or in the environment as
`SIGNALFLAG_CLIENT_ID` and `SIGNALFLAG_CLIENT_SECRET`.

The CLI was previously named `resim` and read `RESIM_*` environment variables. Those are still honoured as a fallback when the corresponding `SIGNALFLAG_*` variable is not set, with a deprecation warning; please rename them.

If you would like to store your credentials in a config file, the CLI will load them from `~/.resim/resim.yaml`. Make sure this file is reasonably secure - only readable by the user that will run the CLI, for example. The file is formatted as follows:

    ## Set ONE of the below pairs

    # Client Credentials
    client-id: <client ID>
    client-secret: <client secret>

    # Password
    username: <username>
    password: <password>

### Token Caching

Authentication tokens will be cached in `~/.resim/cache.json` if the user running the CLI has permission to create that directory and file.

## Usage

To get a list of available commands, just type

    signalflag

To call a particular endpoint, use

    SIGNALFLAG_CLIENT_ID=<client ID> SIGNALFLAG_CLIENT_SECRET=<client secret> signalflag create project <flags>

### Autocomplete

If you would like signalflag commands to autocomplete you can generate autocomplete scripts using e.g.

    signalflag completion bash > signalflag_bash_completion

Then place the generated file in the appropriate location on your system to enable autocomplete e.g.

    mv signalflag_bash_completion /usr/share/bash-completion/completions/signalflag

Other shells are supported, just replace `bash` above with e.g. [`zsh`, `fish`, `powershell`].

### GovCloud

If you use our GovCloud environment, you can configure the CLI to work with it by running `signalflag govcloud enable` (which will store the setting in the configuration file at `~/.resim/resim.yaml`) or by setting `SIGNALFLAG_GOVCLOUD=true` in your environment.

## Contributing

We track issues and feature requests using [Github Issues](https://github.com/resim-ai/api-client/issues).  Feel free to grab an issue and submit a pull request!

## Releasing

The release workflow will run when a tag matching `v*` is pushed, so to do a release from `main` tag the relevant commit with the next appropriate version number.

### Dependencies

You will need Go installed.

### Building the client

    go build -o signalflag ./cmd/signalflag

### Regenerating the client

Whenever the API spec changes, you will need to regenerate the generated code:

    go generate ./...

### Running the end to end test

Whenever you make changes, please ensure that the end to end test is passing:

    go test -v -tags end_to_end -count 1 ./testing

The end to end test requires several environment variables to be passed through: `SIGNALFLAG_CLIENT_ID` and `SIGNALFLAG_CLIENT_SECRET`
which must be valid client credentials for the CLI to access the deployment. `CONFIG` should be either `staging` or `prod` to
test the staging or production deployments and for a customer development deployment the `DEPLOYMENT` name should match
the name of your deployment.
