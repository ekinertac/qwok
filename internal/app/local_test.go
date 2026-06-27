package app

import (
	"os"
	"testing"

	"github.com/ekinertac/qwok/internal/convention"
	"github.com/ekinertac/qwok/internal/registry"
)

// TestLocalNameSelfRegisters verifies that argument-less resolution finds the
// nearest .qwok.toml and registers the app if it wasn't already, so `qwok run`
// works from any project directory.
func TestLocalNameSelfRegisters(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	proj := t.TempDir()
	if err := convention.Save(proj, &convention.Convention{Name: "fromdir", Cmd: "npm run dev"}); err != nil {
		t.Fatal(err)
	}

	// Run from inside the project directory.
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(proj); err != nil {
		t.Fatal(err)
	}
	// The resolved cwd (macOS maps /var -> /private/var) is what gets registered.
	wantDir, _ := os.Getwd()

	name, err := LocalName()
	if err != nil {
		t.Fatalf("LocalName: %v", err)
	}
	if name != "fromdir" {
		t.Fatalf("LocalName = %q, want fromdir", name)
	}

	// It should have self-registered name -> project dir.
	reg, _ := registry.Load()
	if e, ok := reg.Get("fromdir"); !ok || e.Path != wantDir {
		t.Fatalf("not self-registered: %+v, %v (want %s)", e, ok, wantDir)
	}
}

func TestLocalNameOutsideProjectErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	empty := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(empty); err != nil {
		t.Fatal(err)
	}
	if _, err := LocalName(); err == nil {
		t.Fatal("expected error outside a qwok project")
	}
}
