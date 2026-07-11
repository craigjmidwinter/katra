package core

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// newTestStore builds an initialized store in a temp dir.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := InitStore(t.TempDir(), "Test Devlog")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return s
}

// TestNewNodeWritesToCorrectDir verifies each node type lands in its per-type
// directory (and only there), with Type set / Kind() correct.
func TestNewNodeWritesToCorrectDir(t *testing.T) {
	cases := []struct {
		nodeType string
		dir      func(*Store) string
		wantKind string
	}{
		{"entry", (*Store).EntriesDir, "entry"},
		{"", (*Store).EntriesDir, "entry"}, // empty type == entry
		{"task", (*Store).TasksDir, "task"},
		{"epic", (*Store).EpicsDir, "epic"},
		{"decision", (*Store).DecisionsDir, "decision"},
		{"article", (*Store).ArticlesDir, "article"},
	}
	for _, c := range cases {
		t.Run("type="+c.nodeType, func(t *testing.T) {
			s := newTestStore(t)
			e, err := s.NewNode(c.nodeType, Frontmatter{Title: "Hello World"}, "body text")
			if err != nil {
				t.Fatalf("NewNode(%q): %v", c.nodeType, err)
			}
			// File lives in the expected dir.
			wantDir := c.dir(s)
			if filepath.Dir(e.Path) != wantDir {
				t.Errorf("path dir = %s, want %s", filepath.Dir(e.Path), wantDir)
			}
			if _, err := os.Stat(e.Path); err != nil {
				t.Errorf("written file missing: %v", err)
			}
			// Filename follows <date>-<slug>.md.
			base := filepath.Base(e.Path)
			if !strings.HasSuffix(base, "-hello-world.md") {
				t.Errorf("filename = %s, want *-hello-world.md", base)
			}
			// Kind() correct; entries keep empty Type.
			if got := e.Kind(); got != c.wantKind {
				t.Errorf("Kind() = %q, want %q", got, c.wantKind)
			}
			if c.wantKind == "entry" && e.FM.Type != "" {
				t.Errorf("entry FM.Type = %q, want empty", e.FM.Type)
			}
			if c.wantKind != "entry" && e.FM.Type != c.wantKind {
				t.Errorf("FM.Type = %q, want %q", e.FM.Type, c.wantKind)
			}
			// It does not leak into any sibling type dir.
			for _, other := range []string{s.EntriesDir(), s.TasksDir(), s.EpicsDir(), s.DecisionsDir(), s.ArticlesDir()} {
				if other == wantDir {
					continue
				}
				ents, _ := os.ReadDir(other)
				for _, de := range ents {
					if strings.HasSuffix(de.Name(), ".md") {
						t.Errorf("unexpected .md in %s: %s", other, de.Name())
					}
				}
			}
		})
	}
}

// TestNewNodeRequiresTitle checks that a missing title is rejected.
func TestNewNodeRequiresTitle(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("task", Frontmatter{}, "body"); err == nil {
		t.Fatal("NewNode with empty title: want error, got nil")
	}
}

// TestNewNodeCollisionSuffix verifies same-day same-title nodes get -2, -3 …
func TestNewNodeCollisionSuffix(t *testing.T) {
	s := newTestStore(t)
	fm := Frontmatter{Title: "Dup Title", Date: "2026-07-10"}
	first, err := s.NewNode("task", fm, "a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.NewNode("task", fm, "b")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first.Path) == filepath.Base(second.Path) {
		t.Fatalf("collision not suffixed: both %s", filepath.Base(first.Path))
	}
	if !strings.HasSuffix(second.Path, "-dup-title-2.md") {
		t.Errorf("second path = %s, want *-dup-title-2.md", second.Path)
	}
}

// TestListNodesFilterAndSort creates nodes across types and asserts type
// filtering and newest-first ordering.
func TestListNodesFilterAndSort(t *testing.T) {
	s := newTestStore(t)
	// Fixed dates so ordering is deterministic (newest-first).
	mk := func(nodeType, title, date string) {
		if _, err := s.NewNode(nodeType, Frontmatter{Title: title, Date: date}, ""); err != nil {
			t.Fatalf("NewNode(%s,%s): %v", nodeType, title, err)
		}
	}
	mk("entry", "Entry Old", "2026-07-01")
	mk("entry", "Entry New", "2026-07-05")
	mk("task", "Task A", "2026-07-03")
	mk("epic", "Epic A", "2026-07-04")
	mk("decision", "Dec A", "2026-07-02")
	mk("article", "Art A", "2026-07-06")

	// Filter: only tasks.
	tasks, err := s.ListNodes("task")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].FM.Title != "Task A" || tasks[0].Kind() != "task" {
		t.Fatalf("ListNodes(task) = %+v, want single Task A", tasks)
	}

	// Filter: two types, entries + epics.
	mix, err := s.ListNodes("entry", "epic")
	if err != nil {
		t.Fatal(err)
	}
	var gotTitles []string
	for _, e := range mix {
		gotTitles = append(gotTitles, e.FM.Title)
	}
	// newest-first: Entry New (07-05), Epic A (07-04), Entry Old (07-01)
	wantTitles := []string{"Entry New", "Epic A", "Entry Old"}
	if !reflect.DeepEqual(gotTitles, wantTitles) {
		t.Fatalf("ListNodes(entry,epic) order = %v, want %v", gotTitles, wantTitles)
	}

	// No args: every node type, newest-first.
	all, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("ListNodes() len = %d, want 6", len(all))
	}
	// Verify sorted descending by date.
	for i := 1; i < len(all); i++ {
		if all[i-1].FM.Date < all[i].FM.Date {
			t.Errorf("not newest-first at %d: %s before %s", i, all[i-1].FM.Date, all[i].FM.Date)
		}
	}
	if all[0].FM.Title != "Art A" {
		t.Errorf("newest = %q, want Art A", all[0].FM.Title)
	}
}

