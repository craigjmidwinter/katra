package core

import (
	"os"
	"strings"
	"testing"
)

// The metric this field exists for is "who creates and ranks work". An
// instrument that guesses when it does not know measures whoever remembered to
// set a variable, so absence has to survive as absence all the way to disk.

func TestActorTokenIsAbsentNotDefaulted(t *testing.T) {
	for _, unset := range []string{"", "   ", "\t\n"} {
		t.Setenv(ActorEnv, unset)
		if got := ActorToken(); got != "" {
			t.Errorf("ActorToken() = %q for env %q, want empty — a blank token is not an author", got, unset)
		}
	}
	if err := os.Unsetenv(ActorEnv); err == nil {
		if got := ActorToken(); got != "" {
			t.Errorf("ActorToken() = %q with the variable unset, want empty", got)
		}
	}
}

// TestUnsetActorWritesNoAuthorKey: the key must be omitted, not written empty.
// An empty author is a claim that the author is unknown-but-recorded, which is
// a different and false statement from "this node predates attribution".
func TestUnsetActorWritesNoAuthorKey(t *testing.T) {
	t.Setenv(ActorEnv, "")
	s, _ := gitTestStore(t)

	n, err := s.NewNode("task", Frontmatter{Title: "Unattributed"}, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(n.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "author:") {
		t.Errorf("an author key was written with no actor set:\n%s", raw)
	}
}

func TestActorTokenIsRecordedVerbatim(t *testing.T) {
	// Deliberately shaped like nothing katra knows: the token is opaque, and
	// validating its shape would be the first step toward interpreting it.
	const token = "  tier:director/sess-9f3a  "
	t.Setenv(ActorEnv, token)
	s, _ := gitTestStore(t)

	n, err := s.NewNode("task", Frontmatter{Title: "Attributed"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := n.FM.Author; got != strings.TrimSpace(token) {
		t.Errorf("Author = %q, want the token back unchanged (%q)", got, strings.TrimSpace(token))
	}
}

// TestExplicitAuthorWinsOverEnvironment: a caller reconstructing history must
// not be overwritten by whatever the ambient environment happens to say.
func TestExplicitAuthorWinsOverEnvironment(t *testing.T) {
	t.Setenv(ActorEnv, "ambient")
	s, _ := gitTestStore(t)

	n, err := s.NewNode("task", Frontmatter{Title: "Backfilled", Author: "explicit"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if n.FM.Author != "explicit" {
		t.Errorf("Author = %q, want explicit", n.FM.Author)
	}
}

// TestUnattributedNodesCountsWhatCannotBeAttributed backs the doctor warning.
// Reporting authored nodes without this number would measure compliance.
func TestUnattributedNodesCountsWhatCannotBeAttributed(t *testing.T) {
	s, _ := gitTestStore(t)

	t.Setenv(ActorEnv, "")
	if _, err := s.NewNode("task", Frontmatter{Title: "Older work"}, ""); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ActorEnv, "someone")
	if _, err := s.NewNode("task", Frontmatter{Title: "Newer work"}, ""); err != nil {
		t.Fatal(err)
	}

	missing, err := s.UnattributedNodes("task")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 {
		t.Fatalf("UnattributedNodes = %d, want 1", len(missing))
	}
	if missing[0].FM.Title != "Older work" {
		t.Errorf("wrong node reported: %q", missing[0].FM.Title)
	}
}
