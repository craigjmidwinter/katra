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
		t.Setenv(AuthorEnv, unset)
		if got := Author(); got != "" {
			t.Errorf("Author() = %q for env %q, want empty — a blank token is not an author", got, unset)
		}
	}
	if err := os.Unsetenv(AuthorEnv); err == nil {
		if got := Author(); got != "" {
			t.Errorf("Author() = %q with the variable unset, want empty", got)
		}
	}
}

// TestUnsetActorWritesNoAuthorKey: the key must be omitted, not written empty.
// An empty author is a claim that the author is unknown-but-recorded, which is
// a different and false statement from "this node predates attribution".
func TestUnsetActorWritesNoAuthorKey(t *testing.T) {
	t.Setenv(AuthorEnv, "")
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
	t.Setenv(AuthorEnv, token)
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
	t.Setenv(AuthorEnv, "ambient")
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

	t.Setenv(AuthorEnv, "")
	if _, err := s.NewNode("task", Frontmatter{Title: "Older work"}, ""); err != nil {
		t.Fatal(err)
	}
	t.Setenv(AuthorEnv, "someone")
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

// TestAuthorAndClaimAreDifferentVariables is the guard for the conflation that
// nearly shipped: one token feeding both `author` and `claimed_by`.
//
// The runtime exports an ephemeral pane nonce for claims. If that same variable
// fed authorship, every author field would become unreadable hex the moment its
// pane closed — while still looking recorded. Two lifetimes, two variables, and
// nothing may quietly re-join them.
func TestAuthorAndClaimAreDifferentVariables(t *testing.T) {
	if AuthorEnv == ClaimEnv {
		t.Fatalf("author and claim read the same variable (%s); the ephemeral one would destroy the durable one", AuthorEnv)
	}

	t.Setenv(AuthorEnv, "durable-identity")
	t.Setenv(AuthorRoleEnv, "some-role")
	t.Setenv(ClaimEnv, "ephemeral-nonce")

	s, _ := gitTestStore(t)
	n, err := s.NewNode("task", Frontmatter{Title: "Two lifetimes"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if n.FM.Author != "durable-identity" {
		t.Errorf("Author = %q, want the durable identity", n.FM.Author)
	}
	if n.FM.Author == "ephemeral-nonce" {
		t.Error("the pane nonce reached the author field")
	}
	if n.FM.AuthorRole != "some-role" {
		t.Errorf("AuthorRole = %q, want the role captured at creation", n.FM.AuthorRole)
	}
}

// TestAuthorRoleIsAbsentNotInferred: an unset role is absent, never guessed
// from the identity or defaulted to a tier.
func TestAuthorRoleIsAbsentNotInferred(t *testing.T) {
	t.Setenv(AuthorEnv, "someone")
	t.Setenv(AuthorRoleEnv, "")

	s, _ := gitTestStore(t)
	n, err := s.NewNode("task", Frontmatter{Title: "Roleless"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if n.FM.AuthorRole != "" {
		t.Errorf("AuthorRole = %q with the variable unset, want absent", n.FM.AuthorRole)
	}
	raw, err := os.ReadFile(n.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "author_role:") {
		t.Errorf("an author_role key was written with no role set:\n%s", raw)
	}
}
