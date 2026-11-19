# GitHub automation

`workflows/tembo-pr-setup.yml` and `workflows/stamphog-shadow.yml` call reusable
workflows in `resim-ai/workspace` at `@main`. The shared implementations and
their tests live there. Pass only the named secrets each one takes; keep the Tembo
token scoped to this repository. GitHub enforces required reviews and checks.

The stamphog shadow review's policy is `.stamphog/` on `main`; see its README.

Verify caller syntax with `actionlint` on the changed workflow. Before changing a
shared workflow, verify it and its tests in workspace.
