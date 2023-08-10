# api-client
This repository contains the ReSim API command-line interface (CLI).  It is written in Go and produced via code generation with [openapi-cli-generator](https://github.com/danielgtaylor/openapi-cli-generator) from the publicly-available [API spec](https://api.resim.ai).

## Installation

Install using `go install`:

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


## Developing

You will need Go installed.

### Regenerating the client

Whenever the API spec changes, you will need to regenerate the generated code:

  go generate ./...