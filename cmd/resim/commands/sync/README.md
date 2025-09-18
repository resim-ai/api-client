
# Experience Syncing

## Summary

*Note: these are developer docs, meaning that they focus on how these libraries work and not on what
this feature does or how to use it. For user documentation, please refer to the [ReSim
docs](https://docs.resim.ai/).* <!-- TODO(mikebauer) Add a link to a specific user docs page here -->

The contents of this package are intended to facilitate the easy updating of experiences based on a
generated or static config file. This config file describes the experiences the user wants to have
in their resim app, the tags and systems they want them to have, and the test suites they want with
said experiences in them. Here's an example config to make things concrete:

```lang=yaml
experiences:
    - name: scenario-survey-alpha
      description: Aerial survey over test zone
      locations:
        - s3://drone-missions/surveys/alpha-test-zone
      profile: full_stack
	  tags:
	    - regression
      environment_variables:
        - name: MAX_ALTITUDE_M
          value: "120"

    - name: new-scenario-system-check
      description: Regression validation run
      locations:
        - s3://drone-missions/system-checks/regression-1
      profile: planner_stack
	  tags:
	    - regression
		- progression
      systems:
        - mbauer_tmp_hil_repro
      environment_variables:
        - name: TEST_MODE
          value: "true"

managed_test_suites:
    - name: Basic Suite
      experiences:
        - scenario-survey-alpha
        - new-scenario-system-check

managed_experience_tags:
    - regression
    - progression
```

The user can then run:

```lang=bash
resim experiences sync \
    --project <project-name> \
	--experience-config <config/file/path.yaml>
```

And the CLI will be responsible for ensuring that the specified experiences exist and have the
specified tags. Only the tags in the `managed_experience_tags` list are removed from experiences not
explicitly listing them, and the resulting test suites contain *only* the experiences that they
list. Systems and unmanaged experience tags are never removed from experiences.

## Approach

The general approach used here separates any logic that interacts with the API from the logic which
decides what modifications are needed to reconcile the current database state with the provided
configuration. This makes it much easier to unit test the core update logic without using mocks for
everything. It makes it slightly easier to maintain as endpoints change and better endpoints become
available, and it should allow us to relatively easily support "dry running" this process in the
future. Let's look at the different steps of the sync operation.

![Syncing Data Flow](./experience-syncing.svg)

1. `loadExperienceSyncConfig()` - This is pretty simple logic which just unpacks the config yaml
   into a similarly structured go struct. The logic for this is in `config.go`.

2. `getCurrentDatabaseState()` - This function calls a variety of `List*WithResponse` endpoints to
   populate the `DataBaseState` struct. This struct reflects what experiences exist (archived and
   unarchived) what tags and systems exist along with their members, and what test suites exist with
   their ids. The core logic for this is in `ingest.go`.

3. `computeExperienceUpdates()` - This is the core of the syncing operation. It takes the
   `ExperienceSyncConfig` and the `DatabaseState` produced by the previous steps and produces an
   `ExperienceUpdates` object. This object decribes how the current experiences correspond to the
   new experiences listed in the config (matched initially by name and then by ID if an explicit ID
   is provided in the sync config), which experiences need to be added or removed from each tag or
   system, and which experiences to revise each test suite with. This matching has a lot of edge
   cases which we take care to test. This logic is distributed among `experiences.go`, `tags.go`,
   `systems.go`, and `test_suites.go`.

4. `applyUpdates()` - This is where we take the `ExperienceUpdates` and actually call the needed
   endpoints to manifest them in the app. It's a pretty simple matter of calling the right update,
   create, archive, and restore endpoints based on each pair of matched experience and tag/system
   additions and removals. This logic is in `apply.go`.

## Experience Cloning

For convenience, the `sync` command also provides the ability to fetch the current state of the
database and save it to a local config file like so:

```lang=bash
resim experiences sync \
    --project <project-name> \
	--clone \
	--experience-config <config/file/path.yaml>
```

It fetches this information anyway in the normal case, so this is a pretty easy thing to
implement. However, it does not currently do anything with test suites since we don't currently
fetch information about test suite membership when running the `sync`. This information is not
normally required to revise the test suites. We hope to support this soon.
