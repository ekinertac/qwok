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

func TestURLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := &Convention{Name: "a", Cmd: "./serve", Type: TypeApp, URL: "https://host.example:8798/x"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil || out.URL != in.URL {
		t.Fatalf("Load = %+v, %v (want url %q)", out, err, in.URL)
	}
}

// `qwok open` hands the url to open(1), and the file comes with cloned repos, so
// anything but a web address must be refused.
func TestURLMustBeAWebAddress(t *testing.T) {
	for _, bad := range []string{"host.example", "file:///etc/hosts", "-a Calculator", "https://", "javascript:1", "http://a b"} {
		dir := t.TempDir()
		body := "name=\"a\"\ncmd=\"b\"\ntype=\"app\"\nurl=\"" + bad + "\"\n"
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Errorf("url %q should be rejected", bad)
		}
	}
}

// A web app gets its URL from portless, so a url line there must not be ignored in silence.
func TestURLIsOnlyForAppType(t *testing.T) {
	dir := t.TempDir()
	body := "name=\"a\"\ncmd=\"b\"\nurl=\"https://host.example\"\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("url on a web app should be rejected")
	}
}

func TestTypeRoundTripAndDefault(t *testing.T) {
	// No type -> defaults to web (backward compatible).
	web := t.TempDir()
	if err := Save(web, &Convention{Name: "a", Cmd: "npx vite"}); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(web); err != nil || c.Type != TypeWeb {
		t.Fatalf("default type: %+v, %v (want web)", c, err)
	}

	// Explicit app type round-trips.
	desk := t.TempDir()
	if err := Save(desk, &Convention{Name: "b", Cmd: "swift run", Type: TypeApp}); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(desk); err != nil || c.Type != TypeApp {
		t.Fatalf("app type: %+v, %v (want app)", c, err)
	}

	// An unknown type is rejected.
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, FileName), []byte("name=\"x\"\ncmd=\"y\"\ntype=\"bogus\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("expected error for invalid type")
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
