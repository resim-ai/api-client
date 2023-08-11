# api-client
This repository contains the ReSim API command-line interface (CLI).  It is written in Go and produced via code generation with [openapi-cli-generator](https://github.com/danielgtaylor/openapi-cli-generator) from the publicly-available [API spec](https://api.resim.ai).

## Installation

Pre-built binaries are available for linux-amd64, darwin-amd64 (Mac OS) and darwin-arm64 (Mac OS on Apple Silicon/ARM):

For Linux on AMD64:

    curl -L https://github.com/resim-ai/api-client/releases/download/v0.1.4/resim-linux-amd64 -o resim
    chmod +x resim

For Mac OS on Apple Silicon/ARM:

    curl -L https://github.com/resim-ai/api-client/releases/download/v0.1.4/resim-darwin-arm64 -o resim
    chmod +x resim
    
For Mac OS on Intel:

    curl -L https://github.com/resim-ai/api-client/releases/download/v0.1.4/resim-darwin-amd64 -o resim
    chmod +x resim

Or you can install using `go install`:

    go install github.com/resim-ai/api-client/cmd/resim@latest

## Authentication

The ReSim CLI authenticates using client credentials (a client ID and a client secret).  These are obtained by contacting ReSim.  

Client credentials can be specified on the commandline with the `--client_id` and `--client_secret` flags, or in the environment as
`RESIM_CLIENT_ID` and `RESIM_CLIENT_SECRET`.

## Usage

To get a list of available commands, just type

    resim

To call a particular endpoint, use

    RESIM_CLIENT_ID=<client ID> RESIM_CLIENT_SECRET=<client secret> resim create project <flags> 

### Autocomplete

If you would like resim commands to autocomplete you can generate autocomplete scripts using e.g.

    resim complete bash > resim_bash_completion

Then place the generated file in the appropriate location on your system to enable autocomplete e.g.

    mv resim_bash_completion /usr/share/bash-completion/completions/resim

Other shells are supported, just replace `bash` above with e.g. [`zsh`, `fish`, `powershell`]. 

## Contributing

We track issues and feature requests using [Github Issues](https://github.com/resim-ai/api-client/issues).  Feel free to grab an issue and submit a pull request!

### Dependencies

You will need Go installed.

### Regenerating the client

Whenever the API spec changes, you will need to regenerate the generated code:

  go generate ./...
