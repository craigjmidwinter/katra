package core

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckpointThresholdStaysSilentWhenNothingIsInFlight is the property that
// keeps the pre-compact hook from being muted. A hook that fires every time is
// one people turn off, and then it is not there for the compaction that mattered.
func TestCheckpointThresholdStaysSilentWhenNothingIsInFlight(t *testing.T) {
	s, _ := gitTestStore(t)

	if s.BuildCheckpoint().HasOpenLoops() {
		t.Error("a clean store reports open loops; the hook would fire on every compaction")
	}
}

// TestCheckpointThresholdFiresOnUncommittedWork: the case it exists for.
func TestCheckpointThresholdFiresOnUncommittedWork(t *testing.T) {
	s, repo := gitTestStore(t)
	writeFile(t, filepath.Join(repo, "a.go"), "a1\n")

	if !s.BuildCheckpoint().HasOpenLoops() {
		t.Error("changed code in the working tree is not reported as an open loop")
	}
}

// TestCheckpointSpeccedAloneIsNotAnOpenLoop. A specced task is already durable
// on disk; losing context does not lose it, so it must not on its own trigger a
// checkpoint on every compaction for the life of the task.
func TestCheckpointSpeccedAloneIsNotAnOpenLoop(t *testing.T) {
	s, _ := gitTestStore(t)
	if _, err := s.NewNode("task", Frontmatter{Title: "Designed, unstarted", Status: "specced"}, ""); err != nil {
		t.Fatal(err)
	}

	c := s.BuildCheckpoint()
	if len(c.Specced) != 1 {
		t.Fatalf("expected the specced task to be listed, got %d", len(c.Specced))
	}
	if c.HasOpenLoops() {
		t.Error("a specced task alone triggers a checkpoint; it is already durable on disk")
	}
}

// TestCheckpointRendersStatusNotNarrative pins the shape. The whole finding was
// that an entry says what happened while a clearing session needs what is
// unfinished, so the block has to carry the open loops by name.
func TestCheckpointRendersStatusNotNarrative(t *testing.T) {
	s, repo := gitTestStore(t)
	if _, err := s.NewNode("task", Frontmatter{Title: "Live work", Status: "doing"}, ""); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "a.go"), "a1\n")

	got := s.BuildCheckpoint().Render()

	for _, want := range []string{"Checkpoint", "In flight", "live-work", "Changed code", "a.go", "Where", "branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered checkpoint is missing %q:\n%s", want, got)
		}
	}
}

// TestCheckpointNamesUndeclaredCode: the block has to say whether the work is
// declared, because "there are changes" and "there are undeclared changes" are
// different handovers.
func TestCheckpointNamesUndeclaredCode(t *testing.T) {
	s, repo := gitTestStore(t)
	writeFile(t, filepath.Join(repo, "a.go"), "a1\n")

	c := s.BuildCheckpoint()
	if !c.Undeclared {
		t.Fatal("undeclared work is not reported as undeclared")
	}
	if !strings.Contains(c.Render(), "not yet declared") {
		t.Error("the rendered block does not tell the reader the code is undeclared")
	}
}

// TestCheckpointDedupesMemoryByPath: one file with several unresolved
// generations is one problem, and listing it three times reads as three.
func TestCheckpointDedupesMemoryByPath(t *testing.T) {
	c := Checkpoint{Memory: []MemoryObligation{
		{Path: "notes.md", State: MemPending},
		{Path: "notes.md", State: MemPending},
		{Path: "other.md", State: MemPending},
	}}

	got := c.Render()
	if n := strings.Count(got, "`notes.md`"); n != 1 {
		t.Errorf("notes.md listed %d times, want 1:\n%s", n, got)
	}
	if !strings.Contains(got, "`other.md`") {
		t.Error("deduping dropped a distinct path")
	}
}
