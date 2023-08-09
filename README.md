# api-client
This repository contains the ReSim API command-line interface (CLI).  It is written in Go and produced via code generation with [openapi-cli-generator](https://github.com/danielgtaylor/openapi-cli-generator) from the publicly-available [API spec](https://api.resim.ai).

## Usage

To get a list of available commands, just type

    resim

To call a particular endpoint, use

    resim <endpoint> <parameters>

For example, to add a project:

    resim createproject <<EOF
        {
            "name": "test project"
        }
    EOF

### Authentication

The ReSim CLI uses profiles to track your credentials.  There are two login methods supported: user credentials and client credentials.
User credentials will use your ReSim login information.  Client credentials use a client ID and client secret.

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

When running the CLI, you will need to specify a profile with the `--profile` argument.  `--profile` defaults to `default`, so if you name your profile `default`,
you can omit this argument.

## Developing

You will need Go installed as well as the openapi-cli-generator.

    go install github.com/danielgtaylor/openapi-cli-generator

### Regenerating the client

Whenever the API spec changes, you will need to regenerate the generated code:

    cd resim
    wget -O api.yaml https://api.resim.ai
    openapi-cli-generator generate api.yaml

