package core

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestThresholdSeesCommittedUndocumentedWork is the regression for the blindness
// the steward found on the first live use. A session that did a full day of work
// and committed as it went has a clean tree, so InFlight is empty and the first
// three inputs all read zero — at exactly the moment it is fullest.
func TestThresholdSeesCommittedUndocumentedWork(t *testing.T) {
	s, repo := gitTestStore(t)

	for _, name := range []string{"a.md", "b.md", "c.md"} {
		writeFile(t, filepath.Join(repo, name), "work\n")
		runGit(t, repo, "add", name)
		runGit(t, repo, "commit", "-m", "did "+name)
	}

	c := s.BuildCheckpoint()
	if len(c.InFlight) != 0 {
		t.Fatalf("precondition: tree should be clean, got %v", c.InFlight)
	}
	if c.Undocumented == 0 {
		t.Fatal("three committed, unchronicled commits count as zero undocumented work")
	}
	if !c.HasOpenLoops() {
		t.Error("a day of committed work with nothing written down reads as 'nothing to lose'")
	}
	if !strings.Contains(c.Render(), "Undocumented work") {
		t.Error("the block does not tell the reader why it fired")
	}
}

// TestChroniclingClearsTheCounter is the property that keeps the new input from
// being permanently loud: it must fall to zero when someone does the thing the
// hook is asking for.
func TestChroniclingClearsTheCounter(t *testing.T) {
	s, repo := gitTestStore(t)
	writeFile(t, filepath.Join(repo, "a.md"), "work\n")
	runGit(t, repo, "add", "a.md")
	runGit(t, repo, "commit", "-m", "did a")

	if s.BuildCheckpoint().Undocumented == 0 {
		t.Fatal("precondition: the commit should count as undocumented")
	}

	// Chronicle it: an entry stamped with that commit.
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if _, err := s.NewNode("entry", Frontmatter{Title: "Wrote it down", Hash: head}, "body"); err != nil {
		t.Fatal(err)
	}

	if got := s.BuildCheckpoint().Undocumented; got != 0 {
		t.Errorf("Undocumented = %d after chronicling the commit, want 0", got)
	}
}

// TestStoreOnlyCommitsAreNotUndocumentedWork: a commit that only touches katra/
// *is* chronicling, so counting it would mean the counter could never be
// cleared by writing.
func TestStoreOnlyCommitsAreNotUndocumentedWork(t *testing.T) {
	s, repo := gitTestStore(t)
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "store only")

	if got := s.BuildCheckpoint().Undocumented; got != 0 {
		t.Errorf("Undocumented = %d for a store-only commit, want 0", got)
	}
}

// TestCheckpointEntryIsReusedNotScattered: one checkpoint entry per day, rather
// than a new one on every compaction of a long session.
func TestCheckpointEntryIsReusedNotScattered(t *testing.T) {
	s, _ := gitTestStore(t)
	at := time.Now()

	if s.CheckpointEntry(at) != nil {
		t.Fatal("precondition: no checkpoint entry should exist yet")
	}
	made, err := s.NewEntry(Frontmatter{Title: CheckpointTitle(at)}, "x")
	if err != nil {
		t.Fatal(err)
	}
	got := s.CheckpointEntry(at)
	if got == nil || got.Slug != made.Slug {
		t.Errorf("CheckpointEntry did not find today's checkpoint entry (got %v)", got)
	}
}
