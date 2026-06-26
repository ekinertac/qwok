package portless

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRoutes(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PORTLESS_STATE_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "routes.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRoutesEmptyAndMissing(t *testing.T) {
	// Missing file -> no routes, no error.
	t.Setenv("PORTLESS_STATE_DIR", t.TempDir())
	if rs, err := Routes(); err != nil || rs != nil {
		t.Fatalf("missing routes.json: got %v, %v", rs, err)
	}
	// Empty array (portless's idle state) -> no routes.
	writeRoutes(t, "[]")
	if rs, err := Routes(); err != nil || len(rs) != 0 {
		t.Fatalf("empty routes: got %v, %v", rs, err)
	}
}

func TestRouteForMatchesSubdomain(t *testing.T) {
	writeRoutes(t, `[
		{"hostname":"myapp.localhost","port":4123,"pid":4242},
		{"hostname":"api.other.localhost","port":4999,"pid":4243}
	]`)
	r, ok := RouteFor("myapp")
	if !ok || r.Port != 4123 {
		t.Fatalf("RouteFor(myapp) = %+v, %v", r, ok)
	}
	r, ok = RouteFor("api.other")
	if !ok || r.Port != 4999 {
		t.Fatalf("RouteFor(api.other) = %+v, %v", r, ok)
	}
	if _, ok := RouteFor("nope"); ok {
		t.Fatal("RouteFor(nope) should not match")
	}
}

func TestURLUsesPortlessGet(t *testing.T) {
	// A stub `portless` whose `get` subcommand echoes the canonical URL — proving
	// URL() reflects the live proxy scheme (here http) rather than assuming https.
	dir := t.TempDir()
	stub := filepath.Join(dir, "portless")
	script := "#!/bin/sh\n[ \"$1\" = get ] && echo \"http://$2.localhost\" && exit 0\nexit 1\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := URL("myapp", ""); got != "http://myapp.localhost" {
		t.Fatalf("URL via portless get = %q, want http://myapp.localhost", got)
	}
}

func TestFallbackURLFromProxyPort(t *testing.T) {
	// With no portless on PATH, URL() falls back to the proxy-port heuristic.
	t.Setenv("PATH", t.TempDir()) // empty dir: `portless` is not found
	cases := map[string]string{
		"80":   "http://app.localhost",
		"443":  "https://app.localhost",
		"none": "https://app.localhost", // no proxy.port file
		"1355": "http://app.localhost:1355",
	}
	for port, want := range cases {
		psd := t.TempDir()
		t.Setenv("PORTLESS_STATE_DIR", psd)
		if port != "none" {
			if err := os.WriteFile(filepath.Join(psd, "proxy.port"), []byte(port), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := URL("app", ""); got != want {
			t.Errorf("proxy.port=%q: URL=%q, want %q", port, got, want)
		}
	}
}
