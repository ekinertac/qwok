package app

import "testing"

// TestAppTypeRunsWithoutPortless verifies the desktop/non-web path: an app-type
// project launches its command directly, needs no portless on PATH, exposes no
// URL, and still gets full lifecycle (status, stop) from the generic process
// layer.
func TestAppTypeRunsWithoutPortless(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// Empty PATH proves app-type never reaches for portless. We use an absolute
	// command (/bin/sleep) so the launch itself doesn't depend on PATH.
	t.Setenv("PATH", t.TempDir())

	proj := t.TempDir()
	if err := Add(AddOptions{Name: "desk", Cwd: proj, Cmd: "/bin/sleep 30", Type: "app"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	url, err := Run("desk", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() { _ = Kill("desk") })
	if url != "" {
		t.Fatalf("app-type should have no URL, got %q", url)
	}
	if !waitFor(func() bool { return Running("desk") }) {
		t.Fatal("app did not start")
	}

	rows, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 || rows[0].Type != "app" || rows[0].URL != "" {
		t.Fatalf("List = %+v (want one app-type row, no URL)", rows)
	}

	// open is meaningless for an app — it should say so, not invent a URL.
	if _, err := URL("desk"); err == nil {
		t.Fatal("URL() should error for an app-type project")
	}

	if err := Stop("desk"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !waitFor(func() bool { return !Running("desk") }) {
		t.Fatal("app did not stop")
	}
}
