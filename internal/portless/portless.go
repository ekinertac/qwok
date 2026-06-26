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
	"os"
	"path/filepath"
	"strings"
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

// URL is the deterministic browser URL for an app. portless defaults to HTTPS on
// 443 with the .localhost TLD, so the URL never needs the port to be reachable.
func URL(name string) string { return "https://" + name + ".localhost" }
