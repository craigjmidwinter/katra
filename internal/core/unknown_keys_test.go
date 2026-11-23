package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyWithForeignKeys is a task written by another tool (or a newer katra):
// it carries keys this version has no field for, including a nested map and a
// list, next to the ones it does know.
const legacyWithForeignKeys = `---
title: Foreign Task
date: "2026-11-01"
type: task
status: todo
owner: role:steward
blocked_by:
    - other-task
x-tool:
    run: 42
    tags: [a, b]
---

Body.
`

// TestSaveKeepsUnknownFrontmatterKeys pins the format promise that an unknown
// key is ignored, not deleted: a status change made through katra must write
// every foreign key back with its value.
func TestSaveKeepsUnknownFrontmatterKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "2026-11-01-foreign-task.md")
	if err := os.WriteFile(p, []byte(legacyWithForeignKeys), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := ParseEntry(p)
	if err != nil {
		t.Fatal(err)
	}
	e.FM.Status = "doing"
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	got := string(b)
	for _, want := range []string{"status: doing", "owner: role:steward", "blocked_by:", "- other-task", "x-tool:", "run: 42"} {
		if !strings.Contains(got, want) {
			t.Errorf("saved file lost %q:\n%s", want, got)
		}
	}
	again, err := ParseEntry(p)
	if err != nil {
		t.Fatal(err)
	}
	if again.FM.Extra["owner"] != "role:steward" {
		t.Errorf("owner did not round-trip: %#v", again.FM.Extra)
	}
}

// TestKnownOnlyFrontmatterSerializesUnchanged guards the other direction: a
// node with no foreign keys must serialize exactly as before, so the change
// rewrites no existing file.
func TestKnownOnlyFrontmatterSerializesUnchanged(t *testing.T) {
	const src = "---\ntitle: Plain\ndate: \"2026-11-01\"\ntype: task\nstatus: todo\n---\n\nBody.\n"
	p := filepath.Join(t.TempDir(), "2026-11-01-plain.md")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := ParseEntry(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != src {
		t.Errorf("known-only node re-serialized differently:\n got: %q\nwant: %q", b, src)
	}
}
