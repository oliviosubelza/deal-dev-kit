---
name: general-smoke-run
description: "Trigger: done, finished, ready to review, ready to merge, project init, scaffold, before opening a PR. Boot the app in the background on a spare port, probe it, tear it down — evidence it runs, not just green tests."
applies_to: [backend, web, mobile]
---

## Activation Contract

Run this when a change could break startup and the team would not see it break on their own screens: initializing or scaffolding a project, wiring modules or DI, env vars or the config schema, dependencies, the entrypoint, build config or path aliases, migrations or compose.

Skip small changes — the team has the dev environment running and sees those in real time. Skip anything that cannot reach the running app: docs, comments, a test with no production change. Say you skipped it and why.

Green tests are evidence about behaviour. Booting is the only evidence that it runs: the failures it catches live between the pieces — a provider no module registered, a config schema that only validates at boot — where no unit test looks.

## Hard Rules

- Never report a status code, ready line or health state you did not read from actual output.
- Any field you cannot fill in the report means **UNVERIFIED**, not done. Absence of observed evidence is never a pass.
- Never invent a command, compose file or service name. `package.json` → `scripts` and the repo's own compose file are the source of truth; read them.
- Never run the server in the foreground. It blocks the session.
- Never take the project's default port — the team's dev server is on it, and the bind failure gets blamed on the change. Vite needs `--strictPort`.
- Leave nothing behind: kill the process tree, confirm the port is free, and stop only the compose services this run started. A stack that was already up stays up.
- It did not boot? That is the finding. Read the log for the real error and fix it in **this** iteration, then rerun.

## Execution Steps

Step 5 runs even when 2, 3 or 4 fail.

1. Start only the declared compose services that are not already running, and wait for **healthy**, not merely started (`docker compose up -d --wait`). Record what you started — that list is what you stop.
2. Build. This is where a type error or a broken alias surfaces.
3. Start it in the background on a spare port, stdout and stderr to a log file.
4. Poll the port until it answers, then probe once: the liveness route for a backend — a 404 still proves it is listening and routing — or the dev URL for web. Do not wait on a grep for the ready line; a guessed pattern falls through to the timeout. Mobile has no port without a device, so `expo export` with no unresolved import is the whole check.
5. Tear down: kill the process tree, check the port, stop the services from step 1.

## Output Contract

One line: the exact command, the port, the compose services started (or `none`), the ready line quoted verbatim from the log, the probe's status, and that the port and the stack were released.

```
booted: npm run start:dev on 4599 · compose: postgres healthy · "Nest application successfully started" · GET /health/liveness 200 · port released · stack torn down
```

Otherwise:

```
UNVERIFIED: <what was attempted> · <the missing field and why>
```
