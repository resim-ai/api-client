# Sync the metrics config before the metrics-set precheck (WOB-4358)

## Context

`resim batch create --sync-metrics-config --metrics-set <name>` fails on any branch that does not
already have a metrics config:

```
No metrics config exists on this branch. Sync a metrics config to the branch first.
```

The two flags are mutually unusable on a new branch. The `--metrics-set` precheck added by WOB-4051
runs *before* the `--sync-metrics-config` block, so the precheck rejects the branch and the sync that
would have created the config never runs. This breaks CI pipelines that create a build with
`builds create --auto-create-branch` and then create a batch on the freshly-created branch — found by
Bear Flag Robotics CI.

The precheck itself is correct; only its position is wrong. A metrics set is *defined by* the metrics
config, so a set named on the command line frequently exists only in the config that the same
invocation is about to sync. Validating first can only ever see the branch's previous state.

## Regression

- Introduced by `6de71bd` — "feat(cli): validate metrics set exists before create (#213)", 2026-06-29
  (WOB-4051, planned in `2026-06-18-001-cli-metrics-set-validation.md`).
- The sync block it front-runs is much older: `81ab746` — "Wob 3667: Sync config before running a
  batch/test/suite (#166)", 2025-10-17.
- First shipped in v0.59.0 (2026-06-29); present through v0.68.0.

PR #213 tested `--metrics-set <real>`, `--metrics-set bogus`, and no flag — all three assume a branch
that already has a config, which is why its e2e passed. The untested combination is the broken one.

## Approach

Extract the two ordering-coupled steps into one shared helper, `syncAndValidateMetricsSet`
(`cmd/resim/commands/utils.go`), so the ordering is stated once and cannot drift back apart
per-command:

1. Return early when there is neither a sync nor a metrics set — no build lookup.
2. Resolve the build's branch **once** (both steps previously issued their own `GetBuild`).
3. Sync the config when `--sync-metrics-config` is set.
4. Validate the metrics set against the now-current branch.

A sync failure returns before validation, so the user sees the sync error rather than a confusing
"set not found" from a half-synced branch.

The helper returns errors instead of calling `log.Fatal` itself, which is what makes the ordering
unit-testable; callers keep their existing `log.Fatal` behaviour.

## Scope

The four commands that accept both flags:

- `batch create`
- `sweep create`
- `suites run`
- `ingest log`

`report create` and `dashboards create` take `--metrics-set` but have no `--sync-metrics-config`, so
they are unaffected.

Behaviour deliberately preserved:

- **`suites run`** only prechecks an *explicit* `--metrics-set-name-override`. A set inherited from the
  test suite is still left to the server, exactly as before; only the override is passed to the helper.
- A failed precheck now leaves an already-synced config on the branch. This is accepted: the sync was
  explicitly requested, it is idempotent, and the alternative ordering is the bug being fixed.

## Verification

- `go build ./... && go vet ./... && gofmt -l`
- `go test ./... -race`
- Unit (`cmd/resim/commands/validate_metrics_set_test.go`) — four tests on the helper, keyed on the
  order of BFF operations: sync-before-validate, sync failure skips validation, validate works without
  a sync, and no work means no build lookup. The ordering guard was confirmed to fail when the old
  order is restored.
- E2E (`testing/end_to_end_test.go`, `TestBatchAndLogs`) — extends the existing WOB-4051 precheck
  coverage rather than adding a separate test: creates a build on a fresh auto-created branch with no
  config, then runs `batch create --sync-metrics-config --metrics-set woot` in a single invocation and
  asserts the batch is created with that set. A bogus set through the same path is still rejected, so
  the precheck is not merely being skipped when `--sync-metrics-config` is present.
- Manual: reproduced against staging before the fix (project `metrics-set-sync-ordering-repro`,
  `65365e82-3b3b-4117-80fe-fc5ad4698923`) — the same command failed on a fresh branch, succeeded after
  a separate sync had populated the branch.
