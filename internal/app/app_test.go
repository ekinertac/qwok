package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLifecycleWithStubPortless exercises the full daemonless flow end to end:
// add -> run (detached through a fake `portless`) -> running -> stop (SIGTERM to
// the process group) -> stopped. The stub is a real long-lived process so we
// genuinely validate detached launch, the PID hint, live status derivation, and
// process-group signalling — not mocks of them.
func TestLifecycleWithStubPortless(t *testing.T) {
	// Isolate every on-disk location into temp dirs.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PORTLESS_STATE_DIR", t.TempDir())

	// Stub `portless` on PATH: ignores its args and just stays alive, standing in
	// for the real wrapper-that-holds-a-dev-server.
	stubDir := t.TempDir()
	stub := filepath.Join(stubDir, "portless")
	// `get` echoes the canonical URL; any other invocation is the launch wrapper,
	// which just stays alive standing in for a running dev server.
	script := "#!/bin/sh\n[ \"$1\" = get ] && echo \"http://$2.localhost\" && exit 0\nexec sleep 30\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+":"+os.Getenv("PATH"))

	projDir := t.TempDir()

	if err := Add(AddOptions{Name: "demo", Cwd: projDir, Cmd: "npm run dev"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if Running("demo") {
		t.Fatal("should not be running before run")
	}

	if _, err := Run("demo", false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() { _ = Kill("demo") }) // never leak the stub

	if !waitFor(func() bool { return Running("demo") }) {
		t.Fatal("app did not come up")
	}

	// Double-start without force is refused.
	if _, err := Run("demo", false); err == nil {
		t.Fatal("expected refusal to start an already-running app")
	}

	// List reflects derived status.
	rows, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "demo" || !rows[0].Running {
		t.Fatalf("List = %+v", rows)
	}
	if rows[0].URL != "http://demo.localhost" {
		t.Fatalf("URL = %q, want http://demo.localhost (from stub portless get)", rows[0].URL)
	}

	// Graceful stop tears the process group down.
	if err := Stop("demo"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !waitFor(func() bool { return !Running("demo") }) {
		t.Fatal("app did not stop")
	}

	// Stopping an already-stopped app is an error.
	if err := Stop("demo"); err == nil {
		t.Fatal("expected error stopping a stopped app")
	}
}

func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
