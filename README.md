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

```sh
deal-kit init                    # set up this project
deal-kit add ui-kit/data-table   # install an artifact
deal-kit status                  # what is installed, and has it drifted
deal-kit update                  # move the kit pin forward
deal-kit doctor                  # diagnose drift and broken setup
```

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

Two things stay out of scope on purpose:

- **`engram setup claude-code`**, which registers the MCP server. It changes
  permissions and further global files, so it stays a decision of its own.
- **Installing the `engram` binary.** The plugin's hooks call it; `deal-kit
  doctor` reports whether it is on PATH.

On Windows the hooks are shell scripts and need Git Bash or WSL: without one of
them the plugin installs but never runs.

## What is in here

| Path        | Contents                                                          |
| ----------- | ----------------------------------------------------------------- |
| `kit.yaml`  | Manifest of every installable artifact, its destination and deps  |
| `skills/`   | Agent skills: development conventions, PR workflow                |
| `ui-kit/`   | UI component source, copied into projects by the CLI              |
| `tool/`     | The `deal-kit` CLI (Go)                                           |

## Ownership rules

The CLI records every file it writes in the project's `deal-kit.lock`, with a hash.

- It never writes to or deletes a path that is not in the lockfile.
- If a managed file was edited locally, the sync reports it and refuses to
  overwrite. Bring the change back to this repository instead.

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
