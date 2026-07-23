# CLI Topic Archival Support Implementation Plan (WOB-4284)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `resim metrics sync` archive a dropped topic on confirmation (`--allow-topic-archival`), after automatically previewing the impact (rows hidden, chart count, affected dashboards) and refusing to proceed without the flag.

**Architecture:** Two new/changed GraphQL operations (`previewTopicArchivals` query, `allowTopicArchival` arg on `updateMetricsConfig`) generated via genqlient from the BFF schema landed in rerun#3630. `syncMetrics` calls `PreviewTopicArchivals` first; if it returns any entries, prints an impact table and requires `--allow-topic-archival` (via `log.Fatal` otherwise, matching this CLI's existing fail-fast convention — no interactive prompts exist elsewhere in this codebase). `SyncMetricsConfig` threads the flag's value into `UpdateMetricsConfig`.

**Tech Stack:** Go, Cobra (CLI framework), genqlient (GraphQL codegen), testify/mock (test doubles).

## Context

rerun PR #3630 (WOB-4284, merged/deploying) adds topic archival to the BFF: dropping a topic from a `metrics.yml` sync now requires an explicit `allowTopicArchival: true` on `updateMetricsConfig` (independent of the never-wired-up `allowBreakingChanges`), and a new `previewTopicArchivals(branchId, config)` query returns, for each topic a pending sync would drop, `{topicName, rowsToBeHidden, chartCount, dashboards: [{id, name}]}`.

Decisions already made (do not revisit):
- **Scope**: topic archival only. `allowBreakingChanges` remains unwired in this CLI (pre-existing state, unrelated to this effort).
- **UX**: automatic pre-check inside `sync` (not a separate command) — mirrors how `validate` already runs the same checks as `sync` without persisting.

## Global Constraints

- Repo root for all paths below: `~/resim/api-client`.
- Go module `github.com/resim-ai/api-client`. Run `go build ./...` and `go test ./...` before every commit.
- `bff/queries.gen.go` is genqlient-generated from `bff/schema.graphql` + `bff/queries/*.graphql` per `bff/genqlient.yaml` — never hand-edit it; edit the `.graphql` operation files and regenerate.
- No `Co-Authored-By` in commit messages (repo convention, matching `rerun`).
- Mirror existing error-surfacing convention: extract a clean message from a GraphQL error via `errors.As(err, &gqlerror.List)` and use `gqlErrs[0].Message`, exactly as `validateMetricsSetExists` does in `cmd/resim/commands/utils.go:231-248` — do not invent a new error-shape convention.
- Mirror existing test convention: `mockGraphQLClient` (defined in `cmd/resim/commands/metrics_test.go:57-68`) + `testify/mock`, matched by `req.OpName == "..."`. Follow `validate_metrics_set_test.go` and `metrics_test.go`'s existing `TestValidateMetricsConfig_*` tests for style.

---

## Task 1: Add the `previewTopicArchivals` query and regenerate the GraphQL client

**Files:**
- Create: `bff/queries/preview_topic_archivals.graphql`
- Modify: `bff/queries/update_metrics_config.graphql`
- Regenerate: `bff/queries.gen.go` (generated — do not hand-edit)

