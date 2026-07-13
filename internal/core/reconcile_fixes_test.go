package core

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestConcurrentResolveNoLostUpdates pins § fix #1: many overlapping
// read-modify-writes of the memory ledger all land, because every RMW goes
// through the shared state lock. Without the lock the last writer clobbers the
// others (run with -race to also catch the data race).
func TestConcurrentResolveNoLostUpdates(t *testing.T) {
	s, memDir := memTestStore(t)
	const n = 24
	names := make([]string, n)
	for i := 0; i < n; i++ {
		name := "note" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".md"
		names[i] = name
		writeMem(t, memDir, name, projectMem("content "+name))
	}
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	gens, _ := s.MemoryGenerations()
	if len(gens) != n {
		t.Fatalf("scanned %d generations, want %d", len(gens), n)
	}

	var wg sync.WaitGroup
	for i := range gens {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, err := s.ResolveMemory(id, MemImported, "concurrent"); err != nil {
				t.Errorf("resolve %s: %v", id, err)
			}
		}(gens[i].ID)
	}
	wg.Wait()

	after, _ := s.MemoryGenerations()
	if len(after) != n {
		t.Fatalf("after concurrent resolve: %d generations, want %d (lost updates)", len(after), n)
	}
	for _, g := range after {
		if g.State != MemImported {
			t.Fatalf("generation %s state = %q, want imported (a concurrent update was lost)", g.Path, g.State)
		}
	}
}

// TestConcurrentClaimStopBlockSingleWinner pins § fix #8: two processes racing to
// block the same obligation set — only one wins the compare-and-set.
func TestConcurrentClaimStopBlockSingleWinner(t *testing.T) {
	s, _ := gitTestStore(t)
	const workers = 16
	var wins int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.ClaimStopBlock("sess", "wg-1", "fp-1", nil) {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("concurrent ClaimStopBlock winners = %d, want exactly 1", wins)
	}
	// A different fingerprint (e.g. a newly discovered obligation) wins again.
	if !s.ClaimStopBlock("sess", "wg-1", "fp-2", nil) {
		t.Fatal("a changed obligation fingerprint should win a fresh block")
	}
}

// TestIndexAndWorkingTreeFingerprintAgree pins § fix #2: the pre-commit index
// fingerprint equals the Stop working-tree fingerprint when the staged content
// matches the working tree, so a receipt written at reconcile time is found at
// commit time.
func TestIndexAndWorkingTreeFingerprintAgree(t *testing.T) {
	s, repo := gitTestStore(t)
	writeFile(t, filepath.Join(repo, "feature.go"), "package feature\n")
	root, _ := s.RepoRoot()

	wt := s.workGenIDWorkingTree(root, []string{"feature.go"})
	runGit(t, repo, "add", "feature.go")
	idx := s.CoverageReceiptID([]string{"feature.go"})

	if wt == "" || idx == "" {
		t.Fatalf("empty fingerprints: wt=%q idx=%q", wt, idx)
	}
	if wt != idx {
		t.Fatalf("index fingerprint %q != working-tree fingerprint %q for identical staged content", idx, wt)
	}
}

// TestIndexFingerprintTracksStagedNotWorkingTree pins the other half of § fix #2:
// editing the working tree without staging does NOT change the index fingerprint
// (the commit records the staged bytes).
func TestIndexFingerprintTracksStagedNotWorkingTree(t *testing.T) {
	s, repo := gitTestStore(t)
	feat := filepath.Join(repo, "feature.go")
	writeFile(t, feat, "package feature\n")
	runGit(t, repo, "add", "feature.go")

	before := s.CoverageReceiptID([]string{"feature.go"})
	writeFile(t, feat, "package feature\n\nfunc B() {}\n") // unstaged drift
	after := s.CoverageReceiptID([]string{"feature.go"})
	if before != after {
		t.Fatalf("index fingerprint changed on unstaged edit: %q → %q", before, after)
	}
	runGit(t, repo, "add", "feature.go") // now stage it → id must move
	if staged := s.CoverageReceiptID([]string{"feature.go"}); staged == before {
		t.Fatal("index fingerprint unchanged after staging new content")
	}
}

