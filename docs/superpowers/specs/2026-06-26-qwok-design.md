# qwok — design spec

**Date:** 2026-06-26
**Status:** approved design, pre-implementation

> File role: this is the source-of-truth design document for `qwok`. It is the
> first thing to read before touching code. It captures *why* the architecture
> is shaped the way it is (the decisions and their rationale), not just *what*
> to build. The implementation plan derives from this file; if code and this
> doc disagree, one of them is wrong on purpose and the reason should be noted.

## 1. Problem

AI coding agents changed the cadence of starting projects from ~3–4 per *month*
to ~5–6 per *week*. The bottleneck is no longer writing code; it's the ceremony
around *running* a local app you spun up days ago:

1. open a terminal
2. `cd` to the right project
3. recall the stack
4. find the right dev command (`npm run dev`? `manage.py runserver`? …)
5. remember the port
6. open the browser

At 20+ projects a month, this list is no longer holdable in your head. You need
an external registry of "what apps exist and how each one runs," plus one-command
start/stop/status. Think **`brew services` / pm2, but for your own local dev
projects** — and with named `.localhost` URLs instead of remembering ports.

## 2. What qwok is (and is not)

qwok is a **registry + process-lifecycle layer** for local dev apps. It does
**not** reinvent ports, proxies, or TLS — it drives [`portless`](https://www.npmjs.com/package/portless)
for that. The split:

| Concern | Owner |
| --- | --- |
| Pick a free port, run the HTTPS proxy, `https://<name>.localhost` routing, framework `--port`/`--host` quirks, `/etc/hosts`, CA trust | **portless** |
| Persistent registry of *all* projects (add / list / run by name from anywhere) | **qwok** |
| Background-detach the process, capture logs, start / stop / kill / status by name | **qwok** |

**Non-goals (explicitly out of scope, YAGNI):**
- No process supervisor / auto-restart. Dev code churns and frameworks already
  hot-reload (Vite HMR, Next fast-refresh, Django autoreload); stacking a
  supervisor on top is redundant and masks real crashes.
- No reboot survival (no launchd). Possible *opt-in* later; not v1.
- No stack auto-detection. Registration is explicit (see §5) — auto-detect is
  error-prone (silently wrong port), and predictability matters more than saving
  a few keystrokes.
- No TUI dashboard, no remote/LAN. (`portless --lan` exists if ever needed.)

## 3. Core principles (the decisions that shape everything)

1. **Daemonless.** qwok has no long-running process of its own. `run` detaches a
   process and exits. Nothing of ours can crash or need babysitting.
2. **Status is never stored — it is derived from reality.** A `status: running`
   field in a file is a lie the moment a process dies. qwok stores *identity and
   intent* (name → path, how to run) and a PID *hint*, but "is it running?" is
   always answered by asking the OS (`kill -0`) and reading portless's live route
   table. Truth comes from the kernel, not a file.
3. **Wrap portless, don't fork it.** portless owns the genuinely hard,
   platform-specific work. We depend only on its two *stable* surfaces: the CLI
   *input* contract (`portless <name> <cmd>`) and the structured route store
   (`~/.portless/routes.json`). We never parse human-readable CLI text. If we
   ever need deeper access, that's the moment to reconsider — from a position of
   knowing exactly which internal we need.
4. **Single static binary, written in Go.** This tool is mostly process spawning,
   signals, and process-group control — Go's strengths. portless being Node pulls
   us nowhere; we only shell out to it. (The portless npm import was evaluated:
   it exposes `RouteStore` + helpers but *not* the launch orchestration, so it
   would only have bought typed status reads — not worth pulling the tool into
   Node and losing the single binary.)

## 4. Architecture

```
┌─ qwok (the missing layer) ──────────────────────────────┐
│  registry:  name → project path        (intent only)    │
│  launcher:  detach process, capture logs, record PID    │
│  status:    derive live from OS + portless routes.json  │
└──────────────────────┬──────────────────────────────────┘
                       │ shells out to (launch) / reads (status)
┌──────────────────────▼──────────────────────────────────┐
│  portless (the engine)                                   │
│  • assigns free port (4000–4999), runs HTTPS proxy :443  │
│  • https://<name>.localhost  ← deterministic from name   │
│  • registers route on start; route auto-expires when the │
│    owning PID dies (portless filters dead PIDs on read)  │
└──────────────────────────────────────────────────────────┘
```

### Suggested Go module layout
Each unit has one job and a narrow interface so it can be understood and tested
in isolation:

- `cmd/qwok` — CLI entry + subcommand dispatch (`add/run/list/stop/kill/restart/logs/open/rm`).
- `internal/registry` — read/write the global registry (`name → path`). No status.
- `internal/convention` — read/write the per-project `.qwok.toml` (how an app runs).
- `internal/portless` — locate state dir, parse `routes.json`, build the
  `https://<name>.localhost` URL, report which names are live. The *only* place
  that knows portless's on-disk shape.
- `internal/process` — detached launch (own session/process-group), signal
  delivery to the group (SIGTERM/SIGKILL), PID-hint read/write, liveness check.
- `internal/state` — resolve runtime paths (logs, pid hints) under XDG state dir.
- `internal/app` — orchestration that composes the above for each command.

## 5. Configuration: two layers

### Global registry — *what exists* (intent only)
`~/.config/qwok/registry.toml` (honors `XDG_CONFIG_HOME`):

```toml
[apps.myapp]
path = "/Users/ekinertac/Code/myapp"

[apps.otherapp]
path = "/Users/ekinertac/Code/otherapp"
```

The index that lets `run`/`list` work from any directory. **No status field, ever.**

### Per-project convention file — *how this one runs*
`.qwok.toml` at the project root (travels with the repo, self-documenting):

```toml
name = "myapp"          # qwok app name == portless route name == subdomain
cmd  = "npm run dev"    # the dev command portless wraps
# app_port = 3000       # optional: pin a fixed port via `portless --app-port`
# env = { DEBUG = "1" } # optional extra environment for the child
```

Note there is **no generic `--port`**: portless assigns the port. `app_port` is
only an override for the rare app that needs a fixed port.

### Runtime artifacts — *not config, not status-of-record*
`~/.local/state/qwok/` (honors `XDG_STATE_HOME`):
- `logs/<name>.log` — captured stdout+stderr of the detached process.
- `pids/<name>.pid` — the PID of the detached `portless` wrapper (the process-
  group leader). A **hint** only; liveness is always re-checked against the OS.

## 6. Commands

```bash
qwok add <name> --cwd <path> --cmd "<cmd>" [--app-port N] [--env K=V ...]
                        # writes .qwok.toml in <path> + a registry entry.
                        # --cwd defaults to the current directory.

qwok run <name>         # registry → path → read .qwok.toml → launch detached
                        # through portless. Prints https://<name>.localhost.
                        # Refuses if already running unless --force (restarts).

qwok list   (alias ls)  # table of every registered app with LIVE status:
                        # name | running/stopped | url | port (from routes.json)

qwok stop <name>        # SIGTERM the process group (portless deregisters,
                        # dev server flushes/exits cleanly).

qwok kill <name>        # SIGKILL the process group (for wedged processes;
                        # portless's dead-PID filter reclaims the route).

qwok restart <name>     # stop, then run.

qwok logs <name> [-f]   # print the captured log file; -f follows (tail).

qwok open <name>        # open https://<name>.localhost in the browser.

qwok rm <name> [--keep-file]
                        # stop if running, drop the registry entry; deletes
                        # .qwok.toml unless --keep-file.
```

## 7. How `run` works (the daemonless core)

1. Look up `<name>` → project path in the registry.
2. Read `.qwok.toml` at that path for `cmd` (+ optional `app_port`, `env`).
3. Open `logs/<name>.log` for append; set it as the child's stdout+stderr.
4. Launch **`portless <name> <cmd…>`** (adding `--app-port N` if pinned) as a
   **detached process in its own session/process group**
   (`SysProcAttr{Setsid:true, Setpgid:true}`), with the child's env = parent env
   + any `.qwok.toml` `env`. `Start()` it, do **not** `Wait()` — qwok returns
   immediately; the child reparents to launchd and survives qwok's exit.
5. Write the child's PID (the group-leader PID) to `pids/<name>.pid`.
6. Print `https://<name>.localhost`.

portless assigns the port, starts/auto-starts the proxy, and registers the route.
The process tree is `qwok → portless (group leader) → dev server`; signalling the
**process group** therefore reaches both portless and the dev server.

## 8. Status, derived (never stored)

For `list` / status of `<name>`:
- **running** ⇔ the PID in `pids/<name>.pid` exists and is alive (`kill -0`).
- **stopped** otherwise (missing or dead PID hint).
- **Enrichment:** read `~/.portless/routes.json` (a JSON array of
  `{hostname, port, pid}`). If `<name>.localhost` is present *and its PID is
  alive*, show the port; the URL is always `https://<name>.localhost`
  (deterministic from the name, so a port is never required to build it).
- Honor `PORTLESS_STATE_DIR` when locating `routes.json`.

This means qwok and portless never disagree for long: both treat a dead PID as a
dead app. Even a hard `kill` self-heals — portless filters the stale route on its
next read, and qwok's status check sees the dead PID immediately.

## 9. Error handling

- **portless not installed** → detected on `run`; clear message with an install
  hint (`npm i -g portless` / `brew install portless`). qwok does not bundle it.
- **proxy needs sudo (port 443)** → portless auto-elevates/auto-starts the proxy;
  qwok surfaces portless's output. First run may prompt for sudo — documented,
  not swallowed.
- **name already in registry** on `add` → error unless `--force` (overwrite).
- **path does not exist** / **no `.qwok.toml`** → actionable error naming the path.
- **already running** on `run` → print status + URL; `--force` to restart.
- **stale PID hint** (process died) → `list` shows *stopped*; next `run` overwrites.
- **`routes.json` missing/empty** → treated as "no routes"; status falls back to
  the PID hint alone.

## 10. Testing strategy

- **Unit:** registry read/write; `.qwok.toml` parse/write; `routes.json` parsing;
  status derivation with mocked PID-liveness + route table; URL formatting;
  XDG path resolution.
- **Integration:** launch a trivial long-lived command (e.g. a tiny HTTP server)
  through a *stub* `portless` (a script that writes a route into `routes.json`
  and sleeps), then assert: PID hint written, status = running, log captured,
  `stop` delivers SIGTERM to the whole group, `kill` delivers SIGKILL, status
  returns to stopped, registry/route reconciled.
- **First-build validation item (carried from design):** manually confirm that a
  real `portless` deregisters its route on SIGTERM of the detached wrapper. The
  source already shows dead-PID filtering makes this self-healing regardless, so
  this is a confirmation, not a load-bearing assumption. If it ever isn't clean,
  `stop` gains an explicit `portless` route-cleanup fallback.

## 11. Open questions / future (not v1)

- Opt-in reboot survival via launchd (`qwok enable <name>`?).
- `qwok run` with no name from inside a project dir → infer from local `.qwok.toml`.
- Optional `qwok status <name>` JSON output for scripting/agents.
