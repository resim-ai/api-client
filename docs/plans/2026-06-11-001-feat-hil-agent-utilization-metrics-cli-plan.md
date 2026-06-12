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
(`utilization` 0.0–1.0 and `avgConcurrency` ≥ 0.0 per bucket). Customers who
script against ReSim need to pull this through the CLI ("what fraction of last
week was rack-1 actually running tests?") without opening the UI. Today the CLI
has no agents surface at all on `main`.

---

## Requirements

- R1. `resim agents utilization --agent-id <id> [--start-time RFC3339] [--end-time RFC3339] [--interval hour|day]` calls `getAgentUtilization` and renders the bucket series.
- R2. Default output is a readable table: resolved window + interval header, then one row per bucket with `bucketStart`, `utilization` (percentage), `avgConcurrency`.
- R3. `--json` dumps the raw response body (the `agentUtilizationOutput` schema) untransformed.
- R4. Time flags accept RFC3339; invalid values fail fast client-side with a friendly error before any request is made. Omitted flags are omitted from the request so the server applies its documented defaults (end = now, start = end − 7 days).
- R5. `--interval` is validated client-side against `hour|day`; default `day` (matches the spec default).
- R6. 404 maps to a friendly "agent not found" error; 400 surfaces the server's validation message. Non-zero exit codes on failure, per CLI convention.
- R7. Help text carries the spec's caveats: only `EXPERIENCE_RUNNING` time counts, offline buckets read 0.0, and a sustained 100% can indicate a stuck run.

---

## Scope Boundaries

- Not adding charting/sparkline output — table + JSON only.
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
- **Table rendering uses plain `fmt.Printf` columns** like the rest of the command files (no new table engine). Utilization printed as `xx.x%`, concurrency as `x.xx`.
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