**Interfaces:**
- Produces: `bff.PreviewTopicArchivals(ctx, client, branchId string, config string) (*bff.PreviewTopicArchivalsResponse, error)`, where `resp.PreviewTopicArchivals` is `[]bff.PreviewTopicArchivalsPreviewTopicArchivalsTopicArchivalPreview` (genqlient's generated nested-type naming — exact name confirmed at Step 3 below once generated) with fields `TopicName string`, `RowsToBeHidden int`, `ChartCount int`, `Dashboards []{Id string; Name string}`.
- Produces: `bff.UpdateMetricsConfig(ctx, client, projectId, config string, templateFiles []MetricsTemplate, branch string, allowTopicArchival bool) (*bff.UpdateMetricsConfigResponse, error)` — same function, one new trailing parameter.

- [ ] **Step 1: Add the new operation file**

```graphql
query PreviewTopicArchivals($branchId: String!, $config: String!) {
    previewTopicArchivals(branchId: $branchId, config: $config) {
        topicName
        rowsToBeHidden
        chartCount
        dashboards {
            id
            name
        }
    }
}
```

Save as `bff/queries/preview_topic_archivals.graphql`.

- [ ] **Step 2: Add `allowTopicArchival` to the existing mutation**

Update `bff/queries/update_metrics_config.graphql` (currently a single line):

```graphql
mutation UpdateMetricsConfig($projectId: String!, $config: String!, $templateFiles: [MetricsTemplate!]!, $branch: String, $allowTopicArchival: Boolean) {
    updateMetricsConfig(projectId: $projectId, config: $config, templateFiles: $templateFiles, branch: $branch, allowTopicArchival: $allowTopicArchival)
}
```

- [ ] **Step 3: Regenerate `bff/queries.gen.go`**

This repo's `go:generate` target (`bff/cmd/generate.go`) does a **live introspection fetch** against `GRAPHQL_API_ENDPOINT` (default `https://bff.resim.ai/graphql`) — it will only pick up the new schema once rerun#3630 is deployed somewhere reachable. Two ways to run this step depending on what's available when you pick up this task:

**Option A — against a deployed dev/staging BFF (preferred once available):**
```sh
cd bff
GRAPHQL_API_ENDPOINT=<dev-bff-graphql-url> go generate ./...
```

**Option B — offline, using the schema already known from the merged rerun PR (use this if no reachable deployment exists yet):**
```sh
# From the rerun checkout with PR #3630 merged (adjust path as needed):
cp /Users/minfante/resim/rerun/bff/priv/schema.graphql /Users/minfante/resim/api-client/bff/schema.graphql
cd /Users/minfante/resim/api-client/bff
go run github.com/Khan/genqlient
rm schema.graphql
```
(`go run github.com/Khan/genqlient` is genqlient's own standard invocation — it reads `bff/genqlient.yaml` directly, with no network fetch, unlike this repo's custom `cmd/generate.go` wrapper. This produces byte-for-byte the same `queries.gen.go` output Option A would, since both read the same `genqlient.yaml`.)

Either way, confirm the diff:
```sh
git diff bff/queries.gen.go
```
Expected: new `PreviewTopicArchivals` function + response/nested types, and `UpdateMetricsConfig`'s signature gains an `allowTopicArchival bool` parameter (genqlient maps a nullable `Boolean` arg to a Go `bool`, matching the existing `branch string` — a nullable `String` — pattern already in this function). No unrelated diff.

- [ ] **Step 4: Build to confirm the generated code compiles**

Run: `cd bff && go build ./...`
Expected: succeeds (no other code references the new symbols yet, so this only checks the generated file itself is valid Go).

- [ ] **Step 5: Commit**

```bash
git add bff/queries/preview_topic_archivals.graphql bff/queries/update_metrics_config.graphql bff/queries.gen.go
git commit -m "Add previewTopicArchivals query and allowTopicArchival arg to the generated BFF client"
```

---

## Task 2: `--allow-topic-archival` flag + automatic preview in `sync`

**Files:**
- Modify: `cmd/resim/commands/metrics.go`
- Modify: `cmd/resim/commands/utils.go`
- Test: `cmd/resim/commands/metrics_test.go`

**Interfaces:**
- Consumes: `bff.PreviewTopicArchivals`, `bff.UpdateMetricsConfig` (Task 1).
- Produces: `SyncMetricsConfig(projectID, branchID uuid.UUID, configPaths []string, templatesPath string, allowTopicArchival bool, verbose bool) error` — note the new `allowTopicArchival bool` parameter inserted before `verbose` (matches this codebase's convention of trailing `verbose bool` as the last param on this function family).
- Produces: `previewTopicArchivalImpact(branchID uuid.UUID, config string) error` — new helper in `utils.go`, called from `SyncMetricsConfig` before the `UpdateMetricsConfig` call.

- [ ] **Step 1: Write the failing tests**

`SyncMetricsConfig` calls `Client.GetBranchForProjectWithResponse` (the REST/OpenAPI client, package var `Client api.ClientWithResponsesInterface`) before it calls anything on `BffClient`. There is no existing direct-call test for `SyncMetricsConfig`, but the REST client already has a generated mock used elsewhere (`cmd/resim/commands/batch_test.go`'s `CommandsSuite`, via `mockapiclient "github.com/resim-ai/api-client/api/mocks"` and `mockapiclient.NewClientWithResponsesInterface(t)`). `metrics_test.go` itself uses a plain-function style (not a suite) for its `BffClient`-only tests via `withMockBffClient` (defined in `validate_metrics_set_test.go:29-34`) — add an equivalent `withMockClient` helper for the REST client here so both mocks compose in plain-function tests.

Add to `cmd/resim/commands/metrics_test.go` (add `mockapiclient "github.com/resim-ai/api-client/api/mocks"`, `"github.com/resim-ai/api-client/api"`, and `"net/http"` to the file's imports):

```go
func isPreviewTopicArchivalsRequest(req *graphql.Request) bool {
	return req.OpName == "PreviewTopicArchivals"
}

func isUpdateMetricsConfigRequest(req *graphql.Request) bool {
	return req.OpName == "UpdateMetricsConfig"
}

func withPreviewTopicArchivalsResponse(previews []bff.PreviewTopicArchivalsPreviewTopicArchivalsTopicArchivalPreview) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.PreviewTopicArchivalsResponse)
		data.PreviewTopicArchivals = previews
	}
}

func withUpdateMetricsConfigSuccess() func(args mock.Arguments) {
	return func(args mock.Arguments) {
		resp := args.Get(2).(*graphql.Response)
		data := resp.Data.(*bff.UpdateMetricsConfigResponse)
		data.UpdateMetricsConfig = "Success"
	}
}

// withMockClient stubs the package-level REST Client with a mock that returns a
// named branch for any GetBranchForProjectWithResponse call — SyncMetricsConfig's
// first call, needed before it ever touches BffClient.
func withMockClient(t *testing.T, branchName string) *mockapiclient.ClientWithResponsesInterface {
	t.Helper()
	mockClient := mockapiclient.NewClientWithResponsesInterface(t)
	mockClient.On("GetBranchForProjectWithResponse", mock.Anything, mock.Anything, mock.Anything).
		Return(&api.GetBranchForProjectResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Branch{Name: branchName},
		}, nil)

	origClient := Client
	Client = mockClient
	t.Cleanup(func() { Client = origClient })
	return mockClient
}

func TestSyncMetricsConfig_NoTopicsArchived_SucceedsWithoutFlag(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Run(withPreviewTopicArchivalsResponse(nil)).
		Return(nil).Once()
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Run(withUpdateMetricsConfigSuccess()).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", false, false)
	assert.NoError(t, err)
	mockBff.AssertExpectations(t)
}

func TestSyncMetricsConfig_TopicsWouldBeArchived_RejectsWithoutFlag(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	previews := []bff.PreviewTopicArchivalsPreviewTopicArchivalsTopicArchivalPreview{
		{TopicName: "old_topic", RowsToBeHidden: 42, ChartCount: 2},
	}
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Run(withPreviewTopicArchivalsResponse(previews)).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "old_topic")
	assert.Contains(t, err.Error(), "--allow-topic-archival")
	// UpdateMetricsConfig must never be called when the archival preview is rejected.
	mockBff.AssertNotCalled(t, "MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything)
}

func TestSyncMetricsConfig_TopicsWouldBeArchived_ProceedsWithFlag(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	previews := []bff.PreviewTopicArchivalsPreviewTopicArchivalsTopicArchivalPreview{
		{TopicName: "old_topic", RowsToBeHidden: 42, ChartCount: 2},
	}
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Run(withPreviewTopicArchivalsResponse(previews)).
		Return(nil).Once()
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Run(withUpdateMetricsConfigSuccess()).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", true, false)
	assert.NoError(t, err)
	mockBff.AssertExpectations(t)
}

func TestSyncMetricsConfig_PreviewTransportErrorSoftFails(t *testing.T) {
	withMockClient(t, "main")
	mockBff := new(mockGraphQLClient)
	withMockBffClient(t, mockBff)

	// A non-GraphQL (transport) error on the preview must not block sync — sync proceeds
	// and the BFF's own gate is the backstop, mirroring validateMetricsSetExists's soft-fail.
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isPreviewTopicArchivalsRequest), mock.Anything).
		Return(fmt.Errorf("bff unavailable")).Once()
	mockBff.On("MakeRequest", mock.Anything, mock.MatchedBy(isUpdateMetricsConfigRequest), mock.Anything).
		Run(withUpdateMetricsConfigSuccess()).
		Return(nil).Once()

	err := SyncMetricsConfig(uuid.New(), uuid.New(), []string{"testdata/config.yml"}, "testdata/templates", false, false)
	assert.NoError(t, err)
	mockBff.AssertExpectations(t)
}

func TestSyncMetricsCmdHasAllowTopicArchivalFlag(t *testing.T) {
	flag := syncMetricsCmd.Flags().Lookup("allow-topic-archival")
	assert.NotNil(t, flag, "--allow-topic-archival flag should exist on syncMetricsCmd")
	assert.Equal(t, "false", flag.DefValue)
}
```

The test fixtures reference `testdata/config.yml` and `testdata/templates` — check whether `cmd/resim/commands/testdata/` already has a minimal metrics config fixture used by other tests in this package (search for `"testdata/"` in this directory's other `_test.go` files) and reuse it rather than inventing a new one; `prepareMetricsConfig`/`readTemplates` (called inside `SyncMetricsConfig`) read real files from disk, so these paths must exist and parse successfully for the tests to reach the mocked GraphQL calls at all.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api-client && go test ./cmd/resim/commands/... -run TestSyncMetricsConfig -v`
Expected: compile failure (new flag/param/symbols don't exist yet) or failing assertions once it compiles.

- [ ] **Step 3: Add the `--allow-topic-archival` flag**

In `cmd/resim/commands/metrics.go`, add a new const near the others (currently lines 46-56):

```go
	metricsAllowTopicArchivalKey = "allow-topic-archival"
```

In `init()`, add right after the `syncMetricsCmd.Flags().String(metricsTemplatesPathKey, ...)` line (currently line 64):

```go
	syncMetricsCmd.Flags().Bool(metricsAllowTopicArchivalKey, false, "Confirm archiving any topics this sync would drop. Without this flag, a sync that drops a topic is rejected after previewing the impact.")
```

- [ ] **Step 4: Implement the preview + gating logic**

In `cmd/resim/commands/utils.go`, add near `validateMetricsSetExists` (which already establishes the exact error-extraction convention to mirror):

```go
// previewTopicArchivalImpact calls the BFF's previewTopicArchivals query and, if any
// topics would be archived by this config, returns a descriptive error listing the
// impact and requiring --allow-topic-archival. A transport (non-GraphQL) error is
// logged and swallowed — the BFF's own allowTopicArchival gate is the backstop, same
// as validateMetricsSetExists's soft-fail-on-unreachable behavior.
func previewTopicArchivalImpact(branchID uuid.UUID, configB64 string) error {
	resp, err := bff.PreviewTopicArchivals(context.Background(), BffClient, branchID.String(), configB64)
	if err != nil {
		var gqlErrs gqlerror.List
		if errors.As(err, &gqlErrs) && len(gqlErrs) > 0 {
			return errors.New(gqlErrs[0].Message)
		}
		log.Printf("warning: could not preview topic archival impact, continuing: %v", err)
		return nil
	}

	if len(resp.PreviewTopicArchivals) == 0 {
		return nil
	}

	fmt.Println("This sync would archive the following topics:")
	for _, p := range resp.PreviewTopicArchivals {
		fmt.Printf("  - %s: %d row(s) would be hidden, %d chart(s) reference it\n", p.TopicName, p.RowsToBeHidden, p.ChartCount)
		for _, d := range p.Dashboards {
			fmt.Printf("      affects dashboard %q (%s)\n", d.Name, d.Id)
		}
	}

	topicNames := make([]string, 0, len(resp.PreviewTopicArchivals))
	for _, p := range resp.PreviewTopicArchivals {
		topicNames = append(topicNames, p.TopicName)
	}
	return fmt.Errorf(
		"sync would archive topic(s) %s; re-run with --allow-topic-archival to confirm",
		strings.Join(topicNames, ", "),
	)
}
```

Update `SyncMetricsConfig` (currently `utils.go:403-447`) to accept the new parameter and call the preview before `UpdateMetricsConfig`:

```go
func SyncMetricsConfig(projectID uuid.UUID, branchID uuid.UUID, configPaths []string, templatesPath string, allowTopicArchival bool, verbose bool) error {
	branch, err := Client.GetBranchForProjectWithResponse(context.Background(), projectID, branchID)
	if err != nil {
		log.Fatal("unable to retrieve branch associated with the build being run:", err)
	}
	branchName := branch.JSON200.Name
	if branchName == "" {
		log.Fatal("branch has no name associated with it")
	}

	configB64, err := prepareMetricsConfig(configPaths, verbose)
	if err != nil {
		return err
	}

	if !allowTopicArchival {
		if err := previewTopicArchivalImpact(branchID, configB64); err != nil {
			return err
		}
	}

	templates, err := readTemplates(templatesPath, verbose)
	if err != nil {
		return err
	}

	_, err = bff.UpdateMetricsConfig(
		context.Background(),
		BffClient,
		projectID.String(),
		configB64,
		templates,
		branchName, //TODO: We should use branch ids instead of names
		allowTopicArchival,
	)
	if err != nil {
		var gqlErrs gqlerror.List
		if errors.As(err, &gqlErrs) && len(gqlErrs) > 0 {
			return errors.New(gqlErrs[0].Message)
		}
		return fmt.Errorf("failed to sync metrics config: %w", err)
	}

	if verbose {
		fmt.Print("Successfully synced metrics config")
		if len(templates) > 0 {
			fmt.Println(", and the following templates:")
		} else {
			fmt.Println(".")
		}
		for _, t := range templates {
			fmt.Printf("\t%s\n", t.Name)
		}
	}
	return nil
}
```

Note: when `allowTopicArchival` is true, the preview is intentionally skipped (the user already confirmed) — do not call it unconditionally, or every `--allow-topic-archival` sync would print the impact table for a decision already made. Add `"strings"` and `"github.com/vektah/gqlparser/v2/gqlerror"` to `utils.go`'s imports if not already present (check the existing import block first — `gqlerror` may already be imported for `validateMetricsSetExists`; `strings` is very likely already imported elsewhere in this large file, check before adding a duplicate).

Update `syncMetrics` in `metrics.go` (currently lines 102-118) to read and pass the new flag:

```go
func syncMetrics(cmd *cobra.Command, args []string) {
	verboseMode := viper.GetBool(verboseKey)
	projectID := getProjectID(Client, viper.GetString(metricsProjectKey))
	branchName := viper.GetString(metricsBranchNameKey)
	branchID := getBranchID(Client, projectID, branchName, true)
	allowTopicArchival := viper.GetBool(metricsAllowTopicArchivalKey)

	// Prefer --metrics-config-path if explicitly set; fall back to deprecated --config-path
	configPaths := viper.GetStringSlice(metricsConfigPathAliasKey)
	if cmd.Flags().Changed(metricsConfigPathKey) && !cmd.Flags().Changed(metricsConfigPathAliasKey) {
		configPaths = viper.GetStringSlice(metricsConfigPathKey)
	}
	templatesPath := viper.GetString(metricsTemplatesPathKey)

	if err := SyncMetricsConfig(projectID, branchID, configPaths, templatesPath, allowTopicArchival, verboseMode); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api-client && go test ./cmd/resim/commands/... -run TestSyncMetricsConfig -v`
Expected: PASS

- [ ] **Step 6: Update the four other callers of `SyncMetricsConfig`**

`SyncMetricsConfig`'s signature gained a new parameter, so every existing call site must be updated. There are exactly four others (confirmed via `grep -rn "SyncMetricsConfig(" --include="*.go" .` during planning — re-run it yourself to confirm no new call site was added since), each currently passing `false` as the last (`verbose`) argument and none of them exposing any topic-archival-relevant flag of their own, so each becomes `false` for the new `allowTopicArchival` parameter (inserted before the existing trailing `false`):

- `cmd/resim/commands/test_suites.go:651`: `SyncMetricsConfig(projectID, branchID, metricsConfigPaths, metricsTemplatesPath, false)` → `SyncMetricsConfig(projectID, branchID, metricsConfigPaths, metricsTemplatesPath, false, false)`
- `cmd/resim/commands/ingest.go:316`: same change.
- `cmd/resim/commands/batch.go:829`: same change.
- `cmd/resim/commands/sweep.go:288`: same change.

- [ ] **Step 7: Run the full package test suite to check for regressions**

Run: `cd api-client && go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/resim/commands/metrics.go cmd/resim/commands/utils.go cmd/resim/commands/metrics_test.go
git commit -m "Add --allow-topic-archival flag with automatic impact preview to metrics sync"
```

---

## Task 3: Finalize the changelog entry

**Files:**
- Modify: `CHANGELOG.md`

**Interfaces:** none (documentation only).

- [ ] **Step 1: Replace the placeholder "Unreleased" heads-up note**

The `Unreleased` section currently reads (added ahead of this implementation, as a heads-up):

```markdown
### Unreleased

- Heads up: the BFF (rerun#3630, WOB-4284) is adding topic archival for metrics configs. Once deployed, dropping a topic from `metrics sync` will require a new `allowTopicArchival` flag (independent of the not-yet-wired-up `allowBreakingChanges`) — CLI support (flag + a `previewTopicArchivals`-backed pre-check) is planned as a follow-up, not yet implemented here.
```

Replace it with a real entry reflecting what Tasks 1-2 actually shipped, in this repo's established changelog voice (see `### v0.60.0` and other recent entries for tone/format — one or two sentences, present tense "Adds ..."):

```markdown
### Unreleased

- `resim metrics sync` now previews and confirms topic archival: if a config drop would archive a topic, sync prints the affected row count, chart count, and dashboards, then refuses to proceed unless `--allow-topic-archival` is set.
```

(Leave the version number/date as `Unreleased` — this repo's release process presumably assigns the actual version number at release time; do not invent one. If this repo's convention turns out to require a version number even for unreleased work, per a release script or CI check you discover while doing this task, follow that instead and note the discovery in your commit.)

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "Finalize changelog entry for topic archival sync support"
```

---

## Task 4: End-to-end test

**Files:**
- Create: `testing/.resim/metrics/config-no-topics.resim.yml`
- Modify: `testing/end_to_end_test.go`

**Interfaces:** none new — this exercises the real compiled CLI binary against a real (dev/staging) BFF, per this repo's existing e2e convention (`TestMetricsSync` in `testing/end_to_end_test.go:6480-6522`, `syncMetrics`/`validateMetrics` helpers at lines 359-388).

**This task requires a reachable BFF with rerun#3630 deployed to actually run** (same dependency as Task 1's Option A codegen path) — write and self-review the code per this task, but treat "e2e test passes" as verified only once such an environment is reachable; note in your report if you could only confirm the test compiles, not that it passes.

- [ ] **Step 1: Add the second fixture config**

The existing e2e fixture `testing/.resim/metrics/config.resim.yml` (used by `TestMetricsSync`'s default `syncMetrics()` calls) declares topic `ok`, metric `Average Speed` (which queries `ok`), and metrics set `woot`. To archive topic `ok`, a *replacement* config must drop it (and anything referencing it) while still being a schema-valid config with no other breaking changes. Create `testing/.resim/metrics/config-no-topics.resim.yml`:

```yaml
version: 1
topics: {}
```

- [ ] **Step 2: Add a sync helper that supports a custom config path and the new flag**

`syncMetrics()` (end_to_end_test.go:359-383) always uses the CLI's default config path and has no way to pass extra flags. Add a sibling helper right after it, following the same structure:

```go
func syncMetricsWithConfig(projectName string, branch string, configPath string, allowTopicArchival bool, username string, password string) []CommandBuilder {
	metricsCommand := CommandBuilder{Command: "metrics"}

	flags := []Flag{
		{Name: "--project", Value: projectName},
		{Name: "--metrics-config-path", Value: configPath},
	}
	if branch != "" {
		flags = append(flags, Flag{Name: "--branch", Value: branch})
	}
	if allowTopicArchival {
		flags = append(flags, Flag{Name: "--allow-topic-archival"})
	}
	if username != "" {
		flags = append(flags, Flag{Name: "--username", Value: username})
	}
	if password != "" {
		flags = append(flags, Flag{Name: "--password", Value: password})
	}

	syncCommand := CommandBuilder{Command: "sync", Flags: flags}

	return []CommandBuilder{metricsCommand, syncCommand}
}
```

`Flag{Name: "--allow-topic-archival"}` (no `Value` field set) matches the exact pattern `--verbose` already uses two lines above it in `syncMetrics()` — `foldFlags` (line 284-291) appends `flag.Value`'s zero value (`""`) as a separate arg unconditionally, and this already works for `--verbose` in the passing `TestMetricsSync` suite today, so the same shape is safe to reuse here without further verification.

- [ ] **Step 3: Add the subtests**

Add inside `TestMetricsSync` (end_to_end_test.go:6480-6522), after the existing `t.Run("ValidatesMetricsConfig", ...)` block (these must run after `SyncsMetricsConfig` so topic `ok` already exists on the branch):

```go
	t.Run("RejectsTopicArchivalWithoutFlag", func(t *testing.T) {
		absConfigPath, err := filepath.Abs(".resim/metrics/config-no-topics.resim.yml")
		req.NoError(err)

		output := s.runCommand(ts, syncMetricsWithConfig(projectIDString, "", absConfigPath, false, username, password), true)
		ts.Contains(output.StdOut, "This sync would archive the following topics")
		ts.Contains(output.StdOut, "ok")
		ts.Contains(output.StdErr, "--allow-topic-archival")
	})

	t.Run("ArchivesTopicWithFlag", func(t *testing.T) {
		absConfigPath, err := filepath.Abs(".resim/metrics/config-no-topics.resim.yml")
		req.NoError(err)

		output := s.runCommand(ts, syncMetricsWithConfig(projectIDString, "", absConfigPath, true, username, password), false)
		ts.Equal("", output.StdErr)
	})
```

`filepath` must already be imported in this file (confirm — `TestMetricsDebug` above already uses `filepath.Abs`, so it should be); if not, add `"path/filepath"` to the import block.

- [ ] **Step 4: Run the e2e test (only if a reachable dev/staging BFF with rerun#3630 is available)**

Run whatever this repo's existing e2e invocation convention is for a single test (check `README.md`/`Makefile`/CI workflow for the exact `go test` invocation and required env vars — `TestMetricsSync` requires a live API endpoint via env vars this codebase's e2e suite already expects, e.g. `RESIM_API_URL`/`RESIM_USERNAME`/`RESIM_PASSWORD` or similar — do not guess these, find how CI runs this suite and mirror it exactly):

```sh
cd testing && go test -run TestMetricsSync -v ./...
```

Expected: all four subtests (`SyncsMetricsConfig`, `ValidatesMetricsConfig`, `RejectsTopicArchivalWithoutFlag`, `ArchivesTopicWithFlag`) pass.

If no reachable environment exists yet when you pick up this task, run `go vet ./testing/...` and `go build ./testing/...` to confirm the new code at least compiles and type-checks, note this limitation clearly in your report, and do not claim the test passes.

- [ ] **Step 5: Commit**

```bash
git add testing/.resim/metrics/config-no-topics.resim.yml testing/end_to_end_test.go
git commit -m "Add e2e coverage for topic archival sync gating"
```

---

## Verification

- `go build ./...` and `go test ./...` clean at the repo root.
- Manual, once a dev BFF with rerun#3630 deployed is reachable:
  1. `resim metrics sync --project <p> --branch <b> --metrics-config-path <config-with-topic>` — succeeds.
  2. Edit the config to drop that topic, run `resim metrics sync` again without the flag — prints the impact table and exits non-zero with a message mentioning `--allow-topic-archival`.
  3. Re-run with `--allow-topic-archival` — succeeds.
  4. Confirm via `resim` or the BFF directly that querying the dropped topic's data now excludes pre-archival rows (cross-check against the rerun-side manual verification in `rerun`'s `docs/plans/2026-07-16-topic-archival-metrics-config.md`).
