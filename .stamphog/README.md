# .stamphog

Configuration for [stamphog](https://github.com/PostHog/posthog/tree/master/products/stamphog), PostHog's PR-approval engine.
`.github/workflows/stamphog-shadow.yml` calls workspace's shared workflow, which runs it on every PR a person opens and records the verdict in the job summary and a `stamphog-<pr>-<sha>` artifact.
It runs in shadow: it posts nothing and approves nothing.

The engine puts every PR in one of three places:

- **A person reviews it** when a file matches a `deny` category in `policy.yml`, or the PR is over 1,200 lines or 50 files.
- **Approved unread** (tier `T0`) when every file matches `allow`, or every file is a test, and none is new.
- **The model reviews it** otherwise, against the shipped review guidance plus `steering.md`.

`policy.yml` only states what differs from the engine's [default policy](https://github.com/PostHog/posthog/blob/master/products/stamphog/packages/pr-approval-agent/defaults/policy.yml).
Deny patterns are regexes, while allow patterns are plain substrings.
A PR is judged by the policy on `main`, so a change to this directory takes effect once it merges, and always needs a person to review it.

The engine version and how it runs live in workspace's `.github/workflows/stamphog-shadow.yml`.
