# deal-dev-kit

The team's shared development kit: UI components, agent skills, and development
conventions — plus `deal-kit`, the CLI that installs and updates them in your
project.

## Install the CLI

Linux, macOS and WSL:

```sh
curl -fsSL https://raw.githubusercontent.com/oliviosubelza/deal-dev-kit/main/tool/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/oliviosubelza/deal-dev-kit/main/tool/scripts/install.ps1 | iex
```

No Go toolchain required: the installer downloads a prebuilt binary for your
platform and verifies its SHA-256 checksum before installing it. Pin a version
with `DEAL_KIT_VERSION=v1.4.0`.

## Usage

The command is `deal`:

```sh
deal init                    # set up this project
deal add ui-kit/data-table   # install an artifact
deal status                  # what is installed, and has it drifted
deal update                  # move the kit pin forward
deal doctor                  # diagnose drift and broken setup
```

An install from before the rename keeps whatever filename it has on disk:
`self-update` replaces the running binary in place and never renames it. To
switch to `deal`, delete the old file and install again. The release assets are
still named `deal-kit_<os>_<arch>` on purpose — `self-update` builds that name
from a literal, so renaming them would leave every already-installed binary
unable to find its own update.

Every command that writes to disk prints its plan first. Pass `--dry-run` to stop
there, or `--yes` to skip confirmation in CI.

### Engram for Claude Code

Running `deal-kit` with no subcommand opens the interactive browser, whose menu
carries an **Engram para Claude Code** entry. It installs the
[Engram](https://github.com/Gentleman-Programming/engram) plugin — persistent
memory for the agent — into Claude Code at **user-global scope**, by shelling
out to the `claude` CLI:

```sh
claude plugin marketplace add https://github.com/Gentleman-Programming/engram.git#<tag> --scope user
claude plugin install engram@engram --scope user --yes
claude plugin enable engram@engram --scope user
```

This is the one thing deal-kit installs outside the project: it writes to the
user's global Claude Code configuration and never to the repository, so it is
not a `kit.yaml` artifact and "Instalar todo" never includes it. The screen
shows the state, the resolved path of `claude` and the exact commands; only `y`
installs. `--dry-run` and `--offline` show the plan and change nothing.

The plan also installs the **`engram` binary**, which the plugin's hooks and its
MCP server invoke: without it the plugin installs but nothing runs. It goes
first in the plan — `go install` when a Go toolchain is on PATH (upstream's own
recommendation on Windows, where unsigned prebuilt binaries trip antivirus
heuristics), otherwise the pinned release asset, verified against its published
checksum. It lands in `~/.local/bin` (`%LOCALAPPDATA%\Programs\engram` on
Windows). **deal-kit never edits PATH**: when that directory is not on it, the
run prints the exact command for the host platform (`setx` on Windows, an
`export` line on POSIX) and says Claude Code needs a restart. Running it stays
the user's call.

The build installed is the one for the environment deal-kit is running in — the
same one that resolved `claude`. A Linux `engram` is invisible to a
Windows-native Claude Code, so there is no cross-boundary install.

One thing stays out of scope on purpose: **`engram setup claude-code`**. It is
an alternative to the marketplace install, not an extra step — the plugin ships
its own `.mcp.json`, so the MCP server is registered by the install above.

On Windows the hooks are shell scripts and need Git Bash or WSL: without one of
them the plugin installs but never runs.

## What is in here

| Path        | Contents                                                          |
| ----------- | ----------------------------------------------------------------- |
| `kit.yaml`  | Manifest of every installable artifact, its destination and deps  |
| `skills/`   | Agent skills: development conventions, PR workflow                |
| `config/`   | Always-on agent rules, imported from the project's `CLAUDE.md`    |
| `ui-kit/`   | UI component source, copied into projects by the CLI              |
| `tool/`     | The `deal-kit` CLI (Go)                                           |

### The communication persona

`config/persona.md` installs to `.claude/persona.md` in all three project types.
It is not a skill: a skill loads only when the agent judges its `description` to
match the task, and a tone rule that must hold for every response cannot be
conditional.

Claude Code only loads it once the project's `CLAUDE.md` imports it, so the
artifact declares that line in `kit.yaml` and the CLI guarantees it:

```yaml
ensure_line: { file: "CLAUDE.md", line: "@.claude/persona.md" }
```

`init` adds the line if it is missing, creates `CLAUDE.md` holding it if there
is none, and does nothing if it is already there. It never rewrites the rest of
the file: `CLAUDE.md` belongs to the project, and the CLI only appends one line
to it. Running `init` repeatedly converges — the import is never duplicated.

## Ownership rules

The CLI records every file it writes in the project's `deal-kit.lock`, with a hash.

- It never writes to or deletes a path that is not in the lockfile.
- If a managed file was edited locally, the sync reports it and refuses to
  overwrite. Bring the change back to this repository instead.

A line added through `ensure_line` is recorded separately, under `lines`, and
without a hash — the CLI wrote one line of that file, not all of it. What
`status` tracks there is whether the line is still present, so editing the rest
of `CLAUDE.md` is never reported as drift; deleting the line is reported as
`FALTA IMPORT`, and the next sync puts it back.

## Versioning

Two independent tag namespaces, because the content changes far more often than
the binary:

| Tag        | Releases                                              |
| ---------- | ----------------------------------------------------- |
| `v1.2.0`   | The CLI. Cuts a binary release.                       |
| `kit-v1.4.0` | Kit content. What a project pins in `deal-kit.lock`. |

## Development

The CLI requires Go 1.24+ and is self-contained under `tool/` with its own
`go.mod`, so it can be extracted into its own repository with a subtree split if
this repository ever goes private.

```sh
cd tool && go build ./cmd/deal-kit && go test ./...
```
