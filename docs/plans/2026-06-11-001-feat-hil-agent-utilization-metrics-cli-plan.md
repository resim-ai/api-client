---
title: "feat: HiL agent utilization metrics — api-client (CLI)"
type: feat
status: active
date: 2026-06-11
---

# feat: HiL agent utilization metrics — api-client (CLI)

Part of the cross-repo HiL agent utilization metrics feature. The rerun side
(endpoint, query, bucketing) is complete on `rerun` branch
`feat/hil-agent-utilization-metrics` — see
[rerun/docs/plans/2026-06-09-001-feat-hil-agent-utilization-metrics-rerun-plan.md](../../../rerun/docs/plans/)
— and this plan delivers the customer-facing CLI follow-up that the rerun plan
explicitly deferred to this repo.

## Summary

Add a `resim agents utilization` subcommand that calls the new
`GET /agents/{agentID}/utilization` endpoint and renders the dense, bucketed
utilization series either as a human-readable table (default) or raw JSON
(`--json`). Regenerate the customerapi client (`api/client.gen.go` + mocks)
from a spec that includes the new operation.

This branch also **consolidates the agent CLI surface** so all agents work
ships on one branch: `resim agents list/get/archive` and `resim pool-labels
queue`. The implementation of those subcommands is adopted from PR #206
(`pete/wob-4131-api-client-hil-agent-status`), which realigned the original
PR #199 work to the shipped agent-status API and verified it against staging
(unit + E2E smoke tests, prompt-decline coverage, completed-since-days range
validation). Both #199 and #206 are superseded by this branch.

---

## Problem Frame

The rerun branch ships a per-agent, bucketed utilization time-series
(`utilization` 0.0–1.0 per bucket, plus per-bucket `offline` and test-start
counts; `avgConcurrency` was later dropped — see the 2026-06-15 amendment). Customers who
script against ReSim need to pull this through the CLI ("what fraction of last
week was rack-1 actually running tests?") without opening the UI. Today the CLI
has no agents surface at all on `main`.

---

## Requirements

- R1. `resim agents utilization --agent-id <id> [--start-time RFC3339] [--end-time RFC3339] [--interval hour|day]` calls `getAgentUtilization` and renders the bucket series.
- R2. Default output is a readable table: resolved window + interval header, then one row per bucket with `bucketStart`, `utilization` and `offline` (percentages), and `tests` (count). *(Per the 2026-06-15 amendment: no `avgConcurrency` column, and `idle` is not surfaced — it's inferable as 1 − utilization − offline.)*
- R3. `--json` dumps the raw response body (the `agentUtilizationOutput` schema) untransformed.
- R4. Time flags accept RFC3339; invalid values fail fast client-side with a friendly error before any request is made. Omitted flags are omitted from the request so the server applies its documented defaults (end = now, start = end − 7 days).
- R5. `--interval` is validated client-side against `hour|day`; default `day` (matches the spec default).
- R6. 404 maps to a friendly "agent not found" error; 400 surfaces the server's validation message. Non-zero exit codes on failure, per CLI convention.
- R7. Help text carries the spec's caveats: only `EXPERIENCE_RUNNING` time counts, offline buckets read 0.0, and a sustained 100% can indicate a stuck run.
- R8. *(Amendment 2026-06-12)* `--csv` emits the bucket series as CSV (machine-readable raw fractions, UTC RFC3339 timestamps). Mutually exclusive with `--json`.

---

## Scope Boundaries

- Not adding charting/sparkline output. Shipped output is table + JSON; CSV is scoped but deferred (see the 2026-06-12 amendment).
- Not adding org-wide or pool-label rollups (the endpoint is per-agent only).
- Not changing `api/generate.go`'s canonical spec URL — see Key Technical Decisions.

### Coordination with PRs #199 and #206

PR #199 defined `agentsCmd` against a pre-rename spec and was closed. PR #206
(same branch, opened by a parallel workspace) realigned that work to the
shipped agent-status API and staging-verified it, but was generated from the
deployed spec — which lacks `getAgentUtilization` — and based on a stale main.
Rather than land two conflicting PRs, #206's implementation (commands, tests,
E2E smoke, plan doc) is merged into this branch wholesale, with `utilization`
added on top in the same style. #206 closes in favour of this branch.

Carried over from #206's review as deferred polish (validator-confirmed,
non-blocking): non-interactive `archive` decline exits 0; pool-labels tree
co-located in `agents.go`; `archive` lacks `--json`.

