package core

import (
	"testing"
	"time"
)

// `doing` is derived, not stored. These pin the derivation, because getting the
// precedence wrong produces a task that reads as live forever or one that never
// reads as live at all.

func TestClaimDerivesDoing(t *testing.T) {
	s, _ := gitTestStore(t)
	n, err := s.NewNode("task", Frontmatter{Title: "Work", Status: "todo"}, "")
	if err != nil {
		t.Fatal(err)
	}

	if got := n.EffectiveStatus(); got != "todo" {
		t.Fatalf("unclaimed task reads %q, want todo", got)
	}
	n.Claim("opaque-token", time.Now())
	if got := n.EffectiveStatus(); got != "doing" {
		t.Errorf("claimed task reads %q, want doing", got)
	}
	n.ReleaseClaim()
	if got := n.EffectiveStatus(); got != "todo" {
		t.Errorf("released task reads %q, want its stored status back", got)
	}
}

// TestDoingIsNotWrittenToDisk: the point of the seam. If `doing` is still
// stored then two systems hold the same fact and the whole exercise is moot.
func TestDoingIsNotWrittenToDisk(t *testing.T) {
	s, _ := gitTestStore(t)
	n, err := s.NewNode("task", Frontmatter{Title: "Work", Status: "todo"}, "")
	if err != nil {
		t.Fatal(err)
	}
	n.Claim("opaque-token", time.Now())
	if err := n.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := s.GetNode(n.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.FM.Status == "doing" {
		t.Error("`doing` was written to frontmatter; it must be derived from the claim")
	}
	if reloaded.EffectiveStatus() != "doing" {
		t.Error("the claim did not survive a round trip")
	}
}

// TestTerminalStatusBeatsAClaim: a finished task is finished even if someone
// forgot to release. Without this, closing work leaves it reading as live and
// the abandoned-work signal fires on every completed task.
func TestTerminalStatusBeatsAClaim(t *testing.T) {
	for _, terminal := range []string{"done", "cut"} {
		e := Entry{FM: Frontmatter{Type: "task", Status: terminal, ClaimedBy: "someone"}}
		if got := e.EffectiveStatus(); got != terminal {
			t.Errorf("a %s task holding a claim reads %q, want %s", terminal, got, terminal)
		}
	}
}

// TestLegacyStoredDoingStillReadsDoing: the on-disk format is the public API and
// katra is released. Tasks written before this existed carry `status: doing` and
// no claim, and they must not silently revert to todo.
func TestLegacyStoredDoingStillReadsDoing(t *testing.T) {
	e := Entry{FM: Frontmatter{Type: "task", Status: "doing"}}
	if got := e.EffectiveStatus(); got != "doing" {
		t.Errorf("a legacy stored `doing` reads %q, want doing", got)
	}
	if e.IsClaimed() {
		t.Error("a legacy `doing` should not masquerade as a claim")
	}
}

// TestClaimWithNoActorIsStillAClaim. No claim means nobody took the work up; an
// unknown claimant means somebody did and the environment could not say who.
// Collapsing the two would lose the distinction the seam is built on.
func TestClaimWithNoActorIsStillAClaim(t *testing.T) {
	e := Entry{FM: Frontmatter{Type: "task", Status: "todo"}}
	e.Claim("", time.Now())

	if !e.IsClaimed() {
		t.Fatal("a claim with no actor token did not register as claimed")
	}
	if e.FM.ClaimedBy != UnknownActor {
		t.Errorf("ClaimedBy = %q, want %q", e.FM.ClaimedBy, UnknownActor)
	}
	if e.EffectiveStatus() != "doing" {
		t.Error("an unattributed claim should still read as doing")
	}
}

// TestClaimedTaskCountsAsStartedInRollup: an epic with claimed work must not
// roll up planned.
func TestClaimedTaskCountsAsStartedInRollup(t *testing.T) {
	s, _ := gitTestStore(t)
	if _, err := s.NewNode("epic", Frontmatter{Title: "Big epic", Status: "planned"}, ""); err != nil {
		t.Fatal(err)
	}
	n, err := s.NewNode("task", Frontmatter{Title: "Child work", Status: "todo", Epic: "big-epic"}, "")
	if err != nil {
		t.Fatal(err)
	}
	n.Claim("someone", time.Now())
	if err := n.Save(); err != nil {
		t.Fatal(err)
	}

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if got := EpicRollupStatus(nodes, "big-epic"); got != "active" {
		t.Errorf("epic with a claimed task rolls up %q, want active", got)
	}
}
