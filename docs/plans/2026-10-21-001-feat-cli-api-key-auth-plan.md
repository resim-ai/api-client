# CLI: sign in with an API key

## What

The CLI accepts an organization API key, so scripts and CI can run it without a person's
login or a client ID and secret requested from SignalFlag.

- `--api-key`, `SIGNALFLAG_API_KEY` or `api-key:` in `~/.signalflag/signalflag.yaml`.
- The key is sent as the bearer token on every request, to the REST API and the BFF. There
  is no Auth0 exchange, and nothing is written to the token cache, so an existing cached
  login is left as it was.
- A key together with client credentials or a username and password is rejected as
  ambiguous.
- What a key can do is decided by the server: a read-only key can run read commands and is
  refused on writes.

## Out of scope

Managing keys from the CLI (`api-keys create|list|revoke`). Keys are created and revoked in
the web app; resim-ai/api-client#235 had those commands and its branch is kept.

## Files

- `auth/auth.go`: the `apikey` auth mode, a static token source and skipping the cache save.
- `auth/auth_test.go`: mode selection, ambiguity errors, no Auth0 call or cache write,
  govcloud, and both clients sending the key as the bearer token.
- `cmd/signalflag/commands/client.go`: the `--api-key` flag.
- `README.md`, `CLAUDE.md`, `CHANGELOG.md`.

## Verification

- `go test ./...`.
- On staging, every CLI command was run with a read-only and a read & write key from the
  earlier branch, which used the same sign-in code: reads work with both keys, writes work
  with read & write and are refused for read-only.
