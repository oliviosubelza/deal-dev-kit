---
name: deal-kit-maintenance
description: "Trigger: deal-dev-kit, deal-kit CLI, kit.yaml, adding a skill or component to the kit, cutting a kit or CLI release. Rules for changing this repository safely."
license: Apache-2.0
metadata:
  author: "deal-team"
  version: "1.0"
---

## Activation Contract

Load when changing anything in `deal-dev-kit`: the `deal-kit` CLI under `tool/`, `kit.yaml`, a `SKILL.md` under `skills/`, a component under `ui-kit/`, or when tagging a release.

Read `HANDOFF.md` first. It carries current state, open work, settled decisions, and the traps already hit.

## Hard Rules

- Never document a symbol, path, or export without grepping the source for it. Three export names in the original catalog skill were wrong.
- Never write a convention the coordinator's briefing does not state. No gap notes either: state what exists, nothing more.
- Test local kit changes with `--kit-dir /mnt/c/SoftwareDevelopment/deal-dev-kit`. Without it the CLI fetches GitHub and your change is invisible.
- Validate `kit.yaml` with `go test ./internal/kit/ -count=1`. Go caches across changes to files outside the package.
- Commit before tagging. `kit-v0.2.0` shipped six `TODO` skills because it did not.
- The TUI renders `internal/plan` and never decides what a sync does.
- Only `y` applies a plan. A key that navigates must never also commit.
- Add a command to the table in `cmd/deal-kit/commands.go`. Dispatch and name recognition both derive from it.

## Decision Gates

| Change | Tag |
|---|---|
| Anything under `tool/` | `v*` — builds binaries |
| `skills/`, `ui-kit/`, `kit.yaml` | `kit-v*` — what projects pin |
| Both | Both tags |

| New code | Goes in |
|---|---|
| Decides what a sync does | `internal/plan` |
| Reads or writes the project | `internal/cli` |
| Renders | `internal/tui` or `internal/cli/render.go` |

## Execution Steps

1. Read `HANDOFF.md`.
2. Make the change. Add a test that fails without it.
3. `gofmt -l . && go vet ./... && go test ./... -count=1`.
4. Regenerate goldens with `go test ./internal/tui/ -update`, then rerun without it.
5. Exercise the real binary against the test project before claiming it works.
6. Update `HANDOFF.md` when state, pending work, or a decision changes.

## Output Contract

Report files changed, commands run with their result, the tag namespace the change belongs to, and any `HANDOFF.md` update.

## References

- `HANDOFF.md` — state, pending work, decisions and traps.
- `kit.yaml` — artifact manifest and destinations.
- `tool/internal/kit/repo_manifest_test.go` — validates the real manifest in CI.