// TestGetNodeAcrossDirs verifies GetNode resolves slugs regardless of type dir,
// and errors on unknown slugs.
func TestGetNodeAcrossDirs(t *testing.T) {
	s := newTestStore(t)
	made := map[string]string{} // slug -> type
	for _, tc := range []struct{ typ, title string }{
		{"entry", "An Entry"},
		{"task", "A Task"},
		{"epic", "An Epic"},
		{"decision", "A Decision"},
		{"article", "An Article"},
	} {
		e, err := s.NewNode(tc.typ, Frontmatter{Title: tc.title}, "")
		if err != nil {
			t.Fatal(err)
		}
		made[e.Slug] = tc.typ
	}
	for slug, typ := range made {
		got, err := s.GetNode(slug)
		if err != nil {
			t.Errorf("GetNode(%q): %v", slug, err)
			continue
		}
		if got.Kind() != typ {
			t.Errorf("GetNode(%q).Kind() = %q, want %q", slug, got.Kind(), typ)
		}
	}
	if _, err := s.GetNode("does-not-exist"); err == nil {
		t.Error("GetNode(missing): want error, got nil")
	}
}

// TestListReturnsOnlyEntries verifies back-compat: List() ignores other node
// types and returns entries with Kind()=="entry".
func TestListReturnsOnlyEntries(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("entry", Frontmatter{Title: "The Entry"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "The Task"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("epic", Frontmatter{Title: "The Epic"}, ""); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("List() len = %d, want 1 (entries only)", len(entries))
	}
	if entries[0].FM.Title != "The Entry" || entries[0].Kind() != "entry" {
		t.Errorf("List()[0] = %+v, want The Entry/entry", entries[0])
	}
	// NewEntry parity with NewNode("entry", ...).
	e, err := s.NewEntry(Frontmatter{Title: "Direct Entry"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(e.Path) != s.EntriesDir() || e.Kind() != "entry" {
		t.Errorf("NewEntry wrote %s kind %s, want entries dir / entry", e.Path, e.Kind())
	}
}

// TestStatusRoundTripThroughSave verifies Status set on disk survives a
// re-read (ParseEntry) and that ListNodes reflects updates.
func TestStatusRoundTripThroughSave(t *testing.T) {
	s := newTestStore(t)
	e, err := s.NewNode("task", Frontmatter{Title: "Roundtrip Task", Status: "todo"}, "work")
	if err != nil {
		t.Fatal(err)
	}
	// Mutate status and save (mirrors `task done`).
	e.FM.Status = "done"
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	// Re-read from disk.
	reread, err := ParseEntry(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reread.FM.Status != "done" {
		t.Errorf("reread Status = %q, want done", reread.FM.Status)
	}
	// Serialize normalizes the body to a single trailing newline.
	if strings.TrimRight(reread.Body, "\n") != "work" {
		t.Errorf("reread Body = %q, want work", reread.Body)
	}
	// GetNode observes the update too.
	got, err := s.GetNode(e.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.FM.Status != "done" {
		t.Errorf("GetNode Status = %q, want done", got.FM.Status)
	}
}

// TestFrontmatterRoundTrip checks every node-model field survives
// Serialize -> parseEntryBytes unchanged.
func TestFrontmatterRoundTrip(t *testing.T) {
	orig := Entry{
		Path: "/x/2026-07-10-full.md",
		Slug: "full",
		FM: Frontmatter{
			Title:        "Full Node",
			Date:         "2026-07-10",
			Time:         "12:34:56",
			Tags:         []string{"alpha", "beta"},
			Summary:      "a summary",
			Type:         "decision",
			Status:       "accepted",
			Effort:       "M",
			Horizon:      "next",
			Epic:         "some-epic",
			Entry:        "some-entry",
			Supersedes:   []string{"old-decision", "older-decision"},
			SupersededBy: []string{"newer-decision"},
		},
		Body: "The decision body.\n\nWith paragraphs.",
	}
	b, err := orig.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseEntryBytes(orig.Path, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.FM, orig.FM) {
		t.Fatalf("frontmatter round-trip mismatch\n got: %+v\nwant: %+v", got.FM, orig.FM)
	}
	// Serialize normalizes trailing whitespace to a single newline.
	if strings.TrimRight(got.Body, "\n") != strings.TrimRight(orig.Body, "\n") {
		t.Errorf("body round-trip = %q, want %q", got.Body, orig.Body)
	}
}

// TestSupersededByYAMLKey pins the on-disk YAML key to "superseded-by"
// (hyphenated), which the viewer/other tools depend on.
func TestSupersededByYAMLKey(t *testing.T) {
	e := Entry{FM: Frontmatter{Title: "D", SupersededBy: []string{"x"}}, Body: ""}
	b, err := e.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "superseded-by:") {
		t.Errorf("serialized frontmatter missing superseded-by key:\n%s", b)
	}
}

// TestInitStoreCreatesTypeDirs verifies InitStore lays down every per-type dir.
func TestInitStoreCreatesTypeDirs(t *testing.T) {
	s := newTestStore(t)
	dirs := []string{s.EntriesDir(), s.TasksDir(), s.EpicsDir(), s.DecisionsDir(), s.ArticlesDir(), s.MediaDir()}
	for _, d := range dirs {
		if !dirExists(d) {
			t.Errorf("InitStore did not create %s", d)
		}
	}
	// DirForType mapping.
	pairs := map[string]string{
		"entry": s.EntriesDir(), "": s.EntriesDir(),
		"task": s.TasksDir(), "epic": s.EpicsDir(),
		"decision": s.DecisionsDir(), "article": s.ArticlesDir(),
	}
	for typ, want := range pairs {
		if got := s.DirForType(typ); got != want {
			t.Errorf("DirForType(%q) = %s, want %s", typ, got, want)
		}
	}
	// NodeTypes canonical set.
	want := []string{"entry", "task", "epic", "decision", "article"}
	got := append([]string(nil), NodeTypes...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NodeTypes = %v, want (sorted) %v", NodeTypes, want)
	}
}

// TestCloseTasks verifies stamp's task-closing helper: a task is marked done and
// linked to the entry, the batch validates before mutating, and non-tasks /
// missing slugs are rejected.
func TestCloseTasks(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("epic", Frontmatter{Title: "Big epic"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "Do the thing", Status: "doing", Epic: "big-epic"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("decision", Frontmatter{Title: "A call", Status: "accepted"}, ""); err != nil {
		t.Fatal(err)
	}
	entry, err := s.NewEntry(Frontmatter{Title: "Shipped it"}, "body")
	if err != nil {
		t.Fatal(err)
	}

	// Happy path.
	closed, err := s.CloseTasks(entry.Slug, []string{"do-the-thing"})
	if err != nil {
		t.Fatalf("CloseTasks: %v", err)
	}
	if len(closed) != 1 || closed[0] != "do-the-thing" {
		t.Fatalf("closed = %v, want [do-the-thing]", closed)
	}
	got, err := s.GetNode("do-the-thing")
	if err != nil {
		t.Fatal(err)
	}
	if got.FM.Status != "done" {
		t.Errorf("task status = %q, want done", got.FM.Status)
	}
	if got.FM.Entry != entry.Slug {
		t.Errorf("task entry = %q, want %q", got.FM.Entry, entry.Slug)
	}

	// Missing slug is rejected.
	if _, err := s.CloseTasks(entry.Slug, []string{"nope"}); err == nil {
		t.Error("expected error closing a missing slug")
	}

	// A non-task in the batch is rejected AND nothing is mutated (validate-first).
	if _, err := s.CloseTasks(entry.Slug, []string{"a-call"}); err == nil {
		t.Error("expected error closing a decision as a task")
	}
	if d, _ := s.GetNode("a-call"); d.FM.Status != "accepted" {
		t.Errorf("decision status mutated to %q; should stay accepted", d.FM.Status)
	}
}

// TestEpicRollupStatus checks the epic status computed from child tasks.
func TestEpicRollupStatus(t *testing.T) {
	mk := func(status string) Entry {
		return Entry{FM: Frontmatter{Type: "task", Status: status, Epic: "e"}}
	}
	cases := []struct {
		name  string
		tasks []Entry
		want  string
	}{
		{"no children", nil, ""},
		{"all todo", []Entry{mk("todo"), mk("todo")}, "planned"},
		{"one doing", []Entry{mk("todo"), mk("doing")}, "active"},
		{"some done", []Entry{mk("done"), mk("todo")}, "active"},
		{"all done", []Entry{mk("done"), mk("done")}, "done"},
		{"cut ignored -> all-done", []Entry{mk("done"), mk("cut")}, "done"},
		{"only cut -> no basis", []Entry{mk("cut")}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A task for a different epic must never count.
			nodes := append([]Entry{{FM: Frontmatter{Type: "task", Status: "todo", Epic: "other"}}}, c.tasks...)
			if got := EpicRollupStatus(nodes, "e"); got != c.want {
				t.Errorf("EpicRollupStatus = %q, want %q", got, c.want)
			}
		})
	}
}
