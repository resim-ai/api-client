---
title: "feat: resim agents results + pool-labels list CLI subcommands"
type: feat
status: active
date: 2026-06-17
linear: https://linear.app/resim/issue/WOB-4131/api-client-hil-agent-status-agentspool-labels-cli-subcommands
origin: gap analysis — current api-client CLI vs. deployed rerun agent/pool-label API
---

# feat: resim agents results + pool-labels list CLI subcommands

Follow-up to the HiL agent-status CLI work (WOB-4131). The original status-page
plan ([docs/plans/2026-05-05-001-feat-hil-agent-status-page-cli-plan.md](2026-05-05-001-feat-hil-agent-status-page-cli-plan.md))
shipped `agents list/get/archive` and `pool-labels queue`, then explicitly
deferred three additional shipped endpoints to follow-up work. The utilization
plan ([docs/plans/2026-06-11-001-feat-hil-agent-utilization-metrics-cli-plan.md](2026-06-11-001-feat-hil-agent-utilization-metrics-cli-plan.md))
added `agents utilization`. This plan closes the highest-value remaining gap:
two read commands that have no CLI surface but are already served by deployed
rerun and already callable through the committed generated client.

## Summary

Add two read-only subcommands that reach parity with the deployed rerun
agent/pool-label API for the scripting-focused surface:

- `resim pool-labels list` — the org's distinct pool labels (`GET /poolLabels`).
- `resim agents results --agent-id <id>` — an agent's full results history
  (`GET /agents/{agentID}/results`), the paginated superset of the capped
  "recent activity" already shown by `resim agents get`.

Both follow existing CLI conventions: pretty table/list by default, `--json`
for raw output, auto-pagination (no page flags exposed), and friendly
client-side validation. **No client regeneration is required** — the committed
`api/client.gen.go` and `api/mocks/*` already expose both operations (see Key
Technical Decisions). This is purely additive command code plus tests.

---

## What exists vs. the gap

Deployed rerun (`main`, `api/customerapi/rerun.yml`) exposes ten agent/pool-label
operations. The CLI covers six. This plan adds two of the four uncovered ones;
the other two are deferred (see Scope Boundaries).

| Rerun operation | CLI today | This plan |
|---|---|---|
| `listAgents` | `agents list` | — |
| `getAgent` | `agents get` | — |
| `archiveAgent` | `agents archive` | — |
| `listAgentPoolLabelQueue` | `pool-labels queue` | — |
| `getAgentUtilization` | `agents utilization --agent-id` | — |
| `listAgentUtilization` | `agents utilization` (org-wide) | — |
| `listPoolLabels` (`GET /poolLabels`) | **none** | **`pool-labels list`** |
| `listAgentResults` (`GET /agents/{id}/results`) | **none** | **`agents results`** |
| `listAgentResultBranches` (`GET /agents/{id}/result-branches`) | none | deferred (UI picker) |
| `listAgentMarkdownHistory` (`GET /projects/{id}/agent-markdown-history`) | none | deferred |

---

## Problem Frame

Ops, SREs, and platform engineers script against ReSim via the CLI. They can
already enumerate agents, inspect one, archive one, read the pool-label queue,
and pull utilization — but two things the UI surfaces have no CLI equivalent:

1. **What pool labels exist?** There is no way to list the org's distinct pool
   labels without inferring them from `agents list` rows or the queue. The
   `GET /poolLabels` endpoint exists for exactly this (it backs the UI's
   pool-label search/autocomplete).
2. **What has an agent actually run?** `agents get` shows only the capped
   recent-activity card. The agent detail page's "Results" tab is backed by
   `GET /agents/{agentID}/results`, a paginated, filterable history with a
   total count. Scripters who want "everything rack-1 has run since Monday,
   filtered to the `nav` experience" have no CLI path today.

Both endpoints are deployed and already wired into the committed client; the
only missing piece is the command surface.

---

## Requirements

