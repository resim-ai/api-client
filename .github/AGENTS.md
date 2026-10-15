# GitHub automation

`workflows/tembo-pr-setup.yml` calls the reusable Tembo PR workflow in
`resim-ai/workspace` at a pinned commit. The shared implementation and Node tests
live there. Pass only its two named App secrets; keep the temporary token scoped
to this repository. GitHub enforces required reviews and checks.

Verify caller syntax with `actionlint .github/workflows/tembo-pr-setup.yml`.
Before updating the shared commit, verify the workflow and tests in workspace.
