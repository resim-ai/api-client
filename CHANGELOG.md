# Changelog

## ReSim CLI

### Unreleased

Changes in this section will be included in the next release.

### v0.15.0 - April 10, 2025

- Build create/update now accepts a `--name` flag to set the build's name. If `--name` is not provided but `--description` is, the description value will be used for the build's name (matching current behavior). The `--description` flag is used to set the build's description when provided alongside `--name`. In a future release, `--name` will become a required parameter and `--description` will no longer be used to set the build's name, but will instead only be used to set its description.

### v0.14.0 - April 8 2025

#### Changed

- It is now possible to pass `pool-labels` to the `resim ingest` command to support log ingestion via the ReSim agent.

### v0.13.0 - April 7 2025

#### Added

- The `logs download` command now supports downloading a single log by providing the log name.

#### Changed

- The `logs` commands now support batch logs.

### v0.12.0 - March 25 2025

#### Added

- A new `metrics sync` command has been added, for syncing your metrics configuration with ReSim. This command is for
  our next version of metrics management, and is currently unused.

### v0.11.0 - March 20 2025

#### Added

- A `sweeps cancel` command enables the cancellation of parameter sweeps.

#### Changed

- The `resim ingest` command now supports ingesting multiple logs at the same time, either via a `--log` repeated flag, or `--config-file` using a yaml file. For more information see the [ReSim docs](https://docs.resim.ai/guides/log-ingest)

### v0.10.0 - March 14 2025

#### Added

- A new `download` subcommand has been added to the `logs` command. This allows you to download logs for a given job from the ReSim platform to your local machine.

### v0.9.0 - March 13 2025

#### Changed

- The `resim ingest` command now supports using a custom log ingestion build ID, to allow you to preprocess a log. It can be run with `--build-id`. This is mutually exclusive with the `--branch` and `--version` flags.

### v0.8.0 - March 12 2025

#### Added

- A new `ingest` command now supports ingesting existing logs -- from field testing, for example -- into the ReSim platform and running metrics on them. It works by importing a log with a given name and cloud storage location then running metrics on it. For more information see the [ReSim docs](https://docs.resim.ai/guides/log-ingest) for more details.
- The test suite run command now supports a `--metrics-build-override` flag to allow you to take advantage of the test suite grouping, but test out a new metrics build.

### v0.7.0 - February 26 2025

#### Changed

- The `suites run` and `batches create` commands now support using a separate delimiter for parameters: e.g. "key=value" to support cases where a colon is a natural part of the key e.g. `namespace::param=value`

### v0.6.0 - February 19 2025

#### Added

- The test suite `revise` command now supports the `--show-on-summary` flag, which can be used to specify whether the latest results of a test suite should be displayed on the overview dashboard.

### v0.5.0 - February 18 2025

#### Added

- The ReSim Platform now supports container timeouts, which can be set when creating or updating an experience. The intention is to allow users to specify a timeout for the container that is running the experience. If the container runs longer than this, it will be terminated.
- The ReSim CLI now supports updating experiences via `experiences update`. An experience can be updated with a new name, description, location, and container timeout.

### v0.4.1 - February 13 2025

#### Added

- The ReSim CLI now shows help when no subcommands are provided.

#### Fixed

- The CLI no longer fails to select a new project if the config file is present.

### v0.4.0 - January 24 2025

#### Added

- The ReSim CLI now supports updating builds via `builds update`. A build can be updated with a new branch ID and a new description. The version and image must be static.

### v0.3.11 - December 10 2024

#### Added

- The ReSim CLI now supports an allowable failure percent for batches and test suites (`--allowable-failure-percent`). This is a percentage (0-100) that determines the maximum percentage of tests that can have an execution error and have aggregate metrics be computed and consider the batch successfully completed. If not supplied, ReSim defaults to 0, which means that the batch will only be considered successful if all tests complete successfully.

### v0.3.10 - November 26 2024

#### Added

- The ReSim CLI now supports specifying a batch name when running test suites or ad-hoc batches (via `--batch-name`).

#### Changed

- The default behavior for systems and projects is not deletion, but archival, so the CLI has been updated to reflect this.

### v0.3.9 - October 30 2024

#### Added

- The ReSim CLI now supports outputting a Slack Webhook payload from the `batch get` command. Providing
  the `--slack` flag will replace the current JSON output with a formatted payload.

### v0.3.8 - October 4 2024

#### Added

- The ReSim CLI now supports using specific `pool labels` for creating batches, and running test suites and parameter sweeps. Supplying a pool label forces the ReSim Orchestrator to run that batch using an external runner that is compatible with this label combination.
- The ReSim CLI now supports creation of test suites that use the 'show on summary' flag, which enables the latest test suite and its reports to be viewed on an overview dashboard on the ReSim Web App.
- The ReSim CLI now supports updating systems by making changes to the resource requirements.

### v0.3.7 - August 7 2024

### Changed

- For the first authentication, the returned token doesn't have any permissions assigned because the server-side assignment is performed asynchronously. The CLI will now check whether the token has any permissions and retry once if there are none present.

### v0.3.6 - July 26 2024

#### Added

- The ReSim CLI now supports username and password authentication. Set `RESIM_USERNAME` and `RESIM_PASSWORD` in your environment, for example in your CI workflow's variables, to use this authentication method. If you have an existing client ID and client secret, you can continue to use those for authentication (provided as `RESIM_CLIENT_ID` and `RESIM_CLIENT_SECRET`).

### v0.3.5 - July 16 2024

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

#### Changed

- Any historic use of 'job' has been replaced with 'test', which is the more accurate external-facing term for the elements of a test batch.

### v0.3.4 - June 13 2024

#### Added

- The ReSim CLI now supports deleting a system.

### v0.3.3 - June 3 2024

#### Changed

- Experience location validation updated to support non-S3 experience locations.

### v0.3.2 - May 23 2024

#### Changed

- Fixes a small bug where CI/CD environment variable are not successfully retrieved.
- "main" branches are created with a specific MAIN type.

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

  In addition, users are encouraged to explicitly label _experiences_ and _metrics builds_
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

- `project list` will list projects by name. A \* will denote the active project, if set.
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