// TestFingerprintModeAndDeletionDistinct pins § fix #10: a mode-only change and a
// deletion each produce a distinct id rather than colliding with the content hash
// / a bare "absent" marker.
func TestFingerprintModeAndDeletionDistinct(t *testing.T) {
	s, repo := gitTestStore(t)
	root, _ := s.RepoRoot()
	p := filepath.Join(repo, "s.sh")
	writeFile(t, p, "#!/bin/sh\necho hi\n")
	runGit(t, repo, "add", "s.sh")
	runGit(t, repo, "commit", "-m", "add script")

	mode644 := s.workGenIDWorkingTree(root, []string{"s.sh"})
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	mode755 := s.workGenIDWorkingTree(root, []string{"s.sh"})
	if mode644 == mode755 {
		t.Fatal("mode-only change produced the same fingerprint (mode ignored)")
	}
	// Deletion is distinct from any content/mode record.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	del := s.workGenIDWorkingTree(root, []string{"s.sh"})
	if del == mode644 || del == mode755 {
		t.Fatal("deletion collided with a content/mode fingerprint")
	}
}

// TestReconcileCloseRequiresDraft pins § fix #3: declaring a close with no active
// draft errors (rather than silently orphaning the task), and with a draft it
// records the closure durably on the draft's `closes:`.
func TestReconcileCloseRequiresDraft(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("task", Frontmatter{Title: "Do It", Status: "doing"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileClose("do-it"); err == nil {
		t.Fatal("reconcile --close with no active draft must error")
	}
	// With an active draft the closure is attached to it.
	if _, err := s.NewEntry(Frontmatter{Title: "Work Log"}, "body"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileClose("do-it"); err != nil {
		t.Fatalf("reconcile --close with a draft: %v", err)
	}
	d, _ := s.ActiveDraft()
	if d == nil || len(d.FM.Closes) != 1 || d.FM.Closes[0] != "do-it" {
		t.Fatalf("draft closes = %+v, want [do-it]", d)
	}
}

// TestPublishEntryRecoverableAfterStampFailure pins § fix #5: closes are applied
// before the stamp, so a failed stamp leaves the entry a draft and a retry
// completes idempotently. Mutated also lists every written file (§ fix #6).
func TestPublishEntryRecoverableAfterStampFailure(t *testing.T) {
	s, repo := gitTestStore(t)
	if _, err := s.NewNode("epic", Frontmatter{Title: "Ship", Status: "active"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "Only Task", Status: "doing", Epic: "ship"}, ""); err != nil {
		t.Fatal(err)
	}
	entry, err := s.NewEntry(Frontmatter{Title: "Done", Closes: []string{"only-task"}}, "body")
	if err != nil {
		t.Fatal(err)
	}

	// First publish with a bogus hash → Stamp fails, but closes already applied.
	if _, err := s.PublishEntry(entry, []string{"deadbeef-not-a-real-hash"}); err == nil {
		t.Fatal("publish with a bad hash should error")
	}
	if !entry.IsDraft() {
		t.Fatal("entry must remain a draft after a failed stamp (so the retry finds it)")
	}
	if task, _ := s.GetNode("only-task"); task.FM.Status != "done" {
		t.Fatal("closes must be applied before the stamp so a retry recovers")
	}

	// Retry with a real commit → succeeds idempotently and reports every mutated file.
	writeFile(t, filepath.Join(repo, "main.go"), "package main\n")
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "code")
	head, _ := s.HeadHash()
	res, err := s.PublishEntry(entry, []string{head})
	if err != nil {
		t.Fatalf("retry publish: %v", err)
	}
	if !res.Stamped || len(res.Closed) != 1 {
		t.Fatalf("retry res = %+v, want stamped + 1 closed", res)
	}
	// Mutated lists every file the (idempotent) retry wrote: the entry and the
	// closed task. The epic was already rolled up in the first, failed call, so a
	// correct idempotent retry reports no further epic change.
	names := map[string]bool{}
	for _, p := range res.Mutated {
		names[filepath.Base(filepath.Dir(p))] = true // parent dir: entries/tasks/epics
	}
	for _, want := range []string{"entries", "tasks"} {
		if !names[want] {
			t.Errorf("Mutated missing a %s file: %v", want, res.Mutated)
		}
	}
	if epic, _ := s.GetNode("ship"); epic.FM.Status != "done" {
		t.Errorf("epic status = %q, want done (rolled up during publish)", epic.FM.Status)
	}
}

// TestPublishEntryMutatedIncludesEpic pins § fix #6 on the clean path: a single
// successful publish lists the entry, task, and rolled-up epic files.
func TestPublishEntryMutatedIncludesEpic(t *testing.T) {
	s, repo := gitTestStore(t)
	if _, err := s.NewNode("epic", Frontmatter{Title: "Ship", Status: "active"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "Only Task", Status: "doing", Epic: "ship"}, ""); err != nil {
		t.Fatal(err)
	}
	entry, err := s.NewEntry(Frontmatter{Title: "Done", Closes: []string{"only-task"}}, "body")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "main.go"), "package main\n")
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "code")
	head, _ := s.HeadHash()
	res, err := s.PublishEntry(entry, []string{head})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, p := range res.Mutated {
		names[filepath.Base(filepath.Dir(p))] = true
	}
	for _, want := range []string{"entries", "tasks", "epics"} {
		if !names[want] {
			t.Errorf("Mutated missing a %s file: %v", want, res.Mutated)
		}
	}
}

