# Debug dashboard media upload flag (api-client)

Full cross-repo plan: rerun/bff, gotta-go-fast, api-client (this doc). See `bff`'s `docs/plans/debug-dashboard-events.md` for backend context (Workstream B) that this CLI change depends on.

## Context

`resim metrics debug` (`cmd/resim/commands/metrics.go`, `debugMetrics` command) creates an ephemeral debug dashboard from an emissions file via the `create_debug_dashboard` GraphQL mutation. The bff side (Workstream B, branch `debug-dashboard-events-media`) is adding support for uploading image/video media files alongside the emissions file, so debug dashboards can resolve media without a real batch/job. This CLI change is the client half of that: a way to actually pass media files to the mutation.

**Dependency**: only useful once bff's `debug-dashboard-events-media` branch (media_files GraphQL input) is deployed — additive/backward-compatible otherwise, safe to develop in parallel.

## Plan

1. **`cmd/resim/commands/metrics.go`** — add a repeatable flag, e.g.:
   ```go
   debugMetricsCmd.Flags().StringSlice(metricsMediaFilesKey, []string{}, "Path(s) to media files (images/videos) referenced by the emissions file. Can be specified multiple times.")
   ```
2. In `debugMetrics()`, after reading/base64-encoding the emissions file, loop over the media file paths, read + base64-encode each (reuse the existing `readFile()` helper), and build the mutation's media-files input using `filepath.Base(path)` as the `name` (must match the filename referenced in the emissions/events data).
3. **GraphQL mutation**: `bff/queries/create_debug_dashboard.graphql` — add a `$mediaFiles: [MediaFileInput!]` arg, pass through to `createDebugDashboard(...)`. Regenerate the generated client (`bff/queries.gen.go`) via whatever genqlient/codegen command this repo uses (check `Makefile`/`go generate` directives).
4. Update the `bff.CreateDebugDashboard(...)` call site in `metrics.go` to pass the new media files slice.

## Tests

Extend `cmd/resim/commands/metrics_test.go` for the new flag parsing and mutation payload construction (check for existing `debugMetrics`-adjacent tests first).

## Verification

`go build ./...`; run `resim metrics debug --emissions-file ... --media-file ...` against a dev/staging BFF (once bff Workstream B is deployed there) and confirm the printed dashboard URL shows working image/video charts.
