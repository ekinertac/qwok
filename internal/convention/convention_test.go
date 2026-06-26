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
