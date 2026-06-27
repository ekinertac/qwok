# qwok — agent guide

A CLI that registers local dev projects and runs/lists/stops them by name in the
background, wrapping **portless** for named `https://<name>.localhost` URLs.
"pm2 for local dev apps." Full design + rationale: `docs/superpowers/specs/2026-06-26-qwok-design.md` — read it first.

## Non-negotiable architecture (decided, do not re-litigate)

- **Wrap portless, never fork/reimplement it.** Launch by shelling out to
  `portless <name> <cmd>`. Read live status from `~/.portless/routes.json`
  (structured JSON, honor `PORTLESS_STATE_DIR`). Get an app's URL from
  `portless get <name>` (a documented single-value command — the URL reflects the
  live proxy scheme/port/TLD, which is NOT knowable by construction: e.g. a
  `--no-tls` proxy serves http on :80, not https on :443). **Never** parse
  portless's human-readable *table* output (`portless list`); `routes.json` and
  the single-value `get` are the only machine contracts we depend on.
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

`run` with no name infers the app from the nearest `.qwok.toml` (walks up from
cwd, self-registers if needed). `run` also ensures the portless proxy is up
first — it starts it in the foreground (so portless can sudo-prompt once) when
the proxy pid is dead, since the detached launch has no TTY for that prompt.

## Conventions

- Every file starts with a file-level comment block (what it does, where it fits,
  what it depends on, non-obvious constraints). Comment the *why*, not the *what*.
- Commit messages explain *why*. No Claude/Anthropic attribution anywhere.
- Work on `master` directly.
