# WOB-4322: Warn when a metrics config defines a metric unused by any metrics set (CLI)

## Context

`resim metrics sync`/`resim metrics validate` let a user define `metrics:` and
`metrics sets:` in a config file, but nothing today tells them when a metric is
defined and never referenced by any set — it silently exists but never
produces data anywhere. We want a non-blocking warning surfaced from both
commands.

The check is computed server-side (BFF), following the existing
`unusedBuilds` precedent on `workflows runs create` — computed server-side,
returned as response data, printed by the CLI as `WARNING: ...` on stderr
without failing the command. This keeps the CLI consistent with how it
already delegates all other metrics-config semantics (schema, query building,
backwards-compat) to the BFF rather than validating locally.

This is the CLI half of a two-repo change; the BFF half is planned separately
in `~/resim/rerun` (worktree `.worktrees/wob-4322-warn-unused-metrics`).

**Revision note:** the original design added `unusedMetrics` directly onto
the `updateMetricsConfig`/`validateMetricsConfig` response by changing those
fields from bare scalars to result objects. That turned out to be a breaking
GraphQL schema change — it broke an internal consumer in the `rerun` repo we
hadn't accounted for, and there's no way to audit every caller of the BFF's
GraphQL API from either repo alone. The BFF side now exposes a new,
purely-additive `findUnusedMetrics(config: String!): [String!]!` query
instead, leaving `updateMetricsConfig`/`validateMetricsConfig` untouched. The
CLI change below reflects that: one extra query call after a successful
sync/validate, rather than reading the field off the existing response.

## Plan

**1. Query documents** — `bff/queries/update_metrics_config.graphql` and
`bff/queries/validate_metrics_config.graphql` are unchanged. Add a new
`bff/queries/find_unused_metrics.graphql`:
```graphql
query FindUnusedMetrics($config: String!) {
    findUnusedMetrics(config: $config)
}
```

**2. Regenerate the client:** `bff/cmd/generate.go` fetches the schema live
via introspection from `GRAPHQL_API_ENDPOINT` (defaults to
`https://bff.resim.ai/graphql`) — it does not read a local schema file. To
pick up the new field before it's deployed, run a local BFF
(`mix phx.server` in the rerun worktree, default `http://localhost:4000`) and
regenerate against it:
```
GRAPHQL_API_ENDPOINT=http://localhost:4000/graphql go generate ./bff/...
```
This generates `bff.FindUnusedMetrics(ctx, client, configB64)` returning
`*FindUnusedMetricsResponse{FindUnusedMetrics []string}`.

**3. Print the warning** — `cmd/resim/commands/utils.go`: a new
`warnUnusedMetrics(configB64 string)` helper calls `bff.FindUnusedMetrics`
and prints to stderr if non-empty, mirroring `workflows.go:758-764`'s
`WARNING: ...` convention. A transport/BFF error here is logged
(`log.Printf`) and swallowed, not returned — this call is purely
informational and must never fail an otherwise-successful sync/validate
(same soft-fail precedent as `validateMetricsSetExists`). Called from
`SyncMetricsConfig` right after `bff.UpdateMetricsConfig` succeeds, and from
`ValidateMetricsConfig` right after `bff.ValidateMetricsConfig` succeeds.

**4. Tests:**
- `cmd/resim/commands/metrics_test.go`: `TestValidateMetricsConfig_PrintsUnusedMetricsWarning`
  / `_NoWarningWhenAllMetricsUsed` mock both the `ValidateMetricsConfig` and
  `FindUnusedMetrics` GraphQL calls and assert on captured stderr. Added a
  `captureStderr` helper (no existing helper captured stderr; only
  `captureStdout` in `agents_test.go:32-52`, which swaps `os.Stdout`).
- `testing/end_to_end_test.go`: `TestMetricsSync`'s new
  `"WarnsOnMetricUnusedByAnyMetricsSet"` subtest, using a dedicated fixture
  (`testing/.resim/metrics/config_unused_metric.resim.yml`) with a metric
  that isn't in any set, asserting stderr contains
  `"WARNING: the following metrics are not used by any metrics set"` on both
  `sync` and `validate`.

**5. Changelog** — bump `CHANGELOG.md` with a new version entry (pattern:
`### vX.Y.Z - <Month DD, YYYY>`) describing the new warning on both `sync`
and `validate`. User-visible behavior is unchanged by the revision — still
one warning line, same wording, same non-fatal semantics.

## Verification

- `go test ./...`, plus the extended e2e subtest
  (`go test ./testing/... -run TestMetricsSync`) against a local BFF running
  the new backend code, confirming `sync` and `validate` both print the
  `WARNING: the following metrics are not used by any metrics set: ...` line
  for a config with an unreferenced metric, and print nothing extra when
  every metric is used.
- Manual: run `resim metrics sync`/`validate` locally against the fixture
  config with an intentionally-unused metric and visually confirm the warning
  and that the command still exits 0.
