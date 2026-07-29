package viewer

import (
	"sync"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// newHubSet builds a hubSet the way ServeHub does, without starting a server.
func newHubSet(load func() ([]HubProject, error)) *hubSet {
	return &hubSet{
		load:     load,
		hub:      &reloadHub{clients: map[chan string]struct{}{}},
		watching: map[string]bool{},
	}
}

func proj(id, dir string) HubProject {
	return HubProject{ID: id, Store: &core.Store{Dir: dir}}
}

// TestHubSetPicksUpNewProjects is the regression: the hub runs as a login-time
// daemon, so a katra created afterwards must appear without a restart.
func TestHubSetPicksUpNewProjects(t *testing.T) {
	var mu sync.Mutex
	current := []HubProject{proj("alpha", t.TempDir())}
	set := newHubSet(func() ([]HubProject, error) {
		mu.Lock()
		defer mu.Unlock()
		return append([]HubProject(nil), current...), nil
	})

	if changed := set.refresh(); !changed {
		t.Fatal("first refresh should report a change (empty -> 1 project)")
	}
	if _, byID := set.snapshot(); len(byID) != 1 || byID["alpha"] == nil {
		t.Fatalf("expected alpha in the set, got %v", byID)
	}

	// A refresh with the same roster must not claim a change — that would
	// broadcast a needless reload to every open tab every interval.
	if changed := set.refresh(); changed {
		t.Fatal("refresh with an unchanged roster reported a change")
	}

	mu.Lock()
	current = append(current, proj("beta", t.TempDir()))
	mu.Unlock()

	if changed := set.refresh(); !changed {
		t.Fatal("refresh should report the newly registered katra")
	}
	projects, byID := set.snapshot()
	if len(projects) != 2 || byID["beta"] == nil {
		t.Fatalf("beta not picked up: %d projects, byID=%v", len(projects), byID)
	}
}

// TestHubSetKeepsSetOnLoadError guards the daemon against a transient
// unreadable registry: a mid-write read must not blank the board.
func TestHubSetKeepsSetOnLoadError(t *testing.T) {
	var fail bool
	set := newHubSet(func() ([]HubProject, error) {
		if fail {
			return nil, errLoad
		}
		return []HubProject{proj("alpha", t.TempDir())}, nil
	})
	set.refresh()

	fail = true
	if changed := set.refresh(); changed {
		t.Fatal("a load error must not report a roster change")
	}
	if projects, _ := set.snapshot(); len(projects) != 1 {
		t.Fatalf("load error emptied the set: %d projects", len(projects))
	}
}

// TestHubSetWatchesEachStoreOnce keeps the refresh loop from leaking a new
// watcher goroutine per tick for every already-watched store.
func TestHubSetWatchesEachStoreOnce(t *testing.T) {
	dir := t.TempDir()
	set := newHubSet(func() ([]HubProject, error) {
		return []HubProject{proj("alpha", dir)}, nil
	})
	for i := 0; i < 3; i++ {
		set.refresh()
	}
	set.mu.RLock()
	defer set.mu.RUnlock()
	if len(set.watching) != 1 {
		t.Fatalf("watching = %v, want exactly one entry", set.watching)
	}
}

type loadErr struct{}

func (loadErr) Error() string { return "registry unreadable" }

var errLoad = loadErr{}
