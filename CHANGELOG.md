# Changelog

See also https://docs.resim.ai/changelog/ for all ReSim changes

## ReSim CLI

### Unreleased

Changes in this section will be included in the next release.

#### Added

- The ReSim CLI now supports the creation of a `test suite report`, which is an evaluation workflow that 
generates a report (a set of metrics) on the performance of a given branch against that test suite.
- A report can be triggered via:
```shell
resim reports create --name "my-report" --test-suite "Nightly Regression" --branch "main" --length "4w" --metrics-build-id <UUID>
```
which will generate a report using the supplied metrics build (which must be capable of generating a report).
- Other available report commands are similar to batches: `get`, `wait`, `logs`.
- For full details of how reports work and how to generate a report, please read the main ReSim [docs](https://docs.resim.ai)
- When creating a batch, sweep, or report additional environment information will be included to describe
  where it was created from (GitHub, Gitlab, local, etc.)

### v0.3.2 - June 13 2024

#### Added

- The ReSim CLI now supports deleting a system.

### v0.3.1 - May 23 2024

#### Added

- The ReSim CLI now supports extracting a CI/CD username into an associated account field, in
  order to associate users and batches that have been kicked-off via a machine token. This is
  supported automatically for batches created within GitHub and GitLab right now, but is 
  available manually with other platforms via the `--account` flag.
- The ReSim CLI now has full support for creating and managing **test suites** :rocket:
  - A Test Suite, described in detail at [ReSim Docs](https://docs.resim.ai), provides a way to 
  specify a set of experiences and a metrics build that you intend as a repeatedly used test
  for a particular system, e.g. CI Smoke Tests. Then, for a build of that system, one simply 
  needs to run the test suite to get the results, rather than specifying the experiences each 
  time.

  This also has the benefit of decoupling the definition of a regular set of tests from its 
  running. As such, test suites are inherently versioned: updating the name, experiences, or
  metrics build creates a new revision. One can `run` a test suite at its latest revision or
  a specific revision.

  - A test suite can be created within the CLI as follows:
  ```shell
    resim suites create --project "autonomy-stack" --name "smoke tests" \
    --description "The set of smoke tests for my system" \
    --system "Perception" \
    --metrics-build "<metrics-build-id>" \
    --experiences "experience1, experience2, ..."
  ```
  - One can list the test suites: `resim suites list --project "autonomy-stack"` and get a single
  test suite, a specific revision, or all revisions with `resim suites get`
  - A revision can be created with `resim suites revise`, which takes the same parameters as creation
  - Finally, a test suite can be run with `resim suites run --suite "smoke tests" --build <build-id>`

### v0.3.0 - April 10 2024

#### Added

- The ReSim CLI now has full support for creating and managing **systems** :rocket:
  - A System, described in detail at [ReSim Docs](https://docs.resim.ai), provides a way to 
  categorize builds based on what subsystem or testing style they may be e.g. perception 
  log replay, localization closed loop, full-stack. Eeach build belongs to a given 
  system and systems define the resource requirements needed to run that particular 
  build. 

  In addition, users are encouraged to explicitly label *experiences* and *metrics builds* 
  as compatible with a given system, which enables the ReSim platform to validate test 
  batches before they run. For maximum flexibility in experiences and metrics builds, we
  enable them to be registered against many systems.

  - A system can be created within the CLI as follows:
  ```shell
    resim systems create --project "autonomy-stack" --name "perception" \
    --description "The perception subsystem of our autonomy stack" \
    --build-vcpus 4 --build-memory-mib 16384 --build-gpus 0
  ```
  - Build creation now **requires** a `--system "my-system"` flag.
  - One can list the builds for that system: `resim systems builds --project "autonomy-stack" --system "my-system"`
  - One can similarly list the experiences or metrics builds compatible with that system:
  ```shell
    resim systems experiences --project "autonomy-stack" --system "my-system"
    resim systems metrics-builds --project "autonomy-stack" --system "my-system"
  ```
  - When creating a new experience or metrics build, one can pass a `--systems "my-first-system", "my-second-system"` 
  flag to express compatibility. For ad-hoc additions, removes, we offer `resim experiences add-system`
  and `resim experiences remove-system`.
  - In future releases, `resim batches create` will validate compatibility of the test batch you intend to create.

- Cancellation - we introduce a cancel command which cancels a batch. Cancellation impacts any queued tests and lets actively running tests finish.
### v0.2.1 - March 26 2024

#### Added

- GovCloud mode - configure the CLI to work with our GovCloud environment by running `resim govcloud enable` or setting `RESIM_GOVCLOUD=true` in your environment.

### v0.2.0 - March 19 2024

#### Changed

- All ReSim Resources: experiences, experience tags, parameter sweeps, metrics builds will now live under the projects umbrella. This means that creating any resources via the CLI requires a project flag. The easiest way to achieve this is to select your expected projected using the `select project foo` feature.

### v0.1.31 - March 14 2024

#### Added

- Allowed the ability to persist project selection with `select project foo`. If set via this method, `--project` will no longer be a required flag for other commands.
- Added `batch wait` command that will continuously poll (default: 30s) until a batch reaches a final state (success, failure, cancelled, etc)
- Added `batch logs` command to list all logs associated with a batch.

#### Changed

- `project list` will list projects by name. A * will denote the active project, if set.
- Updated to cobra v1.8.0

### v0.1.30 - February 08 2024

#### Added

- Introduces the ability to specify build parameters when creating batches. For example: `batch create ... --parameter "param1:foo","param2:bar"`. This will pass a `parameters.json` file upon test execution, just as with sweeps.
- The `experience create` command now returns a list of files found in the storage location to help with validation.

#### Changed
- Fixed a bug in the reporting of error messages from commands.

### v0.1.29 - December 21 2023

#### Changed

- The CLI validates image URIs when builds and metrics builds are created.
- Fixed a bug with parsing UUIDs

### v0.1.28 - November 30 2023

#### Changed
- Fixed a bug in the handling of invalid/expired cached tokens.

### v0.1.27 - November 29 2023

#### Added
- The CLI now enables the creation, listing, and getting of parameter sweeps. Parameter sweeps enable one to pass specific values to a build to, for example, search for an optimal setting for a particular component. A parameter sweep can be created like a batch, but with the addition of either:
  - A `parameter-name` and `parameter-values` flag pair that enable a single dimension sweep with a comma separated list of values for the named parameter
  - A `grid-search-config` file can be passed, as a json list: `[{"name" : "param", "values" : [ "value1", "value2" ]}, ...]` for example. This can create a multi-dimensional grid search.

### v0.1.26 - October 26 2023

#### Added

- The CLI will now prompt for interactive login if a client ID and secret are not provided.
- The `experience-tags` command now has a `list` subcommand.

#### Changed

- The `experiences create` command now accepts an (optional) `launch-profile` parameter to explicitly select a launch profile

### v0.1.23 - September 29 2023

#### Added

- The `experiences` command now has:
  - `tag` and `untag` subcommands for tagging and untagging experiences
  - A `list` subcommand for listing experiences

- There is a new `experience-tags` command with the following subcommands:
  - `create`
  - `list-experiences` for listing the experiences with a given tag

#### Changed

- The `batch create` subcommand now supports specifying experiences and experience tags by name or ID, using the new `--experiences` and `--experience-tags` flags. (The existing flags are still supported.)

### v0.1.22 - September 19 2023

#### Added

- Metrics builds can now be created and listed with `metrics-build create/list` (ordered by recency)
- The batch `create` subcommand now has an optional `--metrics-build-id` flag, used to specify a metrics build to run as part of the batch.

#### Changed

- Any list commands will now, by default, order by recency, listing newest items first
- Commands that accept a `--branch` flag can now be passed either a branch name or ID

### v0.1.21 - September 7 2023

#### Added

- The batch `create` subcommand now has an optional `--github` flag. Passing this flag causes the batch ID to be output in a form suitable for use in scripts and pipelines, e.g. in GitHub Actions

### v0.1.20 - September 1 2023

#### Added

- The builds, branches and projects commands now have a `list` subcommand 

#### Changed

- Help output for commands now distinguishes between required and optional flags
- Commands that accept a `--project` flag can now be passed either a project name or ID
