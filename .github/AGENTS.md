# GitHub automation

`workflows/tembo-pr-setup.yml` deterministically configures same-repository,
bot-authored Tembo PRs targeting `main`. Rerun adds `ready`; all repositories
enable native squash auto-merge after draft. Required reviews and checks remain
GitHub's responsibility. This privileged workflow must never check out or execute
PR code. The Deployment Bot token is scoped to the current repository.

Verify workflow syntax with `actionlint .github/workflows/tembo-pr-setup.yml` and
metadata behavior with `node --test .github/tembo-pr-setup.test.cjs`.
