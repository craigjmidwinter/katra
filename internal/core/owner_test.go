package core

import (
	"strings"
	"testing"
)

func TestCreationOwner(t *testing.T) {
	s := newTestStore(t)
	if o, err := s.CreationOwner("  me "); err != nil || o != "me" {
		t.Errorf("CreationOwner trims: %q %v", o, err)
	}
	if o, err := s.CreationOwner(""); err != nil || o != "" {
		t.Errorf("optional by default: %q %v", o, err)
	}
	s.Config.RequireOwner = true
	if _, err := s.CreationOwner(" "); err == nil || !strings.Contains(err.Error(), "--owner") {
		t.Errorf("requireOwner: want a refusal naming --owner, got %v", err)
	}
}

func TestOwnerGapsAndBlockerProblems(t *testing.T) {
	mk := func(slug, kind, status, owner string, by ...string) Entry {
		return Entry{Slug: slug, FM: Frontmatter{Type: kind, Status: status, Owner: owner, BlockedBy: by}}
	}
	nodes := []Entry{
		mk("owned", "task", "todo", "me"),
		mk("orphan", "task", "doing", ""),
		mk("finished", "task", "done", ""),
		mk("plan", "epic", "planned", ""),
		{Slug: "an-entry"},
		mk("dep-done", "task", "done", "x"),
		mk("dep-open", "task", "todo", "x"),
		mk("on-typo", "task", "blocked", "x", "dep-open", "dep-opne"),
		mk("on-finished", "task", "blocked", "x", "dep-done"),
		mk("on-outside", "task", "blocked", "x", "dep-done", "craig picks a licence"),
	}
	var gaps []string
	for _, g := range OwnerGaps(nodes) {
		gaps = append(gaps, g.Slug)
	}
	if strings.Join(gaps, ",") != "orphan,plan" {
		t.Errorf("OwnerGaps = %v, want orphan,plan", gaps)
	}
	var got []string
	for _, p := range BlockerProblems(nodes) {
		got = append(got, p.Slug+": "+p.Message)
	}
	want := []string{
		"on-typo: blocked by dep-opne, which names no node",
		"on-finished: every blocker is done or cut — `katra task unblock on-finished`",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("BlockerProblems:\n got %q\nwant %q", got, want)
	}
}
