# deal-dev-kit

The team's shared development kit: UI components, agent skills, and development
conventions — plus the deal-kit CLI, `deal`, which installs and updates them in your
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
platform and verifies its SHA-256 checksum before installing it. It installs the
command as `deal`, adds its directory to your user PATH, and tells you if an
install from before the rename is still on PATH under the old name. Pin a
version with `DEAL_VERSION=v1.4.0` (`DEAL_KIT_VERSION` still works).

## Usage

The command is `deal`:

```sh
deal init                    # set up this project with its profile
deal install                 # set up this project with everything that applies
deal add ui-kit/data-table   # install an artifact
deal status                  # what is installed, and has it drifted
deal update                  # move the kit pin forward
deal doctor                  # diagnose drift and broken setup
```

An install from before the rename keeps whatever filename it has on disk:
`self-update` replaces the running binary in place and never renames it. To
switch to `deal`, run the installer above again and delete the old file — it
will point at it. The release assets are
still named `deal-kit_<os>_<arch>` on purpose — `self-update` builds that name
from a literal, so renaming them would leave every already-installed binary
unable to find its own update.

`init` installs the profile `kit.yaml` declares for the detected project type;
`install` installs every artifact that applies to that type, which is what the
browser's **Instalar todo** entry does — minus Engram, which is a user-global
install and never part of either. Both work on a project that has no lockfile
yet, both are additive on one that does, and running either twice is a no-op.

Every command that writes to disk prints its plan first. Pass `--dry-run` to stop
there, or `--yes` to skip confirmation in CI.

### Engram for Claude Code

Running `deal` with no subcommand opens the interactive browser, whose menu
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

### Sharing Engram memory across machines

Engram keeps its memory in a local SQLite database (`~/.engram/engram.db`), so
by default the context an agent builds up dies with the machine it was built
on. Two commands move it, and the local database always stays the source of
truth.

`engram sync` exports the memories of **the project you are standing in** — it
resolves the project from the git remote — into `.engram/` inside that
repository:

```sh
engram sync                                              # export this project
git add .engram/ && git commit -m "chore: sync engram memories"
```

`engram sync --import` reads what teammates pushed back into your database:

```sh
git pull
engram sync --import
engram sync --status                                     # local/remote/pending counts
```

The export is **project-scoped and incremental**. It writes
`.engram/manifest.json` plus one gzip chunk per sync under `.engram/chunks/`,
never a single shared file: a sync that adds one memory writes a chunk holding
only that memory, and a sync with nothing new prints `Nothing new to sync`.
Chunks are append-only and named by hash, so two people syncing the same day
produce different files and git merges them without a conflict. Importing twice
is a no-op — the chunks already applied are tracked.

Three rules keep this safe:

- **Never run `engram sync --all`.** It drops the project filter and exports
  every project in your database into whatever repository you are standing in.
- **Pass `--project` to `engram save`.** The CLI does not detect the project the
  way `engram sync` does; without the flag the memory is stored with no project
  and no sync will ever pick it up. Memories the agent writes through its MCP
  tools already carry the right project.
- **Treat `.engram/` as public to the repository.** Sync filters by project, not
  by scope, so a `scope: personal` memory saved against that project travels
  with it. Keep personal notes under a project name you never sync.

Sharing memory through git is opt-in per repository: a project that never
commits `.engram/` keeps every memory local.

## What is in here

| Path        | Contents                                                          |
| ----------- | ----------------------------------------------------------------- |
| `kit.yaml`  | Manifest of every installable artifact, its destination and deps  |
| `skills/`   | Agent skills: conventions, security, TDD, the boot gate, architecture, UX review |
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

### The UX/UI review lens

`skills/frontend/ux-review` installs as `frontend-ux-review` in **web and
mobile**. It reads a screen against eight principles — Hick's Law, Miller's Law,
white space, KISS, minimalism, Don't Make Me Think, progressive disclosure and
visual hierarchy — and reports findings anchored to `file:line` with a diff and
one of four severities.

Its thresholds are triggers to justify, never automatic rejections: a reviewer
that rejects correct work stops being read. It is the first artifact whose id
prefix is neither a project type nor `general`, because the principles cover two
of the three types and the id prefix is organisational only — `applies_to` in
`kit.yaml` is what the CLI reads.

### The boot gate

`skills/general/smoke-run` installs as `general-smoke-run` in **all three**
project types. It is the check that comes after the tests: bring the
repository's container dependencies up and wait for them to be *healthy*, build
the change, start the app in the background on a spare port, wait for the app's
own ready line, probe it once, then kill the process tree, confirm the port is
free again and stop only the compose services the gate itself started.

It exists because a green suite is evidence about units, not about startup. The
failures it catches live between the pieces — a provider that was never
registered, an env var no test reads, a circular import, the Zod config schema
that only validates at boot — and every one of them is invisible to the tests
and immediate to whoever runs the app next.

The skill never hardcodes how a repository starts: `package.json` → `scripts`
and the repository's own `docker-compose.yml` / `compose.yaml` are the source of
truth, and the ready line is read from the log rather than guessed. Booting on a
spare port instead of the project's default keeps a developer's already-running
server from turning into a bind failure that gets blamed on the change, and
teardown follows the same discipline: a stack that was already up when the gate
started is left up.

Its report is written to be impossible to fake — the exact command, the ready
line quoted verbatim, the probe's status code, the compose services started and
the teardown confirmation. A missing field is an unverified boot, not a done.
It is one `SKILL.md`, like nine of the kit's eleven skills.

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