// TestBlockingFingerprintReflectsObligations pins § fix #9: the fingerprint
// changes when the blocking obligation set changes (e.g. new memory), so Stop
// re-blocks to surface it, and is stable when nothing changed.
func TestBlockingFingerprintReflectsObligations(t *testing.T) {
	base := ReconcileReport{
		WorkGenerationID: "wg",
		Blocking:         []Issue{{Kind: "task", Message: "declare the task"}},
	}
	same := ReconcileReport{
		WorkGenerationID: "wg",
		Blocking:         []Issue{{Kind: "task", Message: "declare the task"}},
	}
	if BlockingFingerprint(base) != BlockingFingerprint(same) {
		t.Fatal("fingerprint not stable for an identical obligation set")
	}
	withMem := ReconcileReport{
		WorkGenerationID: "wg",
		Blocking: []Issue{
			{Kind: "task", Message: "declare the task"},
			{Kind: "memory", Message: "resolve memory abc… (note.md)"},
		},
	}
	if BlockingFingerprint(base) == BlockingFingerprint(withMem) {
		t.Fatal("fingerprint unchanged after a new memory obligation appeared")
	}
}

// TestResolveMemoryByPrefixAndUnresolvedBasename pins § fix #11: a unique id
// prefix resolves precisely, and the basename fallback targets the newest
// UNRESOLVED generation rather than clobbering an already-resolved newer one.
func TestResolveMemoryByPrefixAndUnresolvedBasename(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "note.md", projectMem("v1"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	writeMem(t, memDir, "note.md", projectMem("v2 completely different"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	gens, _ := s.MemoryGenerations()
	if len(gens) != 2 {
		t.Fatalf("generations = %d, want 2", len(gens))
	}
	// Resolve the newer generation by a unique id prefix.
	if _, err := s.ResolveMemory(gens[1].ID[:10], MemImported, "by-prefix"); err != nil {
		t.Fatalf("resolve by prefix: %v", err)
	}
	gens, _ = s.MemoryGenerations()
	if gens[1].State != MemImported {
		t.Fatalf("newer gen state = %q, want imported (resolved by prefix)", gens[1].State)
	}
	// Basename fallback now targets the older UNRESOLVED generation, not the
	// already-resolved newer one.
	if _, err := s.ResolveMemory("note.md", MemIgnored, "by-name"); err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	gens, _ = s.MemoryGenerations()
	if gens[0].State != MemIgnored {
		t.Fatalf("older gen state = %q, want ignored (basename → newest unresolved)", gens[0].State)
	}
	if gens[1].State != MemImported {
		t.Fatalf("newer gen state = %q, want still imported (not clobbered)", gens[1].State)
	}
}
