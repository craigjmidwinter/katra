package viewer_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/craigjmidwinter/katra/internal/viewer"
)

// TestHubSnapshot locks the contract the macOS menu bar client reads: drafts and
// doing tasks land in InFlight (stamped entries never do), Recent holds only
// logged entries, and every row carries the repo root so a client can shell out
// to the CLI in the right directory.
func TestHubSnapshot(t *testing.T) {
	// Mirror the real layout: the store lives at <repo>/katra, so the snapshot's
	// Root must resolve back to the repo, not the store.
	root := t.TempDir()
	s, err := core.InitStore(filepath.Join(root, core.DefaultDirName), "Alpha")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	// A stamped entry (belongs in Recent), an unstamped draft and a doing task
	// (both in flight), plus a todo task that must stay out of everything.
	if _, err := s.NewEntry(core.Frontmatter{Title: "Shipped it", Date: "2026-01-02", Hash: "abc1234"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewEntry(core.Frontmatter{Title: "Still writing", Date: "2026-01-03"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Wire the API", Status: "doing"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Someday", Status: "todo"}, ""); err != nil {
		t.Fatal(err)
	}

	snap := viewer.HubSnapshot([]viewer.HubProject{{ID: "alpha", Store: s}})

	if len(snap.Projects) != 1 {
		t.Fatalf("Projects = %d, want 1", len(snap.Projects))
	}
	p := snap.Projects[0]
	if p.Title != "Alpha" {
		t.Errorf("Title = %q, want Alpha", p.Title)
	}
	if p.URL != "/p/alpha/" {
		t.Errorf("URL = %q, want /p/alpha/", p.URL)
	}
	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if p.Dir != filepath.Join(root, core.DefaultDirName) {
		t.Errorf("Dir = %q, want the store dir under %q", p.Dir, root)
	}
	if p.Counts.Entries != 2 || p.Counts.Drafts != 1 || p.Counts.Doing != 1 || p.Counts.Tasks != 2 {
		t.Errorf("Counts = %+v, want 2 entries / 1 draft / 1 doing / 2 tasks", p.Counts)
	}

	// In flight: exactly the draft and the doing task.
	if len(snap.InFlight) != 2 {
		t.Fatalf("InFlight = %d rows, want 2: %+v", len(snap.InFlight), snap.InFlight)
	}
	kinds := map[string]string{}
	for _, it := range snap.InFlight {
		kinds[it.Kind] = it.Title
		if it.Root != root {
			t.Errorf("InFlight %q Root = %q, want %q", it.Title, it.Root, root)
		}
		if it.URL == "" || it.Slug == "" {
			t.Errorf("InFlight %q missing URL/slug: %+v", it.Title, it)
		}
	}
	if kinds["draft"] != "Still writing" {
		t.Errorf("draft row = %q, want Still writing", kinds["draft"])
	}
	if kinds["task"] != "Wire the API" {
		t.Errorf("task row = %q, want Wire the API", kinds["task"])
	}

	// Recent: only the stamped entry, carrying its hash.
	if len(snap.Recent) != 1 {
		t.Fatalf("Recent = %d rows, want 1: %+v", len(snap.Recent), snap.Recent)
	}
	if got := snap.Recent[0]; got.Title != "Shipped it" || got.Hash != "abc1234" || got.Kind != "entry" {
		t.Errorf("Recent[0] = %+v, want the stamped 'Shipped it' entry", got)
	}
}

// TestHubSnapshotIncludesTaskSpec locks the spec field on task rows in the
// JSON snapshot: set on a doing task it carries through to InFlight, and it's
// omitted from the wire when empty (the omitempty tag, not just the Go zero
// value).
func TestHubSnapshotIncludesTaskSpec(t *testing.T) {
	root := t.TempDir()
	s, err := core.InitStore(filepath.Join(root, core.DefaultDirName), "Spec Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Specced Work", Status: "doing", Spec: "docs/design/foo.md"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "No Spec Yet", Status: "doing"}, ""); err != nil {
		t.Fatal(err)
	}

	snap := viewer.HubSnapshot([]viewer.HubProject{{ID: "spec-test", Store: s}})
	if len(snap.InFlight) != 2 {
		t.Fatalf("InFlight = %d rows, want 2: %+v", len(snap.InFlight), snap.InFlight)
	}
	byTitle := map[string]string{}
	for _, it := range snap.InFlight {
		byTitle[it.Title] = it.Spec
	}
	if byTitle["Specced Work"] != "docs/design/foo.md" {
		t.Errorf("Spec = %q, want docs/design/foo.md", byTitle["Specced Work"])
	}
	if byTitle["No Spec Yet"] != "" {
		t.Errorf("Spec = %q, want empty", byTitle["No Spec Yet"])
	}

	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"spec": "docs/design/foo.md"`) {
		t.Errorf("marshaled snapshot missing spec field:\n%s", b)
	}
	// The spec-less row's JSON block must not carry a "spec" key at all.
	noSpecBlock := string(b)[strings.Index(string(b), `"No Spec Yet"`):]
	if end := strings.Index(noSpecBlock, "}"); end >= 0 {
		noSpecBlock = noSpecBlock[:end]
	}
	if strings.Contains(noSpecBlock, `"spec"`) {
		t.Errorf("spec-less task row should omit the spec key:\n%s", noSpecBlock)
	}
}

// TestHubSnapshotOrdering checks newest-first ordering across projects, which is
// what makes the menu bar list readable without client-side sorting.
func TestHubSnapshotOrdering(t *testing.T) {
	mk := func(t *testing.T, title string, dates ...string) *core.Store {
		t.Helper()
		s, err := core.InitStore(filepath.Join(t.TempDir(), core.DefaultDirName), title)
		if err != nil {
			t.Fatal(err)
		}
		for i, d := range dates {
			fm := core.Frontmatter{Title: title + "-" + d, Date: d, Hash: "h"}
			if i == 0 {
				fm.Time = "09:00:00"
			}
			if _, err := s.NewEntry(fm, ""); err != nil {
				t.Fatal(err)
			}
		}
		return s
	}

	old := mk(t, "Old", "2026-01-01")
	fresh := mk(t, "Fresh", "2026-06-01")

	snap := viewer.HubSnapshot([]viewer.HubProject{
		{ID: "old", Store: old},
		{ID: "fresh", Store: fresh},
	})

	if snap.Projects[0].ID != "fresh" {
		t.Errorf("projects sorted %q first, want fresh (most recent activity)", snap.Projects[0].ID)
	}
	if snap.Recent[0].Project != "fresh" {
		t.Errorf("recent[0] from %q, want fresh", snap.Recent[0].Project)
	}
}
