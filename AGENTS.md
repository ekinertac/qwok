# qwok — agent guide

A CLI that registers local dev projects and runs/lists/stops them by name in the
background, wrapping **portless** for named `https://<name>.localhost` URLs.
"pm2 for local dev apps." Full design + rationale: `docs/superpowers/specs/2026-06-26-qwok-design.md` — read it first.

## Non-negotiable architecture (decided, do not re-litigate)

- **Wrap portless, never fork/reimplement it.** Launch by shelling out to
  `portless <name> <cmd>`. Read live status from `~/.portless/routes.json`
  (structured JSON, honor `PORTLESS_STATE_DIR`). **Never** parse portless's
  human-readable CLI text — `routes.json` is the only machine contract we read.
- **Daemonless.** qwok has no long-running process. `run` detaches a process into
  its own session/process-group and returns. No supervisor, no auto-restart
  (frameworks hot-reload themselves; a supervisor would mask real crashes).
- **Status is derived, never stored.** Persist only identity/intent + a PID
  *hint*. "Is it running?" is always answered live via `kill -0` + the route
  table. No `status:` field is ever written to disk.
- **Single static Go binary.** Keep dependencies minimal.

## Layout

- `cmd/qwok` — CLI entry + subcommand dispatch.
- `internal/registry` — global registry (`~/.config/qwok/registry.toml`, `name → path`).
- `internal/convention` — per-project `.qwok.toml` (`name`, `cmd`, optional `app_port`, `env`).
- `internal/portless` — locate state dir, parse `routes.json`, build URL, report liveness.
- `internal/process` — detached launch, signal the process group, PID-hint I/O, liveness.
- `internal/state` — XDG runtime paths (logs, pid hints) under `~/.local/state/qwok`.
- `internal/app` — orchestration composing the above per command.

## Commands

`add` · `run` · `list`/`ls` · `stop` (SIGTERM group) · `kill` (SIGKILL group) ·
`restart` · `logs [-f]` · `open` · `rm [--keep-file]`.

## Conventions

- Every file starts with a file-level comment block (what it does, where it fits,
  what it depends on, non-obvious constraints). Comment the *why*, not the *what*.
- Commit messages explain *why*. No Claude/Anthropic attribution anywhere.
- Work on `master` directly.
