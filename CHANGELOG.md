# Changelog

See also https://docs.resim.ai/changelog/ for all ReSim changes

## ReSim CLI

### Unreleased

Changes in this section will be included in the next release.

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
