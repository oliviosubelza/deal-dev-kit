# deal-dev-kit

The team's shared development kit — UI components, agent skills and development
conventions — plus `deal`, the CLI that installs and updates them in your
project.

## Install the CLI

```sh
# Linux, macOS, WSL
curl -fsSL https://raw.githubusercontent.com/oliviosubelza/deal-dev-kit/main/tool/scripts/install.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/oliviosubelza/deal-dev-kit/main/tool/scripts/install.ps1 | iex
```

No Go toolchain needed: the installer downloads a prebuilt binary, verifies its
SHA-256 checksum, installs it as `deal` and adds it to your PATH. Pin a version
with `DEAL_VERSION=v1.4.0`.

## Commands

```sh
deal                         # interactive browser
deal init                    # set up this project with its profile
deal install                 # set up this project with everything that applies
deal add ui-kit/data-table   # install a specific artifact
deal status                  # what is installed, and has it drifted
deal update                  # move the kit pin forward
deal doctor                  # diagnose drift and broken setup
deal self-update             # replace this binary with the latest release
```

**`init` vs `install`** — `init` installs the profile `kit.yaml` declares for
the detected project type, a curated subset. `install` installs *every*
artifact that applies to that type, the same selection as the browser's
**Instalar todo**. Both work on a project with no lockfile yet, both are
additive on one that has it, and running either twice changes nothing.

Every command that writes prints its plan first. `--dry-run` stops there,
`--yes` skips confirmation for CI, `--type` overrides project detection.

## Engram: persistent agent memory

Engram gives Claude Code memory that survives across sessions. The interactive
browser carries an **Engram para Claude Code** entry that installs the
[plugin](https://github.com/Gentleman-Programming/engram) and the `engram`
binary it depends on.

This is the one thing deal-kit installs **outside** the project — it writes to
your user-global Claude Code config — so it is not a `kit.yaml` artifact and
neither `install` nor "Instalar todo" ever includes it. Only `y` applies it.

> **Windows:** the plugin's hooks are shell scripts and need Git Bash or WSL.
> Without one, the plugin installs but never runs.

### Sharing memory across machines

Engram's memory lives in a local SQLite database (`~/.engram/engram.db`), so by
default the context an agent builds up dies with that machine. Two commands
move it, and the local database stays the source of truth:

```sh
engram sync            # export THIS project's memories into .engram/
git add .engram/ && git commit -m "chore: sync engram memories"

git pull               # on another machine, or a teammate's
engram sync --import
```

The export is **project-scoped** — it resolves the project from the git remote —
and **incremental**: it writes `.engram/manifest.json` plus one gzip chunk per
sync, so adding one memory writes a chunk holding only that memory. Chunks are
append-only and named by hash, so two people syncing the same day never
conflict in git. Importing twice is a no-op.

Three rules keep this safe:

- **Never run `engram sync --all`** — it drops the project filter and exports
  every project in your database into whatever repository you are standing in.
- **Pass `--project` to `engram save`** — the CLI does not detect the project
  the way `sync` does, and a memory saved without it is invisible to sync.
  Memories the agent writes through its MCP tools already carry the right one.
- **Treat `.engram/` as public to the repository** — sync filters by project,
  not by scope, so a `scope: personal` memory travels with it.

Sharing through git is opt-in per repository: a project that never commits
`.engram/` keeps every memory local.

## What is in here

| Path       | Contents                                                        |
| ---------- | --------------------------------------------------------------- |
| `kit.yaml` | Manifest of every installable artifact, its destination and deps |
| `skills/`  | Agent skills: conventions, security, TDD, boot gate, architecture, UX review |
| `config/`  | Always-on agent rules, imported from the project's `CLAUDE.md`   |
| `ui-kit/`  | UI component source, copied into projects by the CLI             |
| `tool/`    | The `deal` CLI (Go)                                              |

Each skill documents itself in its own `SKILL.md`. Design decisions and the
traps already hit live in [`HANDOFF.md`](HANDOFF.md).

## Ownership rules

The CLI records every file it writes in `deal-kit.lock`, with a hash.

- It never writes to or deletes a path that is not in the lockfile.
- If a managed file was edited locally, the sync reports it and **refuses to
  overwrite**. Bring the change back to this repository instead.

Two destinations belong to the project rather than the kit, so they are tracked
without a hash and merged instead of rewritten:

| Mechanism     | Target                   | What is tracked                          |
| ------------- | ------------------------ | ---------------------------------------- |
| `ensure_line` | `CLAUDE.md`              | One line is still present                |
| `ensure_json` | `.claude/settings.json`  | The declared keys still hold their value |

Editing the rest of those files is never reported as drift. Removing what the
kit ensured is, and the next sync puts it back — unless the project set a
different value on purpose, which blocks instead of being overwritten.

## Versioning

Two independent tag namespaces, because content changes far more often than the
binary:

| Tag          | Releases                                             |
| ------------ | ---------------------------------------------------- |
| `v1.2.0`     | The CLI. Cuts a binary release.                      |
| `kit-v1.4.0` | Kit content. What a project pins in `deal-kit.lock`. |

A change touching both needs both tags, and **`v*` must ship first**: an older
binary silently ignores manifest features it does not understand.

## Development

The CLI needs Go 1.24+ and is self-contained under `tool/` with its own
`go.mod`, so it can be split into its own repository if this one ever goes
private.

```sh
cd tool && go build ./cmd/deal-kit && go test ./...
```
