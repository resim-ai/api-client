---
title: "feat: HiL Agent Status page — api-client (CLI)"
type: feat
status: active
date: 2026-05-05
origin: https://www.notion.so/345d53911af480318600c2ece3efa406
linear: https://linear.app/resim/project/agent-status-page-5dc07b3cc1e9/overview
---

# feat: HiL Agent Status page — api-client (CLI)

Part of: [docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md](../../../docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md)

## Summary

CLI parity for the new agent endpoints. Add a new top-level `resim agents` command tree (`list`, `get`, **`hide`**) and a `resim pool-labels queue` subcommand. All commands consume the regenerated customerapi client. Mirrors the existing `resim systems` pattern. (Round 1 review: action renamed from `hide` → `hide` to match the auto-clear-on-checkin semantic; agents are org-scoped, so `resim agents list` takes no `--project-id`; `resim pool-labels queue` accepts an optional `--project-id` for project-scoped views.)

---

## Problem Frame

Ops, SREs, and platform engineers script against ReSim via the CLI. They must be able to inspect the agent fleet, fetch a specific agent, soft-hide an agent, and inspect a per-pool-label queue without opening the UI. Today there is no `resim agents …` command tree at all.

---

## Requirements

- R1. `resim agents list --project-id <id> [--window-days 30] [--page-size N] [--page-token T]` — pretty table by default, `--json` for raw output.
- R2. `resim agents get --agent-id <id>` — single-agent detail view.
- R3. `resim agents hide --agent-id <id> [--yes]` — confirms by default; `--yes` skips. Soft-hide semantics noted in the help text.
- R4. `resim pool-labels queue --project-id <id> [--completed-since-days 7]` — pretty grouped output by default, `--json` for raw.
- R5. Help text on every command explains the soft-hide + auto-clear behaviour for hide and the project-scoped data window for the rest.
- R6. Output formatting consistent with existing CLI (table primitive, status colouring via existing helper).

---

## Scope Boundaries

- Not adding interactive selection (no fzf-style picker).
- Not adding a watch / follow mode (no `--watch`).
- Not introducing a new output format engine — reuse the existing one.
- Not modifying any other command tree.

### Deferred to Follow-Up Work

- Agent metrics / per-agent batch history command: out of v1.
- A `resim agents adopt` / `resim agents disable-permanently` distinction: deferred.

---

## Context & Research

### Relevant Code and Patterns

- `cmd/resim/commands/system.go` — canonical pattern for a top-level subcommand tree with create/get/list/archive variants. The "Use" / `AddCommand` shape (lines 18, 25, 31, 37, 43, 49, 167–176) is the template.
- `cmd/resim/commands/batch.go` — for the existing `--pool-labels` flag (`batchPoolLabelsKey`, lines 93, 564, 622–623). We reuse the flag-name convention.
- `cmd/resim/commands/styling.go` — output styling helpers (table, status colouring).
- `cmd/resim/commands/spinner.go` — UX consistency for the hide confirmation flow.
- `cmd/resim/commands/client.go` — the customerapi client wiring; new commands consume the regenerated client.

### Institutional Learnings

- None directly. Workspace `docs/solutions/` had no CLI command-tree learnings.

### External References

- None.

---

## Key Technical Decisions

- **One file per command tree:** new `cmd/resim/commands/agents.go` containing `agentsCmd` + subcommands; new `cmd/resim/commands/pool_labels.go` containing `poolLabelsCmd` + the `queue` subcommand.
- **Flag naming follows existing precedent.** `--project-id`, `--agent-id`, `--page-size`, `--page-token`, `--window-days`, `--completed-since-days`, `--yes`, `--json`. No camelCase, no abbreviations beyond what other commands already use.
- **Hide confirmation prompt** uses the existing pattern (yes/no read from stdin, suppressed by `--yes`). Help text explicitly says: "This soft-hides the agent. The agent will reappear in `resim agents list` if its host checks in again."
- **Table output** for `list` and `get` shows: agent name, pool labels (comma-joined), activity (ACTIVE/IDLE coloured), version (suffixed `(out of date)` when applicable), last check-in (relative), most recent batch status, most recent job status. Wide outputs trim with the existing helper.
- **Queue output** is grouped by pool label with each label as a header followed by an indented list; completed batches behind a "+N completed in last 7 days" footnote, not expanded.
- **JSON output** is the raw client response, no transformation.
- **Auth:** uses the existing `RESIM_ACCESS_TOKEN` / config-file flow; no new auth surface.
- **No backwards-compatibility concerns** — these are new commands.

