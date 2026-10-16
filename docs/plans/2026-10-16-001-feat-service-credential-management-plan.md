---
title: "feat: Service credential authentication and management"
date: 2026-10-16
type: feat
artifact_contract: ce-unified-plan/v1
execution: code
product_contract_source: ce-plan-bootstrap
---

# Service credential authentication and management

Part of workspace `docs/plans/2026-10-16-002-feat-enterprise-integration-access-plan.md` ([orchestration](https://github.com/resim-ai/workspace/blob/enterprise-capability-strategy/docs/plans/2026-10-16-002-feat-enterprise-integration-access-plan.md)). Extends Task 8 of the existing rerun organization API-key plan.

## Goal Capsule

Customers can use organization API keys in automation and manage their lifecycle from the CLI using a human admin session. Extend the existing BFF contract without new provider provisioning. The worker implements/tests in this isolated worktree; root owns review and Graphite submission. Preserve existing auth modes, user files and production release behavior.

## Product Contract

- **R1:** `--api-key` and `SIGNALFLAG_API_KEY` authenticate both REST and GraphQL requests without Auth0 calls or credential-cache writes. Reject conflicting key/client/password input before requests; preserve current device/password/client flows and endpoint rules.
- **R2:** Human sessions can list key metadata; human admins create and revoke via existing BFF operations. Machine/API-key modes cannot manage keys; server remains authoritative for membership, role and trusted client admission.
- **R3:** Create takes a name, READ_ONLY/READ_WRITE and existing expiry choices, defaults to read-only and 90 days, and prints the secret once as explicit command output. Lists/errors/logs never disclose it.
- **R4:** Revoke selects by ID or unique live name and distinguishes REVOKING from REVOKED. Retry is safe. Rotation creates a separately named replacement and leaves the old key active until explicit revoke after installation/testing.
- **R5:** Generated GraphQL source is reproducible against the selected backend SDL; existing default production generation remains available. All final applicable CI passes on a branch current with main.

## Planning Contract

Use existing OAuth2 client constructors with a static bearer token, but select key mode before JWT parsing/Auth0 and bypass `Cache.Save` at shared client setup. Retain existing endpoint and GovCloud validation. Use generated operations with testable command writers/RunE; do not embed log.Fatal in new behavior. A local SDL generator option preserves production fetch defaults and does not append a duplicate schema block to a complete SDL.

`api-keys rotate --id|--name --new-name` is a replacement helper: read source metadata, preserve its access by default, create under a new name with chosen/default expiry, print once and give the explicit old-key revoke command separately. It never changes or revokes the source. Do not claim a pending cross-service revoke completed. Never automatically replay an uncertain creation; direct the user to list metadata, explicitly revoke an unusable newly created key whose secret was lost, then create a replacement. The old rotation key stays active throughout recovery.

## Implementation Units

### U1. API-key auth mode

**Requirements:** R1. **Files:** `auth/auth.go`, `auth/auth_test.go`, `cmd/signalflag/commands/client.go`, config/root command files/tests. **Approach:** Flag/env config, unambiguous mode selection, static source, no cache read/write needed for key mode. **Tests:** Both clients receive exact trimmed bearer, no Auth0 request, existing cache unchanged, conflicts rejected, ordinary modes/GovCloud regressions. **Verification:** Focused auth/command tests then build/vet/race suite.

### U2. Reproducible management client

**Requirements:** R2/R3/R5. **Files:** `bff/queries/api_keys.graphql`, generated `bff/queries.gen.go`, optional local SDL input/tests in `bff/cmd/generate.go`. **Approach:** Existing apiKeys/createApiKey/revokeApiKey API; generate against authoritative backend schema. **Tests:** SDL file vs fetched schema handling, complete schema not appended twice, clear missing/invalid file error; generated code compiles. **Verification:** Run generators; rerun without diff; no manual generated edits.

### U3. Lifecycle commands and replacement rotation

**Requirements:** R2–R4. **Depends:** U1/U2. **Files:** new `cmd/signalflag/commands/api_keys.go`, adjacent tests, root command registration, README/changelog and directory guidance. **Approach:** create/list/revoke/rotate with explicit human-mode admission and server errors, one-time secret output and pending retry guidance. **Tests:** Default/explicit values, duplicate names, selection failures, human member/admin error propagation, machine refusal before network, no cached/logged key, pending revoke, rotate creates replacement without revoke even on failure. **Verification:** Focused command tests, go fmt/vet/build and full race tests.

### U4. Shared capability acceptance

**Requirements:** R1–R5. **Depends:** U3 and available backend revision. **Files:** `testing/` integration coverage where suitable and authentication documentation. **Approach:** Synthetic HTTP tests prove client contracts; a separately identified approved environment verifies actual service lifecycle with UI and both endpoints. Existing machine-authenticated CI E2E cannot create person-only keys. **Tests:** Key reads/write denial, own/foreign org, create/list parity, rotation continuity and revocation after convergence. **Verification:** Final CI and exact runtime receipts; do not silently claim live checks from mocks.

## Verification Contract and Definition of Done

Run `go fmt ./...`, `go vet ./...`, `go build ./...`, `go test ./... -race` and existing CI E2E. Preserve untracked primary-checkout files. New sensitive behavior has request-level tests, generated source is repeatable and no plaintext is cached or logged. Final submitted SHA includes current main and required CI passes. Document unexecuted live checks separately, remove abandoned experiments, and submit through Graphite without force pushing.
