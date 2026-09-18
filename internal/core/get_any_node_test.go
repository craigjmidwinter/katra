package core

import "testing"

// TestGetResolvesEveryNodeType is the fix for a node that could be created and
// then not composed with the tool that created it. `katra decide` wrote a
// decision with a summary, and `katra append --entry <that slug>` could not
// address it, so the body had to be written outside katra — the exact
// displacement katra exists to remove.
func TestGetResolvesEveryNodeType(t *testing.T) {
	s, _ := gitTestStore(t)

	for _, tc := range []struct {
		kind  string
		title string
	}{
		{"decision", "A decision"},
		{"task", "A task"},
		{"epic", "An epic"},
		{"article", "An article"},
		{"entry", "An entry"},
	} {
		made, err := s.NewNode(tc.kind, Frontmatter{Title: tc.title}, "")
		if err != nil {
			t.Fatalf("NewNode(%s): %v", tc.kind, err)
		}
		got, err := s.Get(made.Slug)
		if err != nil {
			t.Errorf("Get(%q) for a %s: %v — it cannot be composed with append", made.Slug, tc.kind, err)
			continue
		}
		if got.Slug != made.Slug {
			t.Errorf("Get(%q) returned %q", made.Slug, got.Slug)
		}
	}
}

// TestGetStillReportsAMissingSlug: resolving more widely must not make a typo
// silently succeed.
func TestGetStillReportsAMissingSlug(t *testing.T) {
	s, _ := gitTestStore(t)
	if _, err := s.Get("nothing-by-this-name"); err == nil {
		t.Error("Get succeeded for a slug that does not exist")
	}
}
