package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateNodeOwnerAndBlockedBy(t *testing.T) {
	ok := Entry{Slug: "a", FM: Frontmatter{Type: "task", Owner: "role:steward", BlockedBy: []string{"b", "craig decides the licence"}}}
	if errs := ValidateNode(ok); errs != nil {
		t.Errorf("valid node rejected: %v", errs)
	}
	if errs := ValidateNode(Entry{Slug: "a", FM: Frontmatter{Type: "task"}}); errs != nil {
		t.Errorf("node without either key rejected: %v", errs)
	}
	cases := map[string]Frontmatter{
		"owner is blank": {Owner: "  "},
		"empty item":     {BlockedBy: []string{""}},
		"names the node": {BlockedBy: []string{"a"}},
		"lists b twice":  {BlockedBy: []string{"b", "b"}},
	}
	for want, fm := range cases {
		errs := ValidateNode(Entry{Slug: "a", FM: fm})
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), want) {
			t.Errorf("%+v: got %v, want one error containing %q", fm, errs, want)
		}
	}
}

// TestOwnerAndBlockedByRoundTrip covers both directions of the new keys: they
// survive a save, and a legacy node without them gains no key at all. Drop
// omitempty from either field and the second half fails.
func TestOwnerAndBlockedByRoundTrip(t *testing.T) {
	dir := t.TempDir()
	withKeys := filepath.Join(dir, "2026-11-23-owned.md")
	src := "---\ntitle: Owned\ndate: \"2026-11-23\"\ntype: task\nstatus: todo\nowner: role:steward\nblocked_by:\n    - other-task\n---\n\nBody.\n"
	if err := os.WriteFile(withKeys, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := ParseEntry(withKeys)
	if err != nil {
		t.Fatal(err)
	}
	if e.FM.Owner != "role:steward" || len(e.FM.BlockedBy) != 1 || e.FM.BlockedBy[0] != "other-task" {
		t.Fatalf("keys not parsed into fields: owner=%q blocked_by=%v extra=%v", e.FM.Owner, e.FM.BlockedBy, e.FM.Extra)
	}
	if b, _ := e.Serialize(); string(b) != src {
		t.Errorf("owned node re-serialized differently:\n got %q\nwant %q", b, src)
	}

	legacy := "---\ntitle: Legacy\ndate: \"2026-01-01\"\ntype: task\nstatus: todo\neffort: S\n---\n\nBody.\n"
	p := filepath.Join(dir, "2026-01-01-legacy.md")
	if err := os.WriteFile(p, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := ParseEntry(p)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := l.Serialize(); string(b) != legacy {
		t.Errorf("legacy node gained keys on save:\n got %q\nwant %q", b, legacy)
	}
}

func TestNormalizeEffort(t *testing.T) {
	for in, want := range map[string]string{"": "", "xs": "XS", " M ": "M", "l": "L", "XL": "XL", "s": "S"} {
		got, err := NormalizeEffort(in)
		if err != nil || got != want {
			t.Errorf("NormalizeEffort(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"XXL", "medium", "2", "S/M"} {
		if _, err := NormalizeEffort(bad); err == nil || !strings.Contains(err.Error(), "XS|S|M|L|XL") {
			t.Errorf("NormalizeEffort(%q) = %v, want an error naming the scale", bad, err)
		}
	}
}

// TestValidateNodeEffort checks the stored value: a writer always stores the
// canonical spelling, so lowercase on disk is a hand edit worth reporting, and
// an off-scale value is an error. Both still parse; only writers check.
func TestValidateNodeEffort(t *testing.T) {
	for eff, want := range map[string]string{"XL": "", "m": `should be written "M"`, "huge": "not one of"} {
		errs := ValidateNode(Entry{Slug: "a", FM: Frontmatter{Type: "task", Effort: eff}})
		switch {
		case want == "" && errs != nil:
			t.Errorf("effort %q rejected: %v", eff, errs)
		case want != "" && (len(errs) != 1 || !strings.Contains(errs[0].Error(), want)):
			t.Errorf("effort %q: got %v, want one error containing %q", eff, errs, want)
		}
	}
}
