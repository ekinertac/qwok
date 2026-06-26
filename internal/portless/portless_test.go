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

func TestURLDeterministic(t *testing.T) {
	if got := URL("myapp"); got != "https://myapp.localhost" {
		t.Fatalf("URL = %q", got)
	}
}
