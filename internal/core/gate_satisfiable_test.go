package core

import (
	"path/filepath"
	"testing"
)

// The regression suite for docs/design/unsatisfiable-gate.md.
//
// The bug was not that the gate fired wrongly — it was that it could not be
// satisfied at all, so every session it blocked either lost its commits or
// learned to use --no-verify. These tests assert the property that was missing:
// after declaring, the commit is allowed.

// declare records a no-task receipt for whatever reconcile currently sees, the
// way `katra reconcile --no-task --reason …` does.
func declare(t *testing.T, s *Store, reason string) {
	t.Helper()
	r := s.EvaluateReconcile()
	if err := s.WriteReceipt(ReconcileReceipt{
		WorkGenerationID: r.WorkGenerationID,
		TouchedPaths:     r.TouchedPaths,
		Task:             TaskDecl{Kind: "none", Reason: reason},
	}); err != nil {
		t.Fatalf("WriteReceipt: %v", err)
	}
}

func stagedWork(t *testing.T, s *Store) []string {
	t.Helper()
	staged, err := s.StagedFiles()
	if err != nil {
		t.Fatalf("StagedFiles: %v", err)
	}
	var out []string
	for _, f := range staged {
		if s.IsWorkProduct(f) {
			out = append(out, f)
		}
	}
	return out
}

// TestGateSatisfiableAfterEarlierCommitInSameTurn is sequence 1: a second unit
// of work in the same turn, produced by something other than Edit/Write. The
// session's touched set still names the already-committed file, so reconcile
// used to attribute nothing and report "no changed code" while the gate
// demanded a receipt for the staged file — mutually exclusive, and declaring
// wrote a receipt over the empty set (the sha256 of "") that covered nothing.
func TestGateSatisfiableAfterEarlierCommitInSameTurn(t *testing.T) {
	s, repo := gitTestStore(t)

	// Unit 1: authored through Edit, declared, committed.
	writeFile(t, filepath.Join(repo, "x.go"), "x1\n")
	if err := s.RecordTouched("s1", "t1", filepath.Join(repo, "x.go")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "x.go")
	declare(t, s, "unit 1")
	runGit(t, repo, "commit", "-m", "x")

	// Unit 2: arrives from a shell command, so nothing records a touch.
	writeFile(t, filepath.Join(repo, "y.go"), "y1\n")
	runGit(t, repo, "add", "y.go")

	if got := s.EvaluateReconcile().TouchedPaths; len(got) == 0 {
		t.Fatal("reconcile sees no work while y.go is staged; declaring cannot help")
	}
	if s.CoverageSatisfied(stagedWork(t, s)) {
		t.Fatal("undeclared staged work should not be covered")
	}

	declare(t, s, "unit 2")

	if !s.CoverageSatisfied(stagedWork(t, s)) {
		t.Error("the gate is still blocking after declaring — it cannot be satisfied")
	}
}

// TestGateSatisfiableOnPartialStage is sequence 2, and the more common one: edit
// two files, stage one, commit that part first. A receipt declaring {a,b} covers
// a commit of {a} — committing a subset of declared work is still declared work.
func TestGateSatisfiableOnPartialStage(t *testing.T) {
	s, repo := gitTestStore(t)

	writeFile(t, filepath.Join(repo, "a.go"), "a1\n")
	writeFile(t, filepath.Join(repo, "b.go"), "b1\n")
	for i, p := range []string{"a.go", "b.go"} {
		if err := s.RecordTouched("s1", string(rune('a'+i)), filepath.Join(repo, p)); err != nil {
			t.Fatal(err)
		}
	}
	declare(t, s, "both files")

	runGit(t, repo, "add", "a.go")
	if !s.CoverageSatisfied(stagedWork(t, s)) {
		t.Error("a receipt covering {a.go, b.go} must cover a commit of {a.go}")
	}
}

// TestGateStillBlocksUndeclaredWork: the gate has to keep doing its job.
func TestGateStillBlocksUndeclaredWork(t *testing.T) {
	s, repo := gitTestStore(t)

	writeFile(t, filepath.Join(repo, "c.go"), "c1\n")
	runGit(t, repo, "add", "c.go")

	if s.CoverageSatisfied(stagedWork(t, s)) {
		t.Error("undeclared staged work was allowed through the gate")
	}
}

// TestGateReblocksOnChangedContent is the property most at risk from making
// coverage per-path: declaring must cover the content declared, not the path
// forever. Edit after declaring and the gate must ask again.
func TestGateReblocksOnChangedContent(t *testing.T) {
	s, repo := gitTestStore(t)

	writeFile(t, filepath.Join(repo, "c.go"), "c1\n")
	if err := s.RecordTouched("s1", "t1", filepath.Join(repo, "c.go")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "c.go")
	declare(t, s, "c")
	if !s.CoverageSatisfied(stagedWork(t, s)) {
		t.Fatal("declared work should be covered")
	}

	writeFile(t, filepath.Join(repo, "c.go"), "c1-CHANGED\n")
	runGit(t, repo, "add", "c.go")

	if s.CoverageSatisfied(stagedWork(t, s)) {
		t.Error("content changed after declaring, but the gate still considers it covered")
	}
}

// TestUncoveredPathsNamesOnlyTheUndeclared backs the gate's message: it can only
// tell someone which paths are the problem if it knows.
func TestUncoveredPathsNamesOnlyTheUndeclared(t *testing.T) {
	s, repo := gitTestStore(t)

	writeFile(t, filepath.Join(repo, "a.go"), "a1\n")
	if err := s.RecordTouched("s1", "t1", filepath.Join(repo, "a.go")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "a.go")
	declare(t, s, "a only")

	writeFile(t, filepath.Join(repo, "z.go"), "z1\n")
	runGit(t, repo, "add", "z.go")

	got := s.UncoveredPaths(stagedWork(t, s))
	if len(got) != 1 || got[0] != "z.go" {
		t.Errorf("UncoveredPaths = %v, want exactly [z.go]", got)
	}
}
