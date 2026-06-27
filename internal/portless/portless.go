// Package portless is the ONLY place in qwok that knows portless's on-disk shape.
//
// Per the design's core principle (§3.3), qwok wraps portless and depends solely
// on its two *stable* surfaces: the CLI input contract (handled in internal/process
// by invoking `portless <name> <cmd>`) and the structured route store read here.
// We never parse portless's human-readable CLI output — only ~/.portless/routes.json,
// a JSON array of {hostname, port, pid} that portless maintains and self-cleans by
// filtering dead PIDs. Honoring PORTLESS_STATE_DIR keeps us correct if the user
// relocates portless's state.
//
// If portless ever changes this file's shape, this is the single file to update.
// Consumed by internal/app for status enrichment.
package portless

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Route mirrors one entry of portless's routes.json. Extra portless fields
// (tailscale/ngrok metadata) are ignored — we only need these three.
type Route struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Pid      int    `json:"pid"`
}

// stateDir resolves portless's state directory the same way portless does.
func stateDir() string {
	if d := os.Getenv("PORTLESS_STATE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".portless")
}

// RoutesPath is the route store qwok reads for live status.
func RoutesPath() string { return filepath.Join(stateDir(), "routes.json") }

// Routes parses the route store. A missing or empty file means "nothing routed"
// — a normal state (no apps running), not an error.
func Routes() ([]Route, error) {
	data, err := os.ReadFile(RoutesPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, nil
	}
	var routes []Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, err
	}
	return routes, nil
}

// RouteFor returns the route registered for an app name, if present. portless
// stores the full hostname (e.g. "myapp.localhost"); we match on the subdomain
// so we stay correct regardless of the configured TLD.
func RouteFor(name string) (Route, bool) {
	routes, err := Routes()
	if err != nil {
		return Route{}, false
	}
	for _, r := range routes {
		if subdomain(r.Hostname) == name {
			return r, true
		}
	}
	return Route{}, false
}

// subdomain strips the TLD label, returning the leading name. "myapp.localhost"
// -> "myapp"; "api.myapp.localhost" -> "api.myapp".
func subdomain(hostname string) string {
	if i := strings.LastIndex(hostname, "."); i >= 0 {
		return hostname[:i]
	}
	return hostname
}

// ProxyRunning reports whether portless's reverse-proxy daemon is up, by reading
// its pid file and checking the process is alive — the same derive-from-the-OS
// approach qwok uses for apps, so we never trust a stale pid file.
func ProxyRunning() bool {
	data, err := os.ReadFile(filepath.Join(stateDir(), "proxy.pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	// signal 0 probes existence; EPERM means it exists but isn't ours (still up).
	if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
		return true
	}
	return false
}

// StartProxy starts the portless proxy in the FOREGROUND with our terminal
// attached, so portless can auto-elevate via sudo (prompting the user once) and
// so any output is visible. It passes no flags: portless reuses the proxy config
// (port/TLS/TLD) from its last run, so the proxy comes back in whatever mode the
// user last chose without qwok having to store that preference. portless
// daemonizes the proxy and returns. Proxy chatter goes to stderr to keep qwok's
// stdout clean for its own structured output.
func StartProxy() error {
	cmd := exec.Command("portless", "proxy", "start")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// URL returns the canonical browser URL for an app.
//
// We do NOT construct it ourselves: the scheme (http vs https), port, and TLD
// all depend on how the user started the proxy (e.g. `sudo portless proxy start
// --no-tls` serves http on :80, not https on :443). Hardcoding https was a real
// bug — it pointed users at a port nothing was serving. Instead we ask portless
// via `portless get <name>`, a documented single-value command meant for
// programmatic wiring; its output reflects the live proxy config (and any git
// worktree prefix when evaluated from dir). This is a stable contract, distinct
// from scraping `portless list` table text, which we still never do.
//
// dir should be the app's project directory so worktree detection matches what
// `portless run` produced. On any failure (portless absent, etc.) we fall back
// to a best-effort URL derived from the persisted proxy port.
func URL(name, dir string) string {
	cmd := exec.Command("portless", "get", name)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err == nil {
		if u := strings.TrimSpace(string(out)); u != "" {
			return u
		}
	}
	return fallbackURL(name)
}

// fallbackURL derives a best-effort URL from the persisted proxy port when
// portless cannot be queried. Scheme is inferred from the port: 80 → http,
// 443 or unknown → https, any other port → http with the port appended (portless
// only uses non-default ports in its no-TLS/sudo-less fallbacks). Cosmetic only;
// the real path is `portless get` above.
func fallbackURL(name string) string {
	host := name + ".localhost"
	port := ""
	if data, err := os.ReadFile(filepath.Join(stateDir(), "proxy.port")); err == nil {
		port = strings.TrimSpace(string(data))
	}
	switch port {
	case "", "443":
		return "https://" + host
	case "80":
		return "http://" + host
	default:
		return fmt.Sprintf("http://%s:%s", host, port)
	}
}
