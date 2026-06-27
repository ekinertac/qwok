// Package app is the orchestration layer: it composes registry + convention +
// portless + process + state into the operations the CLI exposes. Each exported
// function is one user command (internal/cmd dispatches to these), keeping the
// CLI thin and the policy testable here.
//
// The whole package embodies the daemonless, derive-don't-store design: it reads
// intent from the registry, reads "how to run" from the project's .qwok.toml,
// launches through portless detached, and answers "is it running?" live from the
// OS + portless route store. See docs/superpowers/specs/2026-06-26-qwok-design.md.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/ekinertac/qwok/internal/convention"
	"github.com/ekinertac/qwok/internal/portless"
	"github.com/ekinertac/qwok/internal/process"
	"github.com/ekinertac/qwok/internal/registry"
	"github.com/ekinertac/qwok/internal/state"
)

// Status is a derived snapshot for `list` — never persisted, always computed.
type Status struct {
	Name    string
	Path    string
	Running bool
	URL     string
	Port    int // from portless route store, 0 if unknown/not routed
}

// AddOptions carries the explicit registration inputs (no auto-detection).
type AddOptions struct {
	Name    string
	Cwd     string
	Cmd     string
	AppPort int
	Env     map[string]string
	Force   bool
}

// Add registers an app: writes its .qwok.toml and a registry entry pointing at
// the project directory. Explicit by design — qwok never guesses the command.
func Add(o AddOptions) error {
	abs, err := filepath.Abs(o.Cwd)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return fmt.Errorf("not a directory: %s", abs)
	}
	reg, err := registry.Load()
	if err != nil {
		return err
	}
	if _, exists := reg.Get(o.Name); exists && !o.Force {
		return fmt.Errorf("app %q already registered (use --force to overwrite)", o.Name)
	}
	if err := convention.Save(abs, &convention.Convention{
		Name: o.Name, Cmd: o.Cmd, AppPort: o.AppPort, Env: o.Env,
	}); err != nil {
		return err
	}
	reg.Set(o.Name, registry.Entry{Path: abs})
	return registry.Save(reg)
}

// resolve looks up an app and loads its convention file in one step.
func resolve(name string) (registry.Entry, *convention.Convention, error) {
	reg, err := registry.Load()
	if err != nil {
		return registry.Entry{}, nil, err
	}
	entry, ok := reg.Get(name)
	if !ok {
		return registry.Entry{}, nil, fmt.Errorf("unknown app %q (try: qwok add %s ...)", name, name)
	}
	conv, err := convention.Load(entry.Path)
	if err != nil {
		return registry.Entry{}, nil, err
	}
	return entry, conv, nil
}

// Running reports whether the app's hint PID is currently alive.
func Running(name string) bool {
	return process.IsAlive(process.ReadPID(state.PIDPath(name)))
}

// LocalName resolves the app name for an argument-less command (e.g. `qwok run`
// with no name) by finding the nearest .qwok.toml from the current directory. If
// that project isn't registered yet (e.g. the .qwok.toml was committed and
// cloned elsewhere), it self-registers from the file's location so `qwok run`
// just works wherever the project lives.
func LocalName() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir, conv, err := convention.Find(cwd)
	if err != nil {
		return "", fmt.Errorf("not in a qwok project (no .qwok.toml here or above) — run 'qwok add' or name an app: 'qwok run <name>'")
	}
	reg, err := registry.Load()
	if err != nil {
		return "", err
	}
	if _, ok := reg.Get(conv.Name); !ok {
		reg.Set(conv.Name, registry.Entry{Path: dir})
		if err := registry.Save(reg); err != nil {
			return "", err
		}
	}
	return conv.Name, nil
}