---

## Context & Research

### Relevant Code and Patterns

- `cmd/resim/commands/dashboards.go` — current minimal command-tree pattern on `main` (var block, key consts, `init()` flag registration, `rootCmd.AddCommand`).
- `cmd/resim/commands/commands.go` — `Client` global, `OutputJson`, viper flag plumbing.
- `cmd/resim/commands/utils` — `ValidateResponse` helper for status-code handling.
- `cmd/resim/commands/batch_test.go` — `CommandsSuite` with `mockapiclient.ClientWithResponsesInterface`; tests set the `Client` global.
- `api/generate.go` — codegen entry points (oapi-codegen against the deployed spec URL, then mockery).

### Spec source for codegen

The deployed spec (https://api.resim.ai/v1/openapi.yaml) matches rerun `main`
at the operation level but does **not** yet include `getAgentUtilization`
(the rerun branch is unmerged/undeployed). The client is therefore regenerated
from the rerun feature branch's `api/customerapi/rerun.yml`, which is rerun
`main` + the utilization endpoint — a strict superset of the deployed spec.
This also pulls in the already-deployed agents operations (`listAgents`,
`getAgent`, `archiveAgent`, `listAgentPoolLabelQueue`) that `main`'s generated
client predates; that is generated code catching up with production, not new
surface.

---

## Key Technical Decisions

- **Generate from the rerun branch's `rerun.yml`, commit the result, leave `generate.go` untouched.** One-off generation against the local superset spec; once rerun merges and deploys, the canonical `go generate` against the URL reproduces the same code.
- **New file `cmd/resim/commands/agents.go` holds the full `agents` tree** (`list`, `get`, `archive`, `utilization`) plus the `pool-labels` tree (`queue`), consolidating PR #199's surface onto one branch.
- **Flag naming follows precedent:** `--agent-id`, `--start-time`, `--end-time`, `--interval`, `--json`. Time parsing via `time.RFC3339`.
- **Table rendering uses plain `fmt.Printf` columns** like the rest of the command files (no new table engine). Utilization and offline printed as `xx.x%`, tests as an integer count.
- **JSON output is the raw `JSON200` value** via `OutputJson` — no transformation, so it round-trips against the OpenAPI schema.

---

## Implementation Units

- U1. **Regenerate client + mocks** from the feature spec. Verify `GetAgentUtilizationWithResponse` exists; `go build ./...` passes.
- U2. **`agents utilization` subcommand** in `cmd/resim/commands/agents.go`, registered on `rootCmd`. Flag validation, request, table/JSON rendering, friendly 404/400 handling.
- U3. **Tests** in `cmd/resim/commands/agents_test.go`:
  - required/optional flag wiring (dashboards_test.go pattern),
  - happy path: mocked client returns a 3-bucket series → table rows render with correct percentage formatting,
  - `--json`: output parses as JSON and matches the schema fields,
  - empty bucket list → window header still prints, "no buckets" notice,
  - invalid `--start-time` / `--interval` → fail before any client call,
  - 404 → exit path exercised via mocked response.

## Amendment (2026-06-12): CSV output

**Status: deferred — not implemented.** Decided 2026-06-15 to leave CSV unbuilt
for now; the shipped CLI is table + JSON only (no `--csv` flag exists). It's
pure client-side formatting with no server dependency, so it can be picked up
anytime. The spec below stands for whenever it is. Would apply to both the
single-agent (`--agent-id`) and all-agents modes.

> A local-timezone-aware bucketing option (`--timezone`) was scoped alongside
> CSV on 2026-06-12 but **dropped on 2026-06-15** together with the rerun-side
> timezone amendment; buckets stay UTC-aligned. CSV is retained, decoupled
> from it.

### CSV (R8)

- New `--csv` boolean flag, mutually exclusive with `--json`
  (`cmd.MarkFlagsMutuallyExclusive`).
- Written with `encoding/csv` to stdout. Header row:
  `agent_id,bucket_start,bucket_end,utilization,offline,tests_run`.
  `agent_id` is populated in both modes (single-agent and all-agents) so the
  schema is stable; all-agents mode emits one block of rows per agent under
  the single header. No `avg_concurrency` or `idle` columns — concurrency was
  removed, and idle is derivable as `1 - utilization - offline`.
- Values are machine-readable: `utilization`/`offline` as raw fractions
  (no `%`), `tests_run` as an integer, timestamps UTC RFC3339. Spreadsheets
  format; the CLI doesn't.

### Tests to add

- CSV golden output for a 3-bucket series (single-agent) and a 2-agent
  all-agents series; header row present; parses with `encoding/csv`.
- `--csv --json` together → error, no request made.

## Amendment (2026-06-15): drop concurrency; derive idle; per-agent tests-run

**Status: complete (cross-repo).** A HiL agent runs one experience at a time,
so the bucket's concurrency dimension carried no information beyond
utilization (with no overlap, running-seconds == union-of-intervals). Removed
end-to-end:

- **rerun core** (`feat/hil-agent-utilization-breakdown`): dropped
  `avgConcurrency` from the `agentUtilizationBucket` schema in `rerun.yml`,
  from `utilizationBucket`/`bucketUtilization`, and from `mapUtilizationBuckets`;
  regenerated `server.gen.go`; updated unit + endpoint tests.
- **rerun BFF**: regenerated the core-api client + GraphQL types
  (`mix generate.all_core_api`, run twice — `generate.schemas` reads the
  *compiled* client schema, so a second pass is needed for the GraphQL type to
  drop the field). The web UI never referenced `avgConcurrency`, so no UI
  change.
- **api-client**: regenerated `client.gen.go` + mocks from the rerun spec and
  removed the `AVG CONCURRENCY` column + help bullet.

Two related items handled in the same regen:

- **idle is no longer surfaced.** The rerun spec had already dropped the
  server-sent `idle` field (it is fully determined by
  `1 − utilization − offline`); this branch hadn't been regenerated since, so
  the regen synced it away. The CLI drops the `IDLE` column entirely rather
  than deriving it — idle is reasonably inferable from utilization and offline,
  so it doesn't warrant its own column.
- **per-agent absolute tests-run.** The org-wide (`--agent-id`-omitted) view
  now prints `Tests run: N` per agent, summed from that agent's per-bucket
  `testsRun` (each run counts once across the series, so the sum is exact). The
  single-agent view already printed the window total.

Unchanged: `utilization` semantics (% of wall-clock the agent was active);
per-bucket test counts (already attributed to the bucket a run *started* in);
and the avg + median queue wait of runs started in the window.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` pass.
- `./resim agents utilization --help` shows flags and caveat text.
- Manual smoke against staging once the rerun branch deploys (blocked on rerun merge — noted, not a gate for this PR).

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Rerun branch's spec changes before merge (field rename etc.) | Regeneration is one command; CLI touches only stable field names from the reviewed spec |
| PR #199 left open alongside this branch | This branch supersedes it (see Coordination); close #199 when this lands |
| Generated-code diff is large (includes catching up to deployed spec) | Called out in the PR description; reviewers diff `agents.go` + plan only |
| Endpoint not yet deployed → command 404s in prod until rerun ships | PR merges but release waits on rerun deploy; help text is accurate either way |

---

## Sources & References

- rerun branch: `feat/hil-agent-utilization-metrics` (endpoint, spec, plan doc)
- OpenAPI: `GET /agents/{agentID}/utilization`, schemas `agentUtilizationOutput`, `agentUtilizationBucket`
- Related open PR: api-client #199 (`resim agents` tree)
