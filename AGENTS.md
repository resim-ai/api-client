# api-client instructions

Read [CLAUDE.md](CLAUDE.md) for repository commands, testing, generated-code rules and directory guidance.

- Organization API-key authentication lives in `auth/auth.go`.
- Human-authenticated key lifecycle commands and HTTP contract tests live in `cmd/signalflag/commands/api_keys.go` and `api_keys_test.go`.
- Lifecycle operation sources are in `bff/queries/api_keys.graphql`; regenerate `bff/queries.gen.go` instead of editing it.
- Local SDL loading and its tests are in `bff/cmd/`; usage is documented in [README.md](README.md#regenerating-the-client).
