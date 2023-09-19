# Changelog

See also https://docs.resim.ai/changelog/ for all ReSim changes

## ReSim CLI

### Unreleased

#### Added

- The batch `create` subcommand now has an optional `--metrics-build-id` flag, used to specify a metrics build to run as part of the batch.
#### Changed

- Any list commands will now, by default, order by recency, listing newest items first

### v0.1.21 - September 7 2023

#### Added

- The batch `create` subcommand now has an optional `--github` flag. Passing this flag causes the batch ID to be output in a form suitable for use in scripts and pipelines, e.g. in GitHub Actions

### v0.1.20 - September 1 2023

#### Added

- The builds, branches and projects commands now have a `list` subcommand 

#### Changed

- Help output for commands now distinguishes between required and optional flags
- Commands that accept a `--project` flag can now be passed either a project name or ID
