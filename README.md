# api-client
This repository contains the ReSim API command-line interface (CLI).  It is written in Go and produced via code generation with [openapi-cli-generator](https://github.com/danielgtaylor/openapi-cli-generator) from the publicly-available [API spec](https://api.resim.ai).

## Installation

Install using `go install`:

    go install github.com/resim-ai/api-client/resim@latest

## Authentication

The ReSim CLI uses profiles to track your credentials.  There are two login methods supported: user credentials and client credentials.
User credentials will use your ReSim login information.  Client credentials use a client ID and client secret.

Note that, when authenticating via user, the CLI opens a server on port 8484 that your browser will be redirected to.  You need to make sure that port is open
to your browser (for example, if you're using the CLI from within a Docker container, you'll need to have opened port 8484).

Profile information is stored in the `~/.resim/credentials.json` file.  You can add profiles to this with the CLI or by editing that file directly.

To add a user profile with the CLI, use

    resim auth add-profile user <profile name> https://api.resim.ai

For a client credentials profile, use

    resim auth add-profile client <profile name> <client ID> <client secret> https://api.resim.ai

A sample credentials.json might be:

```json
{
  "profiles": {
    "austin": {
      "audience": "https://api.resim.ai",
      "type": "user"
    },
    "client": {
      "audience": "https://api.resim.ai",
      "client_id": "<redacted>",
      "client_secret": "<redacted>",
      "type": "client"
    }
  }
}
```

When running the CLI, you will need to specify a profile with the `--profile` argument.  

If you're only going to use one profile, we recommend using `default` as your profile name.  This will allow you to omit the `--profile` argument.
Otherwise, we suggest using something short and sweet, such as your username (without your domain, like `austin`) or `client` for client credentials flow.

## Usage

To get a list of available commands, just type

    resim

To call a particular endpoint, use

    resim --profile <profilename> <endpoint> <parameters>

Endpoints that POST or PATCH will need you to provide the JSON body on stdin.  We recommend you put the JSON in a file and pipe it in stdin.

```json
project.json
{
    "name": "test project",
    "description": "My Cool Test Project"
}
```

```
resim --profile myprofile createproject <project.json
```

## Developing

You will need Go installed as well as the openapi-cli-generator.

    go install github.com/danielgtaylor/openapi-cli-generator

### Regenerating the client

Whenever the API spec changes, you will need to regenerate the generated code:

    cd resim
    wget -O api.yaml https://api.resim.ai
    openapi-cli-generator generate api.yaml

