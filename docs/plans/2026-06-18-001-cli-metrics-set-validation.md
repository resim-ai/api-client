# CLI metrics-set existence validation (WOB-4051)

## Context

`resim batch create --metrics-set <name>` (and the other create commands) send `metricsSetName`
straight to the core API with **no existence check**. A metrics set is a named collection defined in a
branch's metrics config; its definitions live only in the BFF (`branch_config_versions`), so the Go core
REST endpoint cannot validate them. Today an invalid/typo'd `--metrics-set` is accepted silently and only
surfaces (or silently no-ops) much later at metrics time.

The BFF GraphQL create paths already validate, so the web UI is covered. The gap is the CLI / direct REST
path. The fix is a **client-side precheck in the CLI**: before issuing a create request, call the BFF
GraphQL API to fetch the branch's valid metrics sets and fail fast with a helpful error if the passed name
isn't one of them.

## Approach

The CLI already has a BFF GraphQL client (`BffClient`, genqlient). The BFF schema exposes
`branchConfigVersion(projectId, branchId) { metricsSets { name } }`. A shared helper
(`validateMetricsSetExists`) resolves the branch (already done in each command), queries the latest config
version, and checks the passed name against the available sets.

- Empty `--metrics-set` → no-op.
- BFF lookup error → warn and continue (server stays a backstop; don't block on transient BFF/auth issues).
- Name not found → fatal error listing the available sets.

## Scope

Commands that accept `--metrics-set` and have a resolvable branch get the precheck:

- `batch create` — branch from build.
- `sweep create` — branch from build.
- `report create` — branch from `--branch`.
- `ingest log` — branch from `--branch`/`--build-id`.
- `test-suites run` / revision-run override — branch from build.
- `dashboards create` — branch from `--branch` (BFF `create_dashboard` does not validate either).

Out of scope:

- Test-suite **definition** create — no build/branch context at definition time (mirrors the BFF resolver,
  which also skips when `build_id` is nil).
- `metrics` debug — read-only debug command.

## Verification

- `go build ./... && go vet ./...`
- `go test ./cmd/resim/commands/...`
- Manual: `resim batch create ... --metrics-set bogus` → fatal with available sets;
  `--metrics-set <real>` → succeeds; no flag → unchanged.
