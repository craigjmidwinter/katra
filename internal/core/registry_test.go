package core

import (
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "registry.yml")
	t.Setenv("KATRA_REGISTRY", reg)
	if got := RegistryPath(); got != reg {
		t.Fatalf("RegistryPath() = %q, want %q", got, reg)
	}

	// Missing file -> empty registry, no error.
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(r.Projects) != 0 {
		t.Fatalf("fresh registry not empty: %v", r.Projects)
	}

	// Register a real store, a bogus path, and a duplicate.
	s := newTestStore(t) // has a config.yml
	bogus, _ := filepath.Abs(filepath.Join(t.TempDir(), "gone"))
	for _, d := range []string{s.Dir, bogus, s.Dir} {
		if err := Register(d); err != nil {
			t.Fatalf("Register(%s): %v", d, err)
		}
	}
	r, _ = LoadRegistry()
	if len(r.Projects) != 2 {
		t.Fatalf("want 2 registered (deduped), got %v", r.Projects)
	}

	// Prune drops the bogus one (no config.yml), keeps the real store.
	removed := r.Prune()
	if len(removed) != 1 || removed[0] != bogus {
		t.Fatalf("Prune removed = %v, want [%s]", removed, bogus)
	}
	if len(r.Projects) != 1 || r.Projects[0] != s.Dir {
		t.Fatalf("after prune = %v, want [%s]", r.Projects, s.Dir)
	}
}
