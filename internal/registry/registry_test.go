package registry

import "testing"

func TestRoundTripAndMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Missing registry loads as empty, not an error.
	r, err := Load()
	if err != nil || len(r.Apps) != 0 {
		t.Fatalf("empty load: %v, %v", r, err)
	}

	r.Set("myapp", Entry{Path: "/Users/x/Code/myapp"})
	r.Set("api", Entry{Path: "/Users/x/Code/api"})
	if err := Save(r); err != nil {
		t.Fatal(err)
	}

	r2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := r2.Get("myapp"); !ok || e.Path != "/Users/x/Code/myapp" {
		t.Fatalf("Get(myapp) = %+v, %v", e, ok)
	}
	if names := r2.Names(); len(names) != 2 || names[0] != "api" || names[1] != "myapp" {
		t.Fatalf("Names() = %v (want sorted [api myapp])", names)
	}

	r2.Remove("api")
	if _, ok := r2.Get("api"); ok {
		t.Fatal("api should be removed")
	}
}
