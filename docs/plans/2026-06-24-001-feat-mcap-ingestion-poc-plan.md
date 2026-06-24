# MCAP ingestion PoC — CLI commands

Branch: `andy/mcap-hackathon`
Status: WIP (PoC for hackathon)

## Premise

For the PoC we don't build a dedicated ingestion subsystem in the platform. We
piggy-back on existing primitives:

- **The parser** = the customer-supplied container that contains the
  per-mcap business logic. We model each parser as a **build** under a single
  shared "mcap-parser" **system**, on a single shared dummy **branch**.
- **An ingestion session** = a single **experience** (pointing at the MCAP
  location) plus a single **batch** linking that experience to a chosen
  parser-build.

This lets us reuse all of the existing scheduling / execution / metrics
plumbing without any new platform surface area.

## CLI surface

All three commands live in `cmd/resim/commands/mcap.go` and are wired under
`resim mcap …`.

### `resim mcap create-parser`

Required flags: `--project`, `--description`, `--image`.
Optional: `--name` (defaults to the description if not given).

Behaviour:
1. Look up the `mcap-parser` system in the project. If missing, bootstrap it:
   - Create the system (`mcap-parser`, fixed resource requests).
   - Create a no-op metrics build (`public.ecr.aws/docker/library/hello-world:latest`)
     and register it with the system. Batches against this system require *some*
     metrics build to exist; we never use it for real metrics.
2. Get-or-create the shared branch `mcap-parser-main` (project-scoped).
3. Create a new **build** on that branch under the parser system, pointing at
   the supplied image URI. The build ID is the parser ID surfaced to the user.

The bootstrap (system + metrics build + link) only happens on the very first
`create-parser` call for a project; subsequent calls just add a build.

### `resim mcap list-parsers`

Required flag: `--project`.

Resolves the `mcap-parser` system (failing if it doesn't exist yet) and prints
all builds under it as JSON via `OutputJson`.

### `resim mcap ingest`

Required flags: `--project`, `--session-name`, `--session-description`,
`--location`, `--parser` (a build UUID returned by `list-parsers`).

Behaviour:
1. Resolve the `mcap-parser` system (must exist).
2. Parse `--parser` as a build UUID.
3. Get-or-create an experience named `--session-name`, registered against the
   parser system, with `--location` as its only location, container timeout
   of 1h, and `cacheExempt=true` (ingestion is a one-shot input — we never want
   it served from a cached run).
4. Create a batch with:
   - `BuildID` = parser
   - `ExperienceIDs` = `[experienceID]`
   - `BatchName` = session name
   - `Parameters["session_name"]` = session name (passed through to the parser
     container)
   - `AssociatedAccount` / `TriggeredVia` from the standard CI helpers
5. Print experience ID and batch ID.

## Constants worth knowing

```go
mcapParserSystemName           = "mcap-parser"
mcapParserBranchName           = "mcap-parser-main"
mcapParserMetricsBuildImageURI = "public.ecr.aws/docker/library/hello-world:latest"
mcapParserContainerTimeoutSecs = int32(3600) // 1h
mcapBatchSessionNameParameter  = "session_name"
```

The branch is namespaced (`mcap-parser-main`, not `main`) so we don't collide
with other systems that already use a project-wide `main` branch.

## Tests

`cmd/resim/commands/mcap_test.go` exercises the happy paths against the
mockable API client (`api/mocks`). Cases:

- `TestMcap*CmdRequiredFlags` — each cobra command exposes the right required
  flags.
- `TestMcapCreateParserBootstrapsSystem` — system absent → creates system,
  metrics build, system-↔-metrics-build link, branch, and build; asserts the
  build is linked to the new system and that `--name` falls back to
  `--description` when unset.
- `TestMcapCreateParserReusesExistingSystem` — system and branch already
  present → only `CreateBuildForBranchWithResponse` is called.
- `TestMcapListParsers` — pages `ListBuildsForSystemWithResponse` under the
  parser system.
- `TestMcapIngestCreatesExperienceAndBatch` — happy path; asserts the
  experience body (location, system, container timeout, cache-exempt) and the
  batch body (build ID, experience IDs, batch name, `Parameters["session_name"]`).
- `TestMcapIngestReusesExistingExperience` — existing experience with the
  same name → no `CreateExperienceWithResponse`; the batch references the
  existing experience and still carries `session_name` parameter.

`log.Fatal`-paths (bad image URI, missing parser system on ingest, bad UUID)
aren't covered because they'd `os.Exit` the test process. If we ever
restructure those to return errors we can add coverage.

## Open questions / follow-ups for a real implementation

- Single-tier: every parser shares one branch and version `"1"`. If we ever
  want parser versioning we'd start cutting real branches / bumping versions.
- The no-op metrics build is a workaround for the batch path's requirement
  that the system have one. A nicer model is to make metrics builds optional
  at the system level.
- We blindly reuse any experience whose name matches `--session-name`, even
  if it isn't on the parser system. Fine for PoC, would want a tighter
  match for production.
- The hardcoded 1-hour container timeout is a guess.
