---
title: "feat: HiL Agent Status page — api-client (CLI)"
type: feat
status: completed
date: 2026-05-05
replanned: 2026-06-11
origin: https://www.notion.so/345d53911af480318600c2ece3efa406
linear: https://linear.app/resim/project/agent-status-page-5dc07b3cc1e9/overview
---

# feat: HiL Agent Status page — api-client (CLI)

Part of: [docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md](../../../docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md)

## Summary

CLI parity for the agent endpoints that shipped in rerun. Add a top-level `resim agents` command tree (`list`, `get`, `archive`) and a `resim pool-labels queue` subcommand, consuming the regenerated customerapi client. Mirrors the existing `resim systems` pattern.

**Replanned 2026-06-11** against the rerun API as actually shipped (rerun PRs #3206, #3210, #3341, #3371, #3401, now on `main` and deployed — the production `openapi.yaml` serves the final shape). The in-flight branch (`pete/wob-4131-api-client-hil-agent-status`, PR #199) was written against a pre-ship draft spec and has drifted; this revision realigns it.

---

## What shipped vs. what the branch assumes

| Surface | Shipped (rerun main + deployed spec) | Branch / original plan assumption |
|---|---|---|
| `GET /agents` | **Unpaginated**, no query params; envelope carries `agents` + `latestKnownVersion` | Paginated (`pageSize`/`pageToken`), CLI `--all` flag and pagination loop |
| Soft-delete | `POST /agents/{agentID}/archive` → `archiveAgent`, returns **200** `{agentID, archivedAt}`, idempotent | `removeAgent` expecting **204**; CLI verb `remove` |
| `GET /agents/poolLabels/queue` | **Org-wide**; only param is `completedSinceDays` (1–30, default 7) | Optional `projectID` param; CLI `--project-id` flag |
| `poolLabelQueueItem` | `activeBatches` is an **array** (concurrent runners are normal) | Singular optional `activeBatch` |
| `poolLabelQueueBatch.priority` | **Integer** raw scheduler priority (default 1000; `<1000` = elevated, render "High") | Boolean |
| `agentRecentActivity` | Adds `projectName` (always present), `projectID`, `workerID`, `systemID`/`systemName`, `buildID` (all optional) | Smaller card without project/system/worker metadata |
| Additional endpoints | `GET /poolLabels`, `GET /agents/{agentID}/results`, `GET /agents/{agentID}/result-branches` | Not covered — see Scope Boundaries |

---

## Problem Frame

Ops, SREs, and platform engineers script against ReSim via the CLI. They must be able to inspect the agent fleet, fetch a specific agent, archive (soft-delete) an agent, and inspect the org's per-pool-label queue without opening the UI. Today there is no `resim agents …` command tree at all. PR #199 drafted one against a stale spec; it does not compile against the shipped client shapes once regenerated.

---

## Requirements

- R1. `resim agents list` — pretty table by default, `--json` for raw output. No pagination flags (the endpoint is unpaginated by design).
- R2. `resim agents get --agent-id <id>` — single-agent detail view.
- R3. `resim agents archive --agent-id <id> [--yes]` (aliases: `remove`, `hide`) — confirms by default; `--yes` skips. Soft-delete semantics in the help text; success message reports the server's `archivedAt`.
- R4. `resim pool-labels queue [--completed-since-days 7]` — org-wide, pretty grouped output by default, `--json` for raw.
- R5. Help text on every command explains the archive auto-reappear behaviour and the completed-batch window.
- R6. Output formatting consistent with existing CLI conventions.
- R7. The generated client (`api/client.gen.go` + mocks) is regenerated from the deployed spec so the CLI compiles against the shipped shapes.

---

## Scope Boundaries

- Not adding interactive selection (no fzf-style picker).
- Not adding a watch / follow mode (no `--watch`).
- Not introducing a new output format engine — reuse the existing one.
- Not modifying any other command tree (incidental regenerated-client diffs excepted).

### Deferred to Follow-Up Work

- CLI coverage for the three additional shipped endpoints: `resim pool-labels list` (`GET /poolLabels`), `resim agents results` (`GET /agents/{agentID}/results`), and `resim agents result-branches` (`GET /agents/{agentID}/result-branches`). Per-agent batch history was deferred in the original plan; the result-branches picker is a UI-shaped concern. Revisit on demand.
- A `resim agents adopt` / permanent-disable distinction: deferred.

---

## Context & Research

### Relevant Code and Patterns

- `cmd/resim/commands/system.go`, `experience.go`, `project.go`, `assets.go`, `test_suites.go` — every existing tree uses the **`archive`** verb for soft-delete; `agents` must match.
- `cmd/resim/commands/test_suites_test.go`, `batch_test.go` — the `CommandsSuite` testify-mock pattern (`s.mockClient.On("XWithResponse", matchContext, …)`) for command tests.
- `cmd/resim/commands/client.go` — customerapi client wiring.
- `api/generate.go` — `go:generate` runs oapi-codegen against `https://api.resim.ai/v1/openapi.yaml` then mockery. Production now serves the shipped agent endpoints, so the standard regeneration path is correct.
- The in-flight branch `pete/wob-4131-api-client-hil-agent-status` (PR #199) — `cmd/resim/commands/agents.go` exists and is the starting point; rework rather than rewrite.

### Institutional Learnings

- None directly for api-client in `docs/solutions/`.

---

## Key Technical Decisions

- **Rework PR #199 in place** rather than open a new PR — the command-tree skeleton, styling, and confirmation flow are sound; only the client-shape contact points and missing tests change.
- **Regenerate via the standard `go generate` path** (deployed prod spec). Verified 2026-06-11: prod `openapi.yaml` matches `rerun` `origin/main` for all agent surfaces (unpaginated `listAgents`, `archiveAgent`, org-wide queue).
- **`archive` verb, not `remove`/`hide`** — matches both the shipped operationId (`archiveAgent`) and every existing CLI tree. The 2026-05-05 orchestration plan's U8 said `hide` and its Round 2 designer note renamed the UI button to "Remove"; both predate the shipped API, which settled on archive. The CLI follows the shipped contract and its own conventions, with `remove` and `hide` registered as cobra aliases so users arriving from either older vocabulary or the UI button copy still find the command. PR #199 is unmerged, so no compatibility shim is needed.
- **One file per command tree** stays: `agents.go` (agents tree) and the pool-labels tree. Splitting pool-labels out of `agents.go` into `pool_labels.go` is at the implementer's discretion; tests are required either way.
- **Flag naming follows existing precedent.** `--agent-id`, `--completed-since-days`, `--yes`, `--json`.
- **Table output** for `list` shows: agent ID, activity (ACTIVE/INACTIVE), version (suffixed `(out of date; latest X)` when applicable), pool labels, last check-in. Detail view adds first check-in and recent-activity cards including the new project name (always present) and branch when set.
- **Priority rendering:** integer priority renders as `(High)` when `< 1000`, nothing at default 1000, `(Low)` when `> 1000` — mirrors the scheduler ASC-sort semantics described in the spec.
- **Queue output** grouped by pool label: active batches (plural) listed first, queued batches with their 1-based positions, completed batches summarized as `+N completed in last D days` where D is the flag value (not hardcoded 7).
- **JSON output** is the raw client response, no transformation.
- **Auth:** existing `RESIM_ACCESS_TOKEN` / config-file flow; no new auth surface.

---

## Open Questions

### Resolved During Planning

- *Where does `pool-labels queue` go?* Its own tree (`pool-labels`), unchanged from the original plan.
- *Confirmation prompt or `--yes` only?* Prompt by default with `--yes` to skip, matching existing destructive-action precedent.
- *Keep `--all`/pagination affordances for `agents list`?* No — the shipped endpoint is deliberately unpaginated; carrying dead flags would mislead.
- *Keep `--project-id` on the queue?* No — the shipped endpoint is org-wide with no project filter.

### Deferred to Implementation

- Exact column widths and truncation — defer to existing styling helpers.
- Whether `pool_labels.go` is split from `agents.go`.

---

## Implementation Units

### U4. Regenerate the customerapi client from the shipped spec

**Goal:** `api/client.gen.go` and `api/mocks/*` reflect the deployed API: no `ListAgentsParams`/`RemoveAgent`; `ArchiveAgent*`, array `ActiveBatches`, integer `Priority`, enriched `AgentRecentActivity`.

**Requirements:** R7.

**Dependencies:** none (rerun changes are deployed to prod).

**Files:**
- Regenerate: `api/client.gen.go`, `api/mocks/client_interface.gen.go`, `api/mocks/client_with_responses_interface.gen.go`.

**Approach:** run the repo's standard `go generate` for the `api` package (oapi-codegen against the deployed spec, then mockery). Inspect the diff to confirm the agent-surface shapes listed in the Goal; unrelated incidental spec drift is acceptable if the build stays green.

**Test scenarios:** Test expectation: none — generated code; correctness is proven by U1/U2 compiling and their tests passing.

**Verification:** generated client contains `ArchiveAgentWithResponse` and no `ListAgentsParams`; `go build ./...` fails only in `agents.go` (expected until U1/U2 land).

### U1. `agents` command tree (list, get, archive)

**Goal:** The three agent subcommands work against the shipped client shapes.

**Requirements:** R1, R2, R3, R5, R6.

**Dependencies:** U4.

**Files:**
- Modify: `cmd/resim/commands/agents.go`
- Create: `cmd/resim/commands/agents_test.go`

**Approach:**
- `list`: single `ListAgentsWithResponse(ctx)` call — delete the pagination loop, `--all` flag, and page-token plumbing. Render rows with the `latestKnownVersion`-aware out-of-date suffix; suppress the suffix entirely when `latestKnownVersion` is empty (spec: clients must treat empty as "no canonical latest").
- `get`: unchanged shape; enrich the recent-activity card lines with `projectName` (always present) alongside batch/test names and statuses.
- `archive` (renamed from `remove`; cobra aliases `remove`, `hide` for UI-vocabulary discoverability): `ArchiveAgentWithResponse(ctx, agentID)`, expect 200, print the returned `archivedAt`. Help text and prompt copy state the agent reappears if its host checks in again, and that re-archiving is idempotent.

**Patterns to follow:** `system.go` tree shape; `archive` verb conventions in `experience.go`/`project.go`; `CommandsSuite` mock pattern from `test_suites_test.go`.

**Test scenarios:**
- Happy path (list): mock returns 3 agents + `latestKnownVersion="1.2.3"` → 3 rows; an agent with `isOutOfDate=true` carries `(out of date; latest 1.2.3)`.
- Edge case (list): `latestKnownVersion=""` with `isOutOfDate=false` agents → no out-of-date suffix anywhere.
- Edge case (list): zero agents → "No agents found in this org." not an empty table.
- Edge case (list `--json`): output parses as JSON and round-trips the envelope fields.
- Happy path (get): mock returns one agent with two recent-activity cards → detail view includes project names and branch when set.
- Edge case (get): `recentActivity` empty → "(none)" line, no panic.
- Error path (get 404): non-zero exit, message includes the agent ID.
- Happy path (archive): `--yes` bypasses prompt, calls `ArchiveAgentWithResponse`, prints the returned `archivedAt`.
- Edge case (archive): user declines prompt → exits without calling the client.
- Error path (archive 404): non-zero exit, message includes the agent ID.

**Verification:** `go test ./cmd/resim/commands/ -run TestAgents` (suite methods) passes; `./resim agents --help` lists `list`, `get`, `archive`.

### U2. `pool-labels queue` subcommand

**Goal:** Org-wide queue rendering matching the shipped schema.

**Requirements:** R4, R5, R6.

**Dependencies:** U4.

**Files:**
- Modify: `cmd/resim/commands/agents.go` (or split to `cmd/resim/commands/pool_labels.go`)
- Create: tests in `cmd/resim/commands/agents_test.go` (or `pool_labels_test.go` if split)

**Approach:**
- Drop the `--project-id` flag and `ProjectID` param entirely.
- Validate `--completed-since-days` client-side against the server's 1–30 range with a clear message before any API call.
- Iterate `ActiveBatches` (array) — one `ACTIVE` line per batch.
- Queued batches render their 1-based `queuePosition`.
- Integer `priority`: `(High)` `< 1000`, blank at 1000, `(Low)` `> 1000`.
- Completed footnote interpolates the `--completed-since-days` value.

**Patterns to follow:** existing grouped renderers; U1 for tree shape.

**Test scenarios:**
- Happy path: one label with two active + two queued + three completed → two ACTIVE lines, positions `Queued 1`/`Queued 2`, footnote `+3 completed in last 7 days`.
- Edge case: `--completed-since-days 14` → footnote says `last 14 days`.
- Error path: `--completed-since-days 0` (and 31) rejected client-side with a message naming the 1–30 range, no API call made.
- Edge case: priority 500 renders `(High)`; 1000 renders nothing; 2000 renders `(Low)`.
- Edge case: zero items → "No pool labels in the queue right now."
- Edge case (`--json`): raw client response.
- Edge case: queued batch with nil `queuePosition` doesn't crash the renderer.

**Verification:** suite tests pass; `./resim pool-labels queue --help` shows only `--completed-since-days` and `--json`.

### U3. Smoke E2E coverage

**Goal:** Light end-to-end checks of the new commands against staging.

**Requirements:** R1, R4.

**Dependencies:** U1, U2; staging serves the shipped endpoints (verified 2026-06-11).

**Files:** the existing `end_to_end`-tagged tests under `testing/`.

**Approach:** add a happy-path E2E that runs `agents list` and asserts exit 0 (empty-state output acceptable — staging may have no agents), and one for `pool-labels queue`. No E2E for archive — destructive.

**Test scenarios:**
- Happy path (E2E): `resim agents list` against staging exits 0.
- Happy path (E2E): `resim pool-labels queue` against staging exits 0.

**Verification:** E2E runs locally with standard staging credentials, or is structured to run in the existing CI E2E job.

---

## System-Wide Impact

- **Interaction graph:** Adds two top-level commands; no removed commands; no shared state.
- **Error propagation:** Customer API errors map to non-zero exits with friendly stderr messages, preserving convention.
- **State lifecycle risks:** Archive copy must match the UI's auto-reappear semantics (both derive from the OpenAPI description).
- **API surface parity:** list/get/archive/queue reach parity with the UI's core actions; results/result-branches/pool-label-list remain UI-only (deferred above).
- **Regenerated client blast radius:** regeneration may pull unrelated spec drift into `client.gen.go`; the existing test suite plus `go build ./...` gates regressions.
- **Unchanged invariants:** all existing CLI commands unchanged.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Regeneration pulls unrelated deployed-spec drift | Full build + existing test suite in CI (`lint-and-build.yml`: vet, build, `go test ./... -race`) |
| Drift between UI copy and CLI copy for archive | Both reference the OpenAPI description |
| Pretty-print eats structured info | `--json` always available |
| Staging E2E flaky when org has no agents | Assert exit 0 + either rows or the empty-state message |

---

## Documentation / Operational Notes

- `--help` output is the documentation; descriptions must be useful in isolation.
- PR #199 description should be updated to reflect the realignment (archive verb, unpaginated list, org-wide queue).

---

## Sources & References

- **Workspace orchestration plan:** [docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md](../../../docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md)
- **Shipped rerun changes:** PRs #3206, #3210, #3341 (listAgents unpaginated + version derivation), #3371 (org-wide queue), #3401 (result-branches + result-card metadata) — all on `rerun` `main`.
- **Spec of record:** `rerun` `api/customerapi/rerun.yml` at `origin/main`; deployed at `https://api.resim.ai/v1/openapi.yaml` (verified matching 2026-06-11).
- **In-flight branch:** `pete/wob-4131-api-client-hil-agent-status` / [PR #199](https://github.com/resim-ai/api-client/pull/199).
- Related code: `cmd/resim/commands/system.go`, `styling.go`, `client.go`, `api/generate.go`.