- R1. `resim pool-labels list` — pretty list by default, `--json` for raw. Auto-paginates the full set (no page flags). Supports `--name <substr>` and `--order-by rank|timestamp`.
- R2. `resim agents results --agent-id <id>` — pretty table by default, `--json` for raw. Auto-paginates the full history. Renders a `Total: N` header plus one row per result, reusing the recent-activity table shape from `agents get`.
- R3. `agents results` supports the deployed filters that carry scripting value: `--text <substr>` (experience-name substring) and `--created-after <RFC3339>` (lower bound). Both are validated client-side; invalid values fail fast before any request.
- R4. `--agent-id` is required on `agents results`; a 404 maps to a friendly "agent not found" error naming the ID; non-zero exit on failure, per CLI convention.
- R5. `--order-by` (pool-labels) is validated client-side against `rank|timestamp`; omitted when unset so the server applies its documented `timestamp` default.
- R6. Help text on every command is useful in isolation: explains org scope, the recent-activity-vs-full-history distinction, and the order-by tradeoff (`rank` recommended with `--name`).
- R7. Output formatting and error handling consistent with existing CLI conventions (tabwriter table, `OutputJson`, `ValidateResponse`, empty-state messages).

---

## Scope Boundaries

- Read-only. No mutation commands, no interactive pickers, no watch/follow mode.
- No new output-format engine — reuse the existing tabwriter/`OutputJson` helpers.
- No pagination flags exposed (`--page-size`/`--page-token`); the CLI auto-fetches all pages like every other list command.
- No changes to the six existing agent/pool-label commands beyond an internal refactor to share the recent-activity row renderer (see U2).
- No client/spec/codegen changes — the deployed spec and committed client already serve both operations.

### Deferred to Follow-Up Work