---

## Open Questions

### Resolved During Planning

- *Where does `pool-labels queue` go — under `agents` or its own tree?* Its own tree (`pool-labels`) since pool labels are a first-class concept and the existing UI already lists them.
- *Confirmation prompt or `--yes` only?* Prompt by default with `--yes` to skip, matching existing destructive-action precedent.

### Deferred to Implementation

- Whether to add a `--format=table|json|yaml` selector or stick with `--json` only. Defer to whatever the closest sibling command uses.
- Exact column widths and truncation rules — defer to whatever the existing styling helper does.

---

## Output Structure

    api-client/
      cmd/
        resim/
          commands/
            agents.go                [new]
            agents_test.go           [new]
            pool_labels.go           [new]
            pool_labels_test.go      [new]

---

## Implementation Units

- U1. **`agents` command tree (list, get, hide)**

**Goal:** Implement the three agent subcommands.

**Requirements:** R1, R2, R3, R5, R6.

**Dependencies:** rerun plan U5 + U6 (REST endpoints + regenerated client).

**Files:**
- Create: `cmd/resim/commands/agents.go`
- Create: `cmd/resim/commands/agents_test.go`
- Modify: `cmd/resim/commands/commands.go` (or wherever `rootCmd` registration happens) to call `rootCmd.AddCommand(agentsCmd)`.

**Approach:**
- `var agentsCmd = &cobra.Command{Use: "agents", Short: "Manage and inspect ReSim Agents"}`.
- `listAgentsCmd` calls the regenerated `client.ListAgentsWithResponse`. Handles pagination loop when `--page-token` is unspecified (defaults to first page only) — explicit `--all` flag to follow next-page tokens.
- `getAgentCmd` calls `client.GetAgentWithResponse(ctx, agentID)`. 404 → friendly error.
- `hideAgentCmd` prompts unless `--yes`. On confirm, `client.HideAgentWithResponse(ctx, agentID)`. 204 → success message; 404 → friendly error.
- All commands honour `--json` to dump the raw client response.

