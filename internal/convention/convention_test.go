package convention

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := &Convention{
		Name:    "myapp",
		Cmd:     "npm run dev",
		AppPort: 3000,
		Env:     map[string]string{"DEBUG": "1"},
	}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	if !Exists(dir) {
		t.Fatal("Exists should be true after Save")
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "myapp" || out.Cmd != "npm run dev" || out.AppPort != 3000 || out.Env["DEBUG"] != "1" {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	if err := Save(root, &Convention{Name: "myapp", Cmd: "npm run dev"}); err != nil {
		t.Fatal(err)
	}
	// From a nested subdirectory, Find should locate the project's .qwok.toml.
	sub := filepath.Join(root, "src", "components")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, c, err := Find(sub)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if dir != root || c.Name != "myapp" {
		t.Fatalf("Find = (%q, %+v), want root=%q name=myapp", dir, c, root)
	}
	// A directory with no .qwok.toml anywhere above returns an error.
	if _, _, err := Find(t.TempDir()); err == nil {
		t.Fatal("expected error when no .qwok.toml exists above")
	}
}

func TestLoadValidatesRequiredFields(t *testing.T) {
	dir := t.TempDir()
	// name present, cmd missing -> error.
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`name = "x"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for missing cmd")
	}
	// Missing file -> error.
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error for missing file")
	}
}
