// Package registry is the global index of *what apps exist* — names mapped to
// their project directories, and nothing more.
//
// This is the "intent only" layer from the design (§5). It deliberately stores
// NO runtime status: a `status: running` field would be a lie the moment a
// process died, so liveness is never recorded here — it is derived elsewhere
// (internal/process + internal/portless). The registry exists so `run`/`list`
// can find an app by name from any directory; the *how to run it* lives in the
// per-project .qwok.toml (internal/convention).
//
// Persisted as TOML at state.RegistryPath(). Read alongside:
// internal/convention (the other config layer) and internal/app (the consumer).
package registry

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/ekinertac/qwok/internal/state"
)

// Entry is one registered app. Path is the project directory whose .qwok.toml
// describes how the app runs. Intentionally minimal — no status, no command.
type Entry struct {
	Path string `toml:"path"`
}

// Registry is the whole index, keyed by app name.
type Registry struct {
	Apps map[string]Entry `toml:"apps"`
}

// Load reads the registry. A missing file is not an error — it is an empty
// registry, which is the correct state before the first `add`.
func Load() (*Registry, error) {
	r := &Registry{Apps: map[string]Entry{}}
	data, err := os.ReadFile(state.RegistryPath())
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", state.RegistryPath(), err)
	}
	if r.Apps == nil {
		r.Apps = map[string]Entry{}
	}
	return r, nil
}

// Save writes the registry back to disk, creating directories as needed.
func Save(r *Registry) error {
	if err := state.EnsureDirs(); err != nil {
		return err
	}
	f, err := os.Create(state.RegistryPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(r)
}

// Get returns the entry for name and whether it exists.
func (r *Registry) Get(name string) (Entry, bool) {
	e, ok := r.Apps[name]
	return e, ok
}

// Set adds or replaces an app entry (does not persist; caller calls Save).
func (r *Registry) Set(name string, e Entry) {
	if r.Apps == nil {
		r.Apps = map[string]Entry{}
	}
	r.Apps[name] = e
}

// Remove deletes an app entry (does not persist; caller calls Save).
func (r *Registry) Remove(name string) { delete(r.Apps, name) }

// Names returns all registered app names in sorted order for stable listing.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Apps))
	for n := range r.Apps {
		names = append(names, n)
	}
	// simple insertion sort keeps the dependency surface at zero; lists are tiny
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}
