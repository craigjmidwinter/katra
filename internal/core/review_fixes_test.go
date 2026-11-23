package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTask(t *testing.T, s *Store, name, src string) string {
	t.Helper()
	p := filepath.Join(s.DirForType("task"), name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestHandWrittenShapesDoNotBreakTheStore is Daisy's H1: a scalar blocked_by
// reads as a one-item list, and a list owner leaves the node readable (and
// the store listable) but refuses to be rewritten, so the value survives.
func TestHandWrittenShapesDoNotBreakTheStore(t *testing.T) {
	s := newTestStore(t)
	writeTask(t, s, "2026-11-23-scalar.md", "---\ntitle: Scalar\ndate: \"2026-11-23\"\ntype: task\nstatus: blocked\nblocked_by: other-task\n---\n\nBody.\n")
	listOwner := "---\ntitle: Pair\ndate: \"2026-11-23\"\ntype: task\nstatus: todo\nowner: [a, b]\neffort: M\n---\n\nBody.\n"
	p := writeTask(t, s, "2026-11-23-pair.md", listOwner)

	nodes, err := s.ListNodes("task")
	if err != nil {
		t.Fatalf("one odd file made the store unreadable: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("listed %d nodes, want 2", len(nodes))
	}
	sc, _ := s.GetNode("scalar")
	if strings.Join(sc.FM.BlockedBy, "|") != "other-task" || ValidateNode(*sc) != nil {
		t.Errorf("scalar blocked_by: %q, %v", sc.FM.BlockedBy, ValidateNode(*sc))
	}
	pair, _ := s.GetNode("pair")
	if len(pair.Unreadable) != 1 || pair.FM.Effort != "M" {
		t.Errorf("list owner: unreadable %q, effort %q (the rest must still decode)", pair.Unreadable, pair.FM.Effort)
	}
	if errs := ValidateNode(*pair); len(errs) != 1 || !strings.Contains(errs[0].Error(), "cannot read") {
		t.Errorf("ValidateNode: %v", errs)
	}
	if _, err := s.SetStatus("pair", "doing"); err == nil || !strings.Contains(err.Error(), "fix them by hand") {
		t.Errorf("rewriting a node with an unreadable value: %v, want a refusal", err)
	}
	if b, _ := os.ReadFile(p); string(b) != listOwner {
		t.Errorf("the refused write changed the file:\n%s", b)
	}
}

// TestMigrateOntoNullKeys is Daisy's M4: an empty (null) owner or effort key
// plus the matching tag migrates to a file that parses with the planned values.
func TestMigrateOntoNullKeys(t *testing.T) {
	s := newTestStore(t)
	p := writeTask(t, s, "2026-11-23-nulls.md", "---\ntitle: Nulls\ndate: \"2026-11-23\"\ntags:\n  - owner:craig\n  - size:XL\n  - keep\ntype: task\nstatus: todo\neffort:\nowner:\n---\n\nBody.\n")
	nodes, err := s.ListNodes("task")
	if err != nil {
		t.Fatal(err)
	}
	plan := PlanInterimTags(nodes)
	if len(plan) != 1 {
		t.Fatalf("plan: %+v", plan)
	}
	if err := plan[0].Apply(); err != nil {
		t.Fatal(err)
	}
	got, err := ParseEntry(p)
	if err != nil || len(got.Unreadable) > 0 {
		t.Fatalf("migrated file does not parse: %v %v", err, got.Unreadable)
	}
	if got.FM.Owner != "craig" || got.FM.Effort != "XL" || strings.Join(got.FM.Tags, ",") != "keep" {
		t.Errorf("after migrate: owner %q effort %q tags %v", got.FM.Owner, got.FM.Effort, got.FM.Tags)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "\n  - keep\n") {
		t.Errorf("2-space list indent not kept:\n%s", b)
	}
	if strings.Contains(string(b), "!!null") {
		t.Errorf("null tag leaked:\n%s", b)
	}
}

func TestReviewLowNotes(t *testing.T) {
	// Two size tags are a conflict, not last-one-wins.
	plan := PlanInterimTags([]Entry{{Slug: "n", FM: Frontmatter{Type: "task", Tags: []string{"size:S", "size:XL"}}}})
	if len(plan) != 1 || plan[0].Node.FM.Effort != "" || len(plan[0].Skipped) != 2 {
		t.Errorf("two size tags: %+v", plan)
	}
	// Duplicates are compared trimmed.
	if errs := ValidateNode(Entry{Slug: "a", FM: Frontmatter{BlockedBy: StringList{"b", "b "}}}); len(errs) != 1 {
		t.Errorf("trimmed duplicate: %v", errs)
	}
	// The hyphenated key is named, not silently ignored.
	if errs := ValidateNode(Entry{Slug: "a", FM: Frontmatter{Extra: map[string]any{"blocked-by": "x"}}}); len(errs) != 1 || !strings.Contains(errs[0].Error(), "blocked_by") {
		t.Errorf("blocked-by key: %v", errs)
	}
	// An ask id is a pending decision, not a missing node.
	nodes := []Entry{{Slug: "t", FM: Frontmatter{Type: "task", Status: "blocked", BlockedBy: StringList{"20261111-1d3c47"}}}}
	if p := BlockerProblems(nodes); len(p) != 0 {
		t.Errorf("ask id reported: %+v", p)
	}
}
