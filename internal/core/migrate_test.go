package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// handWritten is a task in the style the galaxy-brain log uses: double-quoted
// strings, blank lines before the body, and interim tags.
const handWritten = `---
title: "Ship it: the first release"
date: "2026-10-23"
tags:
    - layer:L3
    - owner:brainstem
    - size:xl
    - blocked-by:pick-a-licence
    - wont-build
summary: "Two lines' worth of summary, quoted the hand-written way"
type: task
status: todo
effort: L
epic: oss-p4
---


Body, after two blank lines.
`

const handWrittenMigrated = `---
title: "Ship it: the first release"
date: "2026-10-23"
tags:
    - layer:L3
    - wont-build
summary: "Two lines' worth of summary, quoted the hand-written way"
type: task
status: todo
effort: XL
owner: brainstem
blocked_by:
    - pick-a-licence
epic: oss-p4
---


Body, after two blank lines.
`

// TestInterimTagMigrationIsMinimalAndIdempotent pins the whole contract on a
// hand-written file: the plan names each change, Apply rewrites only the
// migrated lines (quoting and the body's blank lines survive), and a second
// plan is empty.
func TestInterimTagMigrationIsMinimalAndIdempotent(t *testing.T) {
	s := newTestStore(t)
	p := filepath.Join(s.DirForType("task"), "2026-10-23-ship-it.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(handWritten), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodes("task", "epic")
	if err != nil {
		t.Fatal(err)
	}
	plan := PlanInterimTags(nodes)
	if len(plan) != 1 {
		t.Fatalf("plan has %d nodes, want 1", len(plan))
	}
	want := "owner: brainstem|effort: L → XL|blocked_by: + pick-a-licence|tags: drop 3 migrated"
	if got := strings.Join(plan[0].Changes, "|"); got != want {
		t.Errorf("changes:\n got %q\nwant %q", got, want)
	}
	if b, _ := os.ReadFile(p); string(b) != handWritten {
		t.Fatal("planning wrote the file")
	}
	if err := plan[0].Apply(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != handWrittenMigrated {
		t.Errorf("migrated file:\n%s\nwant:\n%s", b, handWrittenMigrated)
	}
	nodes, _ = s.ListNodes("task", "epic")
	if again := PlanInterimTags(nodes); len(again) != 0 {
		t.Errorf("second plan not empty: %+v", again)
	}
}

// TestInterimTagConflictsStayTags: two owner tags, an owner tag that
// disagrees with the key, and a size off the scale are reported and kept.
func TestInterimTagConflictsStayTags(t *testing.T) {
	mk := func(owner string, tags ...string) Entry {
		return Entry{Slug: "n", FM: Frontmatter{Type: "task", Owner: owner, Tags: tags}}
	}
	for _, c := range []struct {
		e    Entry
		skip string
	}{
		{mk("", "owner:a", "owner:b"), "2 owner tags"},
		{mk("kate", "owner:bosun"), `owner is already "kate"`},
		{mk("", "size:huge"), "not on the effort scale"},
	} {
		plan := PlanInterimTags([]Entry{c.e})
		if len(plan) != 1 || len(plan[0].Skipped) == 0 || !strings.Contains(plan[0].Skipped[0], c.skip) {
			t.Errorf("%v: want a skip containing %q, got %+v", c.e.FM.Tags, c.skip, plan)
			continue
		}
		for _, tag := range c.e.FM.Tags {
			if !contains(plan[0].Node.FM.Tags, tag) {
				t.Errorf("%v: skipped tag %q was dropped", c.e.FM.Tags, tag)
			}
		}
	}
	// An owner tag that agrees with the key is just dropped.
	plan := PlanInterimTags([]Entry{mk("kate", "owner:kate", "x")})
	if len(plan) != 1 || strings.Join(plan[0].Node.FM.Tags, ",") != "x" || len(plan[0].Skipped) != 0 {
		t.Errorf("agreeing owner tag: %+v", plan)
	}
	// Entries and decisions are not migrated.
	if plan := PlanInterimTags([]Entry{{Slug: "e", FM: Frontmatter{Tags: []string{"owner:x"}}}}); len(plan) != 0 {
		t.Errorf("an entry was planned: %+v", plan)
	}
}