**Patterns to follow:**
- `system.go` for the subcommand tree pattern.
- `batch.go` lines 564, 622–623 for the array-flag handling pattern (we don't use it here, but consistent flag naming).
- `styling.go` for table output.
- Existing destructive-action confirmation pattern (search for `Confirm` / `--yes` in current handlers).

**Test scenarios:**
- Happy path (list): mocked client returns 3 agents → command prints a 3-row table with the right columns.
- Happy path (get): mocked client returns one agent → command prints the detail view.
- Happy path (hide): `--yes` flag bypasses the prompt and calls the mutation.
- Edge case (hide): user types `n` → command exits 0 without calling the mutation.
- Edge case (list `--json`): output parses as JSON matching the OpenAPI spec.
- Edge case (list with empty result): prints "No agents found." not a header-only empty table.
- Error path (get 404): non-zero exit code, stderr message includes the agent_id.
- Error path (hide 404): same.
- Edge case (out-of-date marker): version field shows `(out of date)` suffix when `isOutOfDate=true`.

**Verification:**
- `go test ./cmd/resim/commands/ -run TestAgents` passes.
- `go build -o resim ./cmd/resim` succeeds.
- `./resim agents --help` shows the subcommand list.

---

- U2. **`pool-labels queue` subcommand**

**Goal:** New `pool-labels` tree with a single `queue` subcommand.

**Requirements:** R4, R5, R6.

**Dependencies:** rerun plan U7 (queue endpoint + regenerated client).

**Files:**
- Create: `cmd/resim/commands/pool_labels.go`
- Create: `cmd/resim/commands/pool_labels_test.go`
- Modify: `cmd/resim/commands/commands.go` — `rootCmd.AddCommand(poolLabelsCmd)`.

**Approach:**
- `var poolLabelsCmd = &cobra.Command{Use: "pool-labels", Short: "Inspect HiL pool labels and their batch queues"}`.
- `queuePoolLabelsCmd` calls `client.ListAgentPoolLabelQueueWithResponse(ctx, params)`. Renders grouped output: each label as header, active batch highlighted, queued batches numbered, completed batches summarized as a footnote.

**Patterns to follow:**
- `agents.go` from U1 for command-tree shape.
- Existing styling helpers for grouped output (search for similar grouped renderers).

**Test scenarios:**
- Happy path: mocked queue with one label, one active + two queued + three completed → output matches the documented format.
- Edge case: zero pool labels in window → "No pool labels in the last 30 days for this project."
- Edge case (`--json`): raw client response.
- Error path: 400 (missing project ID) → friendly error pointing at the flag.
- Edge case: queue position `null` for active batches doesn't crash the renderer.

**Verification:**
- `go test ./cmd/resim/commands/ -run TestPoolLabels` passes.
- `./resim pool-labels queue --help` shows the flags.

---

- U3. **Smoke E2E coverage in `.env.local` E2E**

**Goal:** Light end-to-end tests against staging that exercise the new commands.

**Requirements:** R1, R2, R3, R4.

**Dependencies:** U1, U2; staging deployment of rerun changes.

**Files:**
- Modify (or create): an existing E2E test entry under `.env.local`-driven tests.

**Approach:**
- Add a happy-path E2E that runs `resim agents list --project-id $RESIM_PROJECT_ID` and asserts non-empty output (assuming staging has at least one agent). If staging cannot guarantee an agent, skip with a clear reason.
- Add a happy-path E2E for `resim pool-labels queue --project-id $RESIM_PROJECT_ID`.
- No E2E for hide — destructive against staging, would need a dedicated test agent.

**Test scenarios:**
- Happy path (E2E): `resim agents list` against staging exits 0 with at least a header.
- Edge case: against an empty project → exits 0 with empty-state message.

**Verification:**
- E2E runs locally with the standard staging credentials.

---

## System-Wide Impact

- **Interaction graph:** Adds two top-level commands, no removed commands. No shared state.
- **Error propagation:** Customer API errors mapped to non-zero exit codes with friendly stderr messages; preserves the codebase convention.
- **State lifecycle risks:** Hide operates on the same soft-hide semantics as the UI — the help text and the prompt copy must match.
- **API surface parity:** This plan IS the parity layer between the UI and the CLI. Every UI action exists here.
- **Integration coverage:** Smoke E2E in U3.
- **Unchanged invariants:** All existing CLI commands unchanged.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Drift between UI copy ("If this host comes online again, the agent will reappear here.") and CLI copy | Both reference the same OpenAPI description verbatim |
| Pretty-print eats valuable structured info | `--json` always available |
| User confused by "queue position" being null for active batches | Renderer hides queue-position column when null; help text explains |
| Pagination footgun (user thinks they got everything but only got page 1) | First-page-only by default; loud `--all` flag for full sweep; help text explains |

---

## Documentation / Operational Notes

- `--help` output is the documentation; ensure the descriptions are useful in isolation.
- After ship: nothing for `docs/solutions/` from this side specifically; the rerun plan covers the institutional knowledge.

---

## Sources & References

- **Workspace orchestration plan:** [docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md](../../../docs/plans/2026-05-05-001-feat-hil-agent-status-page-plan.md)
- **rerun plan:** [rerun/docs/plans/2026-05-05-002-feat-hil-agent-status-page-rerun-plan.md](../../../rerun/docs/plans/2026-05-05-002-feat-hil-agent-status-page-rerun-plan.md)
- **gotta-go-fast plan:** [gotta-go-fast/docs/plans/2026-05-05-001-feat-hil-agent-status-page-ui-plan.md](../../../gotta-go-fast/docs/plans/2026-05-05-001-feat-hil-agent-status-page-ui-plan.md)
- **Origin speclet:** [Notion — HIL Agent status page](https://www.notion.so/345d53911af480318600c2ece3efa406)
- Related code: `cmd/resim/commands/system.go`, `cmd/resim/commands/batch.go`, `cmd/resim/commands/styling.go`, `cmd/resim/commands/client.go`.