// Run launches the app detached through portless. Refuses a double-start unless
// force is set (in which case it stops the old one first).
func Run(name string, force bool) (string, error) {
	entry, conv, err := resolve(name)
	if err != nil {
		return "", err
	}
	if Running(name) {
		if !force {
			return "", fmt.Errorf("app %q is already running at %s", name, portless.URL(name, entry.Path))
		}
		if err := Stop(name); err != nil {
			return "", err
		}
	}
	if _, err := exec.LookPath("portless"); err != nil {
		return "", fmt.Errorf("portless not found on PATH — install it: npm i -g portless (or brew install portless)")
	}

	// Ensure the proxy is up before detaching the app. portless can auto-start it,
	// but only with a TTY for the sudo prompt — which the detached launch lacks. So
	// we start it here in the foreground (prompting once per boot), then launch.
	if !portless.ProxyRunning() {
		fmt.Fprintln(os.Stderr, "portless proxy isn't running — starting it (may ask for your password)…")
		if err := portless.StartProxy(); err != nil {
			return "", fmt.Errorf("starting portless proxy: %w", err)
		}
	}

	cmdTokens, err := splitCommand(conv.Cmd)
	if err != nil {
		return "", fmt.Errorf("invalid cmd in %s: %w", convention.Path(entry.Path), err)
	}
	// portless <name> [--app-port N] <cmd...>
	argv := []string{"portless", name}
	if conv.AppPort > 0 {
		argv = append(argv, "--app-port", strconv.Itoa(conv.AppPort))
	}
	argv = append(argv, cmdTokens...)

	pid, err := process.Launch(entry.Path, argv, conv.Env, state.LogPath(name))
	if err != nil {
		return "", err
	}
	if err := process.WritePID(state.PIDPath(name), pid); err != nil {
		return "", err
	}
	return portless.URL(name, entry.Path), nil
}

// List returns a derived status snapshot for every registered app.
func List() ([]Status, error) {
	reg, err := registry.Load()
	if err != nil {
		return nil, err
	}
	var out []Status
	for _, name := range reg.Names() {
		path := reg.Apps[name].Path
		s := Status{Name: name, Path: path, URL: portless.URL(name, path)}
		s.Running = Running(name)
		if s.Running {
			if r, ok := portless.RouteFor(name); ok {
				s.Port = r.Port
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// Stop sends SIGTERM to the app's process group so portless can deregister its
// route and the dev server can exit cleanly.
func Stop(name string) error { return signal(name, syscall.SIGTERM) }

// Kill sends SIGKILL to the process group for a wedged process. portless's
// dead-PID filter reclaims the route on its next read.
func Kill(name string) error { return signal(name, syscall.SIGKILL) }

func signal(name string, sig syscall.Signal) error {
	pid := process.ReadPID(state.PIDPath(name))
	if !process.IsAlive(pid) {
		process.RemovePID(state.PIDPath(name))
		return fmt.Errorf("app %q is not running", name)
	}
	if err := process.SignalGroup(pid, sig); err != nil {
		return err
	}
	process.RemovePID(state.PIDPath(name))
	return nil
}

// Restart stops (if running) then runs.
func Restart(name string) (string, error) {
	if Running(name) {
		if err := Stop(name); err != nil {
			return "", err
		}
	}
	return Run(name, false)
}

// LogPath exposes the log file location for the `logs` command.
func LogPath(name string) (string, error) {
	if _, _, err := resolve(name); err != nil {
		return "", err
	}
	return state.LogPath(name), nil
}

// URL exposes the canonical URL for the `open` command.
func URL(name string) (string, error) {
	entry, _, err := resolve(name)
	if err != nil {
		return "", err
	}
	return portless.URL(name, entry.Path), nil
}

// Remove stops the app if running, drops its registry entry, and (unless
// keepFile) deletes the project's .qwok.toml.
func Remove(name string, keepFile bool) error {
	reg, err := registry.Load()
	if err != nil {
		return err
	}
	entry, ok := reg.Get(name)
	if !ok {
		return fmt.Errorf("unknown app %q", name)
	}
	if Running(name) {
		_ = Stop(name)
	}
	if !keepFile {
		_ = os.Remove(convention.Path(entry.Path))
	}
	process.RemovePID(state.PIDPath(name))
	reg.Remove(name)
	return registry.Save(reg)
}
