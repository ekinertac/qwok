// Package convention reads and writes the per-project .qwok.toml file — the
// "how this one app runs" layer from the design (§5).
//
// This file travels with the repo and is self-documenting: it names the app,
// the dev command portless should wrap, and (rarely) a pinned port or extra
// env. It deliberately has no generic port field — portless assigns the port;
// AppPort is only an override for the uncommon app that needs a fixed one.
//
// The registry (internal/registry) points name→directory; this package reads
// the .qwok.toml found in that directory. Consumed by internal/app.
package convention

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the convention file's fixed name, looked up at a project root.
const FileName = ".qwok.toml"

// App types. "web" routes through portless (free port + proxy + .localhost URL);
// "app" runs the command directly with no portless involvement, for desktop /
// non-web projects (Swift, Flutter, Electron builds, …) that have no PORT or URL.
const (
	TypeWeb = "web"
	TypeApp = "app"
)

// Convention is the parsed .qwok.toml. Name must match the registry key and the
// portless route/subdomain so http(s)://<name>.localhost is consistent for web
// apps. Type selects the run model (see the Type* constants); AppPort applies
// only to web apps.
type Convention struct {
	Name    string            `toml:"name"`
	Cmd     string            `toml:"cmd"`
	Type    string            `toml:"type,omitempty"`
	AppPort int               `toml:"app_port,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

// Path returns the .qwok.toml path for a project directory.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// Load reads and validates the .qwok.toml in dir. A missing file or empty
// required fields are errors here because, by the time we Load, the registry
// promised this app is runnable — a broken convention file is a real fault.
func Load(dir string) (*Convention, error) {
	p := Path(dir)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}
	var c Convention
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if c.Name == "" {
		return nil, fmt.Errorf("%s: missing required field 'name'", p)
	}
	if c.Cmd == "" {
		return nil, fmt.Errorf("%s: missing required field 'cmd'", p)
	}
	if c.Type == "" {
		c.Type = TypeWeb // default: backward-compatible with files written before types existed
	}
	if c.Type != TypeWeb && c.Type != TypeApp {
		return nil, fmt.Errorf("%s: invalid type %q (want %q or %q)", p, c.Type, TypeWeb, TypeApp)
	}
	return &c, nil
}

// Save writes a clean, self-documenting .qwok.toml into dir. We hand-render
// rather than use the TOML encoder so optional fields appear as commented hints
// when unset (instead of noisy `app_port = 0`), making the file read like the
// documentation it is meant to be when it travels with the repo.
func Save(dir string, c *Convention) error {
	var b strings.Builder
	b.WriteString("# qwok app definition — how this project runs (read by `qwok run`).\n")
	b.WriteString(fmt.Sprintf("name = %q\n", c.Name))
	b.WriteString(fmt.Sprintf("cmd  = %q\n", c.Cmd))
	if c.Type == TypeApp {
		// Desktop / non-web: the command runs directly, so portless-only knobs
		// (app_port) don't apply and are omitted.
		b.WriteString("type = \"app\"   # desktop/non-web: run the command directly, no portless or URL\n")
	} else {
		b.WriteString("# type = \"app\"   # set for desktop/non-web projects (run directly, no portless/URL)\n")
		if c.AppPort > 0 {
			b.WriteString(fmt.Sprintf("app_port = %d\n", c.AppPort))
		} else {
			b.WriteString("# app_port = 3000   # optional: pin a fixed port (default: portless auto-assigns)\n")
		}
	}
	if len(c.Env) > 0 {
		b.WriteString("\n[env]\n")
		keys := make([]string, 0, len(c.Env))
		for k := range c.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%s = %q\n", k, c.Env[k]))
		}
	} else {
		b.WriteString("\n# [env]\n# DEBUG = \"1\"\n")
	}
	return os.WriteFile(Path(dir), []byte(b.String()), 0o644)
}

// Exists reports whether dir already has a .qwok.toml.
func Exists(dir string) bool {
	_, err := os.Stat(Path(dir))
	return err == nil
}

// Find walks up from startDir looking for the nearest .qwok.toml, returning its
// directory and parsed contents. This is what lets `qwok run` work with no name
// from anywhere inside a project tree (mirrors how portless finds its config).
func Find(startDir string) (string, *Convention, error) {
	dir := startDir
	for {
		if Exists(dir) {
			c, err := Load(dir)
			if err != nil {
				return "", nil, err
			}
			return dir, c, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			break
		}
		dir = parent
	}
	return "", nil, fmt.Errorf("no %s found in %s or any parent directory", FileName, startDir)
}