- **`resim agents result-branches`** (`GET /agents/{id}/result-branches`) — the spec itself describes this as backing the Results-tab branch *filter picker*; it is a UI-shaped concern with little standalone scripting value. Deferred (continues the 2026-05-05 plan's deferral).
- **`resim agents markdown-history`** (`GET /projects/{id}/agent-markdown-history`) — newer, project-scoped; CLI value unproven and not covered by any prior plan. Deferred pending a concrete scripting use case.
- **`agents results --branch-id <uuid>` filter** — maps directly to the `branchIds` param, but is awkward without `result-branches` to discover branch UUIDs. Revisit alongside `result-branches`.
- **`pool-labels queue --project-id`** — rerun `main` keeps the queue org-wide (no `projectID` param); there is active rerun work re-introducing project scoping. When that ships and deploys, re-adding `--project-id` reverses the 2026-05-05 org-wide-only decision. Deferred, gated on the rerun change deploying.
- **`agents utilization` by pool-label** — a pool-level utilization rollup (capacity planning per pool) is valuable but **not reachable against deployed rerun**: `listAgentUtilization` exposes no `poolLabel` filter, and the per-agent series (`AgentUtilizationSeries`) carries only `agentID` + `buckets`, no pool labels — so the CLI cannot group the existing data without a second `agents list` cross-reference. Even then, an agent belongs to *multiple* pool labels (summing per pool double-counts), and "pool utilization" has no server-defined aggregation (mean-of-fractions vs. capacity-weighted busy-seconds differ), so a CLI-only rollup would invent a metric that won't match the UI. Correct sequencing mirrors utilization itself: a rerun-side pool-label dimension first (a `poolLabel` filter param or pool labels on the series, with a defined aggregation), then a thin CLI flag. Deferred, gated on that rerun change.
- **`agents archive --json`** and **`agents utilization --csv`** — carried-over deferred polish from the prior plans; out of scope here.

---

## Context & Research

### Relevant Code and Patterns

- `cmd/resim/commands/agents.go` — the home of both new commands. Already defines the `agents` and `pool-labels` trees, the `agentActivityHeader` recent-activity table, `formatAgentDetail` (which renders capped recent activity), `displayVersion`, the tabwriter list pattern (`listAgents`), and client-side flag validation (`parseAgentUtilizationParams`, `validateCompletedSinceDays`).
- `cmd/resim/commands/system.go` — canonical **auto-pagination** pattern: `listSystems` loops with `PageSize: Ptr(100)`, accumulates into a slice, breaks when `NextPageToken` is nil/empty, then `OutputJson(accumulated)`. Mirror the loop *shape*, but run `ValidateResponse` and nil-check `response.JSON200` **before** reading `NextPageToken`/`Items`/`PoolLabels`/`Total` — `listSystems` (`system.go:287-288`) assigns the token before its `JSON200 == nil` guard, a read-before-nil-check ordering the new commands should not copy. Note `ListPoolLabelsOutput.PoolLabels` is a pointer slice (`*[]PoolLabel`) needing the same nil-check `listSystems` applies to `Systems`, whereas `ListAgentResultsOutput.Items` is a non-pointer slice.
- `cmd/resim/commands/agents_test.go` — the `CommandsSuite` testify-mock pattern: `s.mockClient.On("XWithResponse", matchContext, …)`, `captureStdout`, `sampleAgent`/`sampleUtilizationOutput` builders. Tests set the `Client` global to the mock.
- `cmd/resim/commands/commands.go` / `utils` — `Client` global, `OutputJson`, `ValidateResponse`, viper flag plumbing.
- `api/generate.go` — `go:generate` runs oapi-codegen against `https://api.resim.ai/v1/openapi.yaml` then mockery. **Not invoked by this plan** — the deployed spec already serves both operations and the committed artifacts already reflect them.

### API surface (verified against committed client + deployed `rerun` spec)

`GET /poolLabels` → `ListPoolLabelsWithResponse(ctx, *ListPoolLabelsParams)`:
- Params: `Name *string` (≤256 chars, trigram search), `OrderBy *ListPoolLabelsParamsOrderBy` (`rank`|`timestamp`, server default `timestamp`), `PageSize`, `PageToken`.
- Output `ListPoolLabelsOutput`: `PoolLabels *[]PoolLabel` (`PoolLabel = string`), `NextPageToken *string`.
- Responses: 200, 400, 401 (no 404).

`GET /agents/{agentID}/results` → `ListAgentResultsWithResponse(ctx, agentID, *ListAgentResultsParams)`:
- Params: `PageSize`, `PageToken`, `BranchIds *[]uuid` (form/explode=false), `CreatedAfter *time.Time` (RFC3339), `Text *string` (experience-name substring).
- Output `ListAgentResultsOutput`: `Items []AgentRecentActivity`, `NextPageToken *string`, `Total int`. **`AgentRecentActivity` is the same struct `agents get` already renders** (project/batch/test/status/branch/timestamp, plus optional buildVersion/errorSummary/system fields).
- Responses: 200, 400, 401, 404.

### Institutional Learnings

- `docs/solutions/` does not exist in this repo — no prior learnings to carry forward (consistent with the 2026-05-05 plan's note).

---

## Key Technical Decisions

- **No regeneration.** Confirmed `ListPoolLabelsWithResponse` and `ListAgentResultsWithResponse` are present in both the committed `api/client.gen.go` and `api/mocks/client_with_responses_interface.gen.go`. This plan adds no generated code — it is the first of the agent-CLI plans that needs no `go generate` unit, which removes the undeployed-endpoint risk the predecessors carried.
- **Auto-paginate, hide the pagination.** Both commands loop internally with `PageSize: Ptr(100)` and accumulate, following the `listSystems` loop shape (with the nil-check ordering corrected — see Context & Research). No `--page-size`/`--page-token` flags — consistent with every existing list command, and avoids leaking pagination into a scripting surface.
- **`--json` emits the accumulated items**, not the per-page envelope — matching `listSystems`' `OutputJson(allSystems)`. For `pool-labels list` that is a JSON array of label strings; for `agents results` a JSON array of `AgentRecentActivity` (the `Total` is then `len()`), so output round-trips against the item schema.
- **Reuse the recent-activity renderer.** Extract the per-row rendering currently inline in `formatAgentDetail` into a shared helper, parameterized on indent, so `agents get` (indented, capped) and `agents results` (top-level, full, with a `Total: N` header) render identical columns. Minimal refactor; no behavior change to `get`.
- **Filter flags follow the omit-when-empty precedent** of `parseAgentUtilizationParams`: unset flags are not sent so the server applies defaults; set flags are validated client-side (`--created-after` parsed via `time.RFC3339`, `--order-by` checked against the enum) and fail fast with a friendly message before any request leaves the machine.
- **Flag naming follows existing precedent:** `--agent-id`, `--json`, `--name`, `--order-by`, `--text`, `--created-after`. Reuse the existing `agentIDKey`/`agentJSONKey` consts.
- **Command placement:** `agents results` joins the `agents` tree and `pool-labels list` joins the `pool-labels` tree, both in `agents.go` (where the trees already live). Splitting the pool-labels tree into `pool_labels.go` stays at the implementer's discretion, matching the prior plan's stance.
- **Auth:** existing `RESIM_ACCESS_TOKEN` / config-file flow; no new auth surface. Both endpoints require `projects:read`.

---

## Implementation Units

### U1. `resim pool-labels list` subcommand

**Goal:** List the org's distinct pool labels, auto-paginated, with `--name`/`--order-by` filters and `--json`.

**Requirements:** R1, R5, R6, R7.

**Dependencies:** none.

**Files:**
- Modify: `cmd/resim/commands/agents.go` (register `listPoolLabelsCmd` under `poolLabelsCmd`; add flag consts, `init()` wiring, run func, formatter).
- Modify: `cmd/resim/commands/agents_test.go` (suite methods).

**Approach:**
- New `listPoolLabelsCmd` with flags `--name` (string), `--order-by` (string, empty default), `--json`.
- Validate `--order-by` against `rank|timestamp` when non-empty; build `ListPoolLabelsParams` setting `Name`/`OrderBy` only when provided.
- Auto-paginate (`PageSize: Ptr(100)`, nil-check `JSON200` then accumulate `*PoolLabels`, break on empty `NextPageToken`) per the corrected `listSystems` loop shape.
- Pretty output: a `N pool labels:` header (or empty-state message) followed by one label per line. `--json`: the accumulated `[]string`.

**Patterns to follow:** `listSystems` pagination loop (`system.go`); `listAgents` tabwriter/empty-state shape and `--json` branch (`agents.go`); `--order-by` validation mirrors `parseAgentUtilizationParams`' interval check.

**Test scenarios:**
- Happy path (multi-page): page 1 returns labels + a `nextPageToken`, page 2 returns more with empty token → output contains labels from *both* pages (proves accumulation) and the count header reflects the total.
- Happy path (single page): 3 labels → 3 lines under an `N pool labels:` header.
- Edge: zero labels → "No pool labels found." message, not an empty list.
- Edge (`--json`): output parses as a JSON array of strings and includes labels from all pages.
- Flag: `--order-by rank` → `OrderBy` param set to `rank`; `--name foo` → `Name` set.
- Error path: `--order-by bogus` → client-side error naming `rank|timestamp`, no API call made.
- Edge: `--order-by` unset → `OrderBy` omitted from the request (nil).

**Verification:** `go test ./cmd/resim/commands/ -run TestCommandsSuite` passes; `./resim pool-labels list --help` shows `--name`, `--order-by`, `--json` and no page flags.

### U2. `resim agents results` subcommand

**Goal:** Render an agent's full, paginated results history with a total count and the deployed filters, reusing the recent-activity table.

**Requirements:** R2, R3, R4, R6, R7.

**Dependencies:** none (U1 independent; may land in either order).

**Files:**
- Modify: `cmd/resim/commands/agents.go` (register `agentResultsCmd` under `agentsCmd`; add flag consts, `init()` wiring, run func; extract a shared recent-activity row helper from `formatAgentDetail`).
- Modify: `cmd/resim/commands/agents_test.go` (suite methods).

**Approach:**
- New `agentResultsCmd` with required `--agent-id` (via cobra `MarkFlagRequired`, enforced at `Execute()` like `getAgent`), plus `--text` (string), `--created-after` (RFC3339 string), `--json`.
- Validate `--created-after` via `time.Parse(time.RFC3339, …)` and set `Text`/`CreatedAfter` only when provided.
- Auto-paginate (`PageSize: Ptr(100)`, nil-check `JSON200` then accumulate `Items`, break on empty `NextPageToken`).
- Map 404 → friendly "agent %q not found" (mirror `getAgent`/`archiveAgent`); other non-200 → `ValidateResponse`.
- Pretty output: a header line with the agent ID and `Total: N` — use the server `Total` field as the authoritative count (not "server-or-accumulated"). Confirm during implementation whether server `Total` reflects the **filtered** count when `--text`/`--created-after` are set; if it is unfiltered, derive the displayed count from the rendered rows so the header never overstates what is shown. Then the recent-activity table via the shared row helper (top-level, non-indented). Empty-state: "No results for agent %q." `--json`: the accumulated `[]AgentRecentActivity` — consumers get the count via array length (no separate `Total`/`nextPageToken`), which equals the full set by design.
- Refactor: pull the per-row formatting out of `formatAgentDetail` into a helper taking an indent prefix so `get` (indented, capped) and `results` (top-level, full) share columns; `get`'s output is unchanged.

**Patterns to follow:** `formatAgentDetail`'s recent-activity table and `agentActivityHeader` (`agents.go`); `getAgent`'s 404 handling; `listSystems` pagination; `parseAgentUtilizationParams`' RFC3339 parsing.

**Test scenarios:**
- Happy path (multi-page): two pages of `AgentRecentActivity` → all rows render in the table; the `Total` header matches.
- Happy path (columns): a result with project/test/status/branch set → row shows project name, test name, both conflated statuses, branch, timestamp; matches `get`'s columns.
- Edge: zero results → "No results for agent %q." with `Total: 0`, no empty table.
- Edge: result with nil `BranchName`/`BuildVersion`/`ErrorSummary` → renders without panic (empty branch cell).
- Edge (`--json`): output parses as a JSON array and round-trips the item fields across pages.
- Filter: `--text merge` → `Text` param set; `--created-after 2026-06-01T00:00:00Z` → `CreatedAfter` set.
- Error path: `--created-after notatime` → client-side error before any API call.
- Error path: 404 → non-zero exit, message includes the agent ID.
- Flag wiring: `--agent-id` is registered with cobra `MarkFlagRequired` (enforced at `Execute()`, like `getAgent`/`archiveAgent`). `CommandsSuite` invokes run funcs directly and so does not assert the required-flag error — do not add a suite case for it.
- Regression: `formatAgentDetail` output for `agents get` is unchanged after the row-helper extraction (existing `get` tests still pass).

**Verification:** `go test ./cmd/resim/commands/ -run TestCommandsSuite` passes; `./resim agents results --help` shows `--agent-id` (required), `--text`, `--created-after`, `--json`; `./resim agents get` output is byte-identical to before.

### U3. CHANGELOG entry and E2E smoke

**Goal:** Document the new commands and add a light staging smoke check.

**Requirements:** R1, R2.

**Dependencies:** U1, U2.

**Files:**
- Modify: `CHANGELOG.md` (new unreleased "ReSim CLI" entry covering both commands).
- Modify: the `end_to_end`-tagged tests under `testing/` (match the prior plan's E2E placement).

**Approach:**
- CHANGELOG: one bullet per command, noting org scope, auto-pagination, filters, and `--json`; plus a "Library consumers" note only if any exported surface changed (it should not — the client methods already shipped).
- E2E: a happy-path check that `resim pool-labels list` exits 0 (empty-state output acceptable — staging may have no labels). No E2E for `agents results` (it needs a known agent ID; deriving one from `agents list` is brittle when the org has no agents) — note this explicitly rather than adding a flaky test.

**Test scenarios:**
- Happy path (E2E): `resim pool-labels list` against staging exits 0 with either labels or the empty-state message.
- Test expectation (CHANGELOG): none — documentation only.

**Verification:** `go build ./...`, `go vet ./...`, `go test ./...` pass; E2E runs in the existing CI E2E job or locally with staging credentials.

---

## System-Wide Impact

- **Interaction graph:** adds one subcommand to each existing tree (`agents`, `pool-labels`); no removed or renamed commands; no shared state.
- **Refactor blast radius:** extracting the recent-activity row helper touches `formatAgentDetail`; the `agents get` regression test (U2) gates against output drift.
- **Error propagation:** customer-API errors map to non-zero exits with friendly stderr messages, preserving convention; 404 on `results` names the agent ID.
- **API surface parity:** `pool-labels list` and `agents results` reach the deployed rerun surface for the scripting use cases; `result-branches`/`markdown-history` remain UI-shaped and deferred.
- **No regenerated-client churn:** this plan regenerates nothing, so it adds no incidental-spec-drift of its own and carries no undeployed-endpoint risk (both operations are already live). The command code still depends on the generated types for these two operations, so a *future* canonical `go generate` (against the live spec) could drift them like any other call site — that risk is deferred, not eliminated.
- **Unchanged invariants:** all existing CLI commands behave identically.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Recent-activity refactor changes `agents get` output | Regression test asserting `get`'s formatted output is unchanged (U2) |
| Large results history → many pages / slow command | `PageSize: 100` like the rest of the CLI; `--text`/`--created-after` narrow the set; acceptable for a scripting command |
| Staging has no pool labels → E2E ambiguous | Assert exit 0 + (labels OR empty-state message), per the prior plan's E2E convention |
| `agents results` E2E needs an agent ID | Explicitly omit the E2E rather than add a flaky derive-an-ID test (documented in U3) |
| Endpoints are deployed but org lacks data | Empty-state messages on both commands; no crash on empty/optional fields (tested) |

---

## Documentation / Operational Notes

- `--help` output is the documentation; descriptions must stand alone (org scope, recent-activity-vs-full-history distinction, `rank`-with-`--name` tip).
- No deploy gate: both endpoints are already live in production, so the release can ship as soon as the CLI lands (contrast with the utilization plan, which waited on a rerun deploy).

---

## Sources & References

- **Linear:** [WOB-4131](https://linear.app/resim/issue/WOB-4131/api-client-hil-agent-status-agentspool-labels-cli-subcommands) — the agent-status CLI ticket whose "Deferred to Follow-Up Work" this plan continues.
- **Predecessor plans:** [docs/plans/2026-05-05-001-feat-hil-agent-status-page-cli-plan.md](2026-05-05-001-feat-hil-agent-status-page-cli-plan.md) (deferred these endpoints), [docs/plans/2026-06-11-001-feat-hil-agent-utilization-metrics-cli-plan.md](2026-06-11-001-feat-hil-agent-utilization-metrics-cli-plan.md).
- **Spec of record:** `rerun` `api/customerapi/rerun.yml` at `main`, deployed at `https://api.resim.ai/v1/openapi.yaml`; operations `listPoolLabels`, `listAgentResults`.
- **Generated artifacts (already present):** `api/client.gen.go`, `api/mocks/client_with_responses_interface.gen.go`.
- **Patterns:** `cmd/resim/commands/system.go` (auto-pagination), `cmd/resim/commands/agents.go` (trees, recent-activity table, flag validation).
