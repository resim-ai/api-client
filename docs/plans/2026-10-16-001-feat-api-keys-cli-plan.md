# Org API keys in the CLI

## What

- `signalflag api-keys create|list|revoke` for org admins to manage their organization's API keys through the BFF GraphQL API (`apiKeys`, `createApiKey`, `revokeApiKey`).
- API key sign-in with `--api-key`, `SIGNALFLAG_API_KEY` or `api-key:` in the config file.

## Why

Customers need long-lived credentials for CI and integrations that they can create and revoke themselves, without asking ReSim for an Auth0 machine client.

## How

- `auth.ModeAPIKey`: the key is used as a static bearer token (`oauth2.StaticTokenSource`), so the REST and BFF clients are unchanged. No Auth0 call is made and nothing is written to the token cache. A key combined with client credentials or a username and password is rejected as ambiguous.
- `api-keys` commands refuse to run unless a person is signed in (device code or password), before any request, with "Managing API keys needs a person signed in as an org admin." The BFF enforces the same rule.
- `create` prints the key once. `revoke --name` revokes the active key with that name, or finishes a revocation still in progress. BFF errors are printed as the BFF returns them.
- `bff/queries.gen.go` regenerated against a BFF with the API key schema (resim-ai/rerun#4363).

## Verification

`go fmt ./...`, `go vet ./...`, `go test ./... -race`. Needs resim-ai/rerun#4359–#4363 deployed.
