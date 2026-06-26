# qwok

Run your local dev apps by name. A tiny, daemonless CLI that registers your
projects and starts/stops them in the background, wrapping
[portless](https://github.com/portless/portless) so each one gets a named
`*.localhost` URL instead of a port you have to remember.

Think `pm2`, but for the local dev servers you spin up all week.

```console
$ qwok add myapp --cmd "npm run dev"
registered "myapp" -> /Users/you/Code/myapp

$ qwok run myapp
started "myapp" -> http://myapp.localhost

$ qwok list
NAME    STATUS   URL                     PORT
myapp   running  http://myapp.localhost  4317
api     stopped  http://api.localhost

$ qwok stop myapp
stopped "myapp"
```

## Why

AI coding agents turned "3–4 new projects a month" into "5–6 a week." You can't
hold that many in your head, and the friction was never *running* an app — it
was the ceremony around it: open a terminal, `cd` to the right place, recall the
stack, find the dev command, remember the port, open the browser.

qwok makes that one named command. It keeps a registry of what exists and how
each app runs, and lets portless handle ports, the proxy, and `.localhost` URLs.

## Install

```sh
brew install ekinertac/tap/qwok
```

Builds from source (Go), so there's no notarization step.

qwok needs **portless** at runtime:

```sh
npm install -g portless     # or: brew install portless
```

Start the portless proxy once (it persists across reboots of your session):

```sh
sudo portless proxy start              # https on :443
# or, no TLS:
sudo portless proxy start --no-tls     # http on :80
```

## Usage

```sh
qwok add <name> --cmd "<command>" [--cwd <dir>] [--app-port N] [--env K=V ...]
qwok run <name> [--force]
qwok list                       # alias: ls
qwok stop <name>                # graceful SIGTERM
qwok kill <name>                # forceful SIGKILL
qwok restart <name>
qwok logs <name> [-f]
qwok open <name>                # open the app's URL in your browser
qwok rm <name> [--keep-file]
```

`add` writes a `.qwok.toml` in the project and an entry in the global registry:

```toml
# .qwok.toml — how this project runs (read by `qwok run`).
name = "myapp"
cmd  = "npm run dev"
# app_port = 3000   # optional: pin a fixed port (default: portless auto-assigns)

# [env]
# DEBUG = "1"
```

### A note on ports

portless assigns each app a free port and routes its `.localhost` URL to it.
Frameworks that read `PORT` (Next, Express, Nuxt…) or that portless knows about
(Vite, Astro, Angular, Expo…) work with no extra config. For a bare command that
ignores `PORT`, either pin one with `--app-port` or reference `$PORT` yourself:

```sh
qwok add docs --cmd "sh -c 'python3 -m http.server \$PORT'"
```

## How it works

qwok is a thin registry + lifecycle layer; portless does the hard part.

- **Daemonless.** `run` detaches the dev server into its own process group and
  exits. Nothing of qwok's stays resident.
- **No stored status.** "Is it running?" is always derived live from the OS
  (`kill -0`) plus portless's route table — never written to a file that could
  go stale. `stop` SIGTERMs the group (portless deregisters the route); `kill`
  SIGKILLs it.
- **Wraps portless, doesn't reinvent it.** Launches via `portless <name> <cmd>`,
  reads live status from `~/.portless/routes.json`, and gets each URL from
  `portless get <name>` so the scheme/port match however you started the proxy.

Two config layers: a global registry at `~/.config/qwok/registry.toml` (just
`name → path`), and the per-project `.qwok.toml` (how that one app runs).

## License

MIT
