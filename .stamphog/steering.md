api-client-specific risky territory, which the path rules can't see. Treat each as risky territory under the policy above:

- Customer scripts: customers run the CLI in CI, so a changed flag, default, output format or exit code breaks them silently.
- Destructive commands: archiving, deleting or overwriting without the existing confirmation or `--force` guard.
- Credentials: printing a token, client secret or API key, logging it in verbose mode, or caching it somewhere broader than before.
- Uploads: `ingest` and `sync` sending more local files or data than the user selected, or to a new host.
- Generated code: a hand edit to `api/client.gen.go`, `bff/queries.gen.go` or `api/mocks/` rather than a regenerated file.

Dependencies: a minor or patch version bump of a dependency the repo already uses is not risky territory here, and needs no independent assurance.
A new dependency, a major version bump, a changed source (a Go `replace` directive) or a removed checksum is risky territory.
