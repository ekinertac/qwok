// Package state resolves all on-disk locations qwok uses, and nothing else.
//
// It is the single source of truth for *where* things live so no other package
// hardcodes a path. Two distinct trees, per the design (see
// docs/superpowers/specs/2026-06-26-qwok-design.md §5):
//
//   - config tree (~/.config/qwok): the registry of intent. Honors XDG_CONFIG_HOME.
//   - state tree  (~/.local/state/qwok): runtime artifacts (logs, PID hints).
//     Honors XDG_STATE_HOME.
//
// Keeping path logic here (rather than in registry/process/app) means the XDG
// resolution and directory creation are tested and reasoned about in one place.
package state

import (
	"os"
	"path/filepath"
)

// configDir is where intent lives (the registry). It must persist; it is not
// runtime state.
func configDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "qwok")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "qwok")
}

// stateDir is where disposable runtime artifacts live (logs, PID hints). Losing
// it loses nothing of record, because status is always re-derived from the OS.
func stateDir() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "qwok")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "qwok")
}

// RegistryPath is the global name→path index.
func RegistryPath() string { return filepath.Join(configDir(), "registry.toml") }

// LogPath is where a running app's captured stdout+stderr is appended.
func LogPath(name string) string { return filepath.Join(stateDir(), "logs", name+".log") }

// PIDPath holds the hint PID of the detached portless wrapper for an app. It is
// only ever a hint: liveness is re-checked against the OS, never trusted blindly.
func PIDPath(name string) string { return filepath.Join(stateDir(), "pids", name+".pid") }

// EnsureDirs creates every directory qwok writes into. Safe to call repeatedly.
func EnsureDirs() error {
	for _, d := range []string{
		configDir(),
		filepath.Join(stateDir(), "logs"),
		filepath.Join(stateDir(), "pids"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
