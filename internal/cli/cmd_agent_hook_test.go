package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// cliGitStore builds a store inside a real git repo under a temp HOME (so Claude
// memory resolution stays empty). Returns the store and repo root.
func cliGitStore(t *testing.T) (*core.Store, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	if real, err := filepath.EvalSymlinks(repo); err == nil {
		repo = real
	}
	tgit(t, repo, "init")
	tgit(t, repo, "config", "user.email", "t@example.com")
	tgit(t, repo, "config", "user.name", "Test")
	tgit(t, repo, "commit", "--allow-empty", "-m", "init")
	s, err := core.InitStore(filepath.Join(repo, "katra"), "CLI Git Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return s, repo
}

func tgit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestStopConversationalTurnNeverBlocks: a dirty repo with no Edit/Write observed
// this turn (a conversational / read-only turn) must always allow.
func TestStopConversationalTurnNeverBlocks(t *testing.T) {
	s, repo := cliGitStore(t)
	// Repo is dirty from work done outside this turn.
	write(t, filepath.Join(repo, "leftover.go"), "package main\n")

	// No RecordTouched → no Edit/Write observed this turn.
	if block, _ := stopDecision(s, hookInput{SessionID: "conv"}); block {
		t.Fatal("conversational turn blocked; must always allow")
	}
	// Even after a turn boundary with still no edits.
	_ = s.RecordTurnStart("conv")
	if block, _ := stopDecision(s, hookInput{SessionID: "conv"}); block {
		t.Fatal("read-only turn with dirty repo blocked; must always allow")
	}
}

// TestStopEditThenRevertNeverBlocks: an edit reverted within the same turn nets
// no working-tree change, so there is nothing to reconcile → allow.
func TestStopEditThenRevertNeverBlocks(t *testing.T) {
	s, repo := cliGitStore(t)
	app := filepath.Join(repo, "app.go")
	write(t, app, "package app\n")
	tgit(t, repo, "add", "app.go")
	tgit(t, repo, "commit", "-m", "app")

	// Edit (observed) then revert to the committed content.
	write(t, app, "package app\n\nfunc Tmp() {}\n")
	_ = s.RecordTouched("rev", "tool-1", app)
	write(t, app, "package app\n")

	if block, _ := stopDecision(s, hookInput{SessionID: "rev"}); block {
		t.Fatal("edit-then-revert blocked; net change is zero → must allow")
	}
}

// TestStopUnrelatedDirtyPlusRevertNeverBlocks pins § fix #4 (the highest-value
// fix): a pre-existing unrelated dirty file, plus this turn's edit-then-revert of
// a different file, nets no authored change — Stop must never block. The unit is
// this turn's touched paths ∩ their net change, so leftover.go (never touched)
// and app.go (reverted) both drop out.
func TestStopUnrelatedDirtyPlusRevertNeverBlocks(t *testing.T) {
	s, repo := cliGitStore(t)
	// A file committed so we can revert app.go back to its committed content.
	app := filepath.Join(repo, "app.go")
	write(t, app, "package app\n")
	tgit(t, repo, "add", "app.go")
	tgit(t, repo, "commit", "-m", "app")

	// Pre-existing unrelated dirt, authored OUTSIDE this turn.
	write(t, filepath.Join(repo, "leftover.go"), "package main\n// dirty from before\n")

	// This turn: a new turn boundary, then edit app.go and revert it.
	_ = s.RecordTurnStart("sess")
	write(t, app, "package app\n\nfunc Tmp() {}\n")
	_ = s.RecordTouched("sess", "tool-1", app)
	write(t, app, "package app\n") // revert to committed

	if block, _ := stopDecision(s, hookInput{SessionID: "sess"}); block {
		t.Fatal("edit-then-revert with unrelated pre-existing dirt blocked; must allow (§ fix #4)")
	}
}

// TestStopNewMemoryReblocksAfterFirstBlock pins § fix #9: after Stop blocks and
// the same work continues, a genuinely NEW memory obligation must re-block (be
// offered) rather than being suppressed by the work-generation watermark.
func TestStopNewMemoryReblocksAfterFirstBlock(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)

	// First block on the unresolved task.
	if block, _ := stopDecision(s, hookInput{SessionID: "work"}); !block {
		t.Fatal("first unresolved edit did not block")
	}
	// Resolve the task for this work generation so the ONLY thing that can block
	// next is a new obligation.
	report := s.EvaluateReconcileForSession("work")
	if err := s.WriteReceipt(core.ReconcileReceipt{
		WorkGenerationID: report.WorkGenerationID,
		Task:             core.TaskDecl{Kind: "none", Reason: "refactor"},
	}); err != nil {
		t.Fatal(err)
	}
	// Same work now allows.
	if block, _ := stopDecision(s, hookInput{SessionID: "work"}); block {
		t.Fatal("resolved work still blocked")
	}
	// A new memory generation appears for this repo (same code work generation).
	md, _ := s.MemoryDir()
	if err := os.MkdirAll(md, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(md, "note.md"),
		"---\nname: x\nmetadata:\n  node_type: memory\n  type: project\n---\n\na new note\n")
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	// The new memory obligation changes the blocking fingerprint → Stop re-blocks.
	if block, _ := stopDecision(s, hookInput{SessionID: "work"}); !block {
		t.Fatal("newly discovered memory obligation was suppressed by the watermark (§ fix #9)")
	}
}

// TestReadHookInputMalformedFailsOpen pins § fix #13: malformed/empty hook stdin
// yields an error so every event fails open (allow), never sharing the zero-value
// "default" session.
func TestReadHookInputMalformedFailsOpen(t *testing.T) {
	withStdin(t, "{ this is not json")
	if _, err := readHookInput(); err == nil {
		t.Fatal("malformed hook JSON must return an error (fail-open)")
	}
	withStdin(t, "   \n")
	if _, err := readHookInput(); err == nil {
		t.Fatal("empty hook stdin must return an error (fail-open)")
	}
	withStdin(t, `{"session_id":"s","hook_event_name":"Stop"}`)
	if in, err := readHookInput(); err != nil || in.SessionID != "s" {
		t.Fatalf("valid hook JSON: in=%+v err=%v", in, err)
	}
}

// withStdin redirects os.Stdin to a pipe carrying content for the duration of the
// test call, restoring it afterward.
func withStdin(t *testing.T, content string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
	})
}

// TestStopRealEditBlocksOnce: real Edit/Write with an unresolved unit blocks
// exactly once; a second Stop on the same unchanged work allows (watermark).
func TestStopRealEditBlocksOnce(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)

	block, reason := stopDecision(s, hookInput{SessionID: "work"})
	if !block {
		t.Fatal("real unresolved edit did not block")
	}
	if !strings.Contains(reason, "feature.go") || !strings.Contains(reason, "reconcile") {
		t.Errorf("reason missing path/instruction: %q", reason)
	}

	// Second Stop, same session, same work generation → allow (blocked once).
	if b2, _ := stopDecision(s, hookInput{SessionID: "work"}); b2 {
		t.Fatal("re-blocked the same work generation; must block only once")
	}
}

// TestStopHookActiveNeverBlocks: when Claude Code re-enters after a block
// (stop_hook_active), never block — no loops.
func TestStopHookActiveNeverBlocks(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)

	if block, _ := stopDecision(s, hookInput{SessionID: "work", StopHookActive: true}); block {
		t.Fatal("blocked with stop_hook_active=true; must always allow")
	}
}

// TestStopAfterReconcileAllows: once the agent declares a task (receipt written
// for the current work generation) and no memory is pending, Stop allows.
func TestStopAfterReconcileAllows(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)

	// Simulate `katra reconcile --advance` writing a receipt for this work gen.
	report := s.EvaluateReconcile()
	if err := s.WriteReceipt(core.ReconcileReceipt{
		WorkGenerationID: report.WorkGenerationID,
		Task:             core.TaskDecl{Kind: "advance", Slug: "some-task"},
	}); err != nil {
		t.Fatal(err)
	}
	// Fresh session so the allow is due to the receipt, not the watermark.
	if block, _ := stopDecision(s, hookInput{SessionID: "fresh"}); block {
		t.Fatal("blocked after reconciliation; a declared work gen must allow")
	}
}

// TestStopNoTaskDeclarationAllows: an explicit --no-task declaration resolves the
// checkpoint just like advance/close.
func TestStopNoTaskDeclarationAllows(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)

	report := s.EvaluateReconcile()
	if err := s.WriteReceipt(core.ReconcileReceipt{
		WorkGenerationID: report.WorkGenerationID,
		Task:             core.TaskDecl{Kind: "none", Reason: "pure refactor"},
	}); err != nil {
		t.Fatal(err)
	}
	if block, _ := stopDecision(s, hookInput{SessionID: "fresh"}); block {
		t.Fatal("--no-task declaration should allow the stop")
	}
}

// TestStopSkipResolvesUnit: --skip resolves this unit of work entirely.
func TestStopSkipResolvesUnit(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)

	report := s.EvaluateReconcile()
	if err := s.WriteReceipt(core.ReconcileReceipt{
		WorkGenerationID: report.WorkGenerationID,
		Skip:             true,
		SkipReason:       "throwaway",
	}); err != nil {
		t.Fatal(err)
	}
	if block, _ := stopDecision(s, hookInput{SessionID: "fresh"}); block {
		t.Fatal("--skip should allow the stop")
	}
}

// TestSnapshotNeverBlocks: PreCompact / SessionEnd snapshots never block or
// error, whether or not they write a checkpoint.
func TestSnapshotNeverBlocks(t *testing.T) {
	s, _ := cliGitStore(t)
	t.Setenv("KATRA_DIR", s.Dir)
	for _, ev := range []string{"pre-compact", "session-end", ""} {
		withStdin(t, `{"session_id":"s","hook_event_name":"PreCompact"}`)
		if err := hookSnapshotRun(ev); err != nil {
			t.Fatalf("snapshot(%q) returned error: %v", ev, err)
		}
	}
}

// TestSessionEndWritesNoCheckpoint: only pre-compact checkpoints. A session that
// ended has usually finished, and a checkpoint on every exit is the ceremony
// that gets a hook muted.
func TestSessionEndWritesNoCheckpoint(t *testing.T) {
	s, repo := cliGitStore(t)
	t.Setenv("KATRA_DIR", s.Dir)
	write(t, filepath.Join(repo, "a.go"), "a1\n")

	before, err := s.ListNodes("entry")
	if err != nil {
		t.Fatal(err)
	}
	withStdin(t, `{"session_id":"s","hook_event_name":"SessionEnd"}`)
	if err := hookSnapshotRun("session-end"); err != nil {
		t.Fatal(err)
	}
	after, err := s.ListNodes("entry")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("session-end wrote a checkpoint: %d entries before, %d after", len(before), len(after))
	}
}

// TestPreCompactWritesCheckpointForOpenLoops is the fix for the hook that stood
// on the right moment in silence. Compaction destroys context whether or not
// the session cooperates, so the derived half is written without being asked.
func TestPreCompactWritesCheckpointForOpenLoops(t *testing.T) {
	s, repo := cliGitStore(t)
	t.Setenv("KATRA_DIR", s.Dir)
	write(t, filepath.Join(repo, "a.go"), "a1\n")

	withStdin(t, `{"session_id":"s","hook_event_name":"PreCompact"}`)
	if err := hookSnapshotRun("pre-compact"); err != nil {
		t.Fatal(err)
	}

	entries, err := s.ListNodes("entry")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range entries {
		b, err := os.ReadFile(e.Path)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "### Checkpoint") && strings.Contains(string(b), "a.go") {
			found = true
		}
	}
	if !found {
		t.Error("pre-compact did not capture the open loops; the moment passed in silence again")
	}
}

// TestPreCompactSilentWhenNothingInFlight: the threshold, at the hook.
func TestPreCompactSilentWhenNothingInFlight(t *testing.T) {
	s, _ := cliGitStore(t)
	t.Setenv("KATRA_DIR", s.Dir)

	before, err := s.ListNodes("entry")
	if err != nil {
		t.Fatal(err)
	}
	withStdin(t, `{"session_id":"s","hook_event_name":"PreCompact"}`)
	if err := hookSnapshotRun("pre-compact"); err != nil {
		t.Fatal(err)
	}
	after, err := s.ListNodes("entry")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Error("pre-compact wrote a checkpoint with nothing in flight; this is how a hook gets muted")
	}
}

// TestPreCommitCoveragePassesWhenReconciled: staged code covered by a receipt
// passes the pre-commit coverage gate.
func TestPreCommitCoveragePassesWhenReconciled(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	tgit(t, repo, "add", "feature.go")

	id := s.CoverageReceiptID([]string{"feature.go"})
	if err := s.WriteReceipt(core.ReconcileReceipt{WorkGenerationID: id, Task: core.TaskDecl{Kind: "advance", Slug: "t"}}); err != nil {
		t.Fatal(err)
	}
	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m done"
	if preCommitBlocks(s, in) {
		t.Fatal("covered staged code was blocked at pre-commit")
	}
}

// TestPreCommitCoverageFailsWhenCodeChanged: staging new content after the
// receipt was written (the STAGED index blob differs from the receipt's work
// gen) fails coverage. Pre-commit fingerprints the index — the exact bytes the
// commit will record (§ fix #2) — so the change must be re-staged to matter.
func TestPreCommitCoverageFailsWhenCodeChanged(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	tgit(t, repo, "add", "feature.go")

	// Receipt written for the original staged content.
	id := s.CoverageReceiptID([]string{"feature.go"})
	if err := s.WriteReceipt(core.ReconcileReceipt{WorkGenerationID: id, Task: core.TaskDecl{Kind: "advance", Slug: "t"}}); err != nil {
		t.Fatal(err)
	}
	// Code changes AND is re-staged after the checkpoint → new index blob.
	write(t, feat, "package feature\n\nfunc New() {}\n")
	tgit(t, repo, "add", "feature.go")

	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m done"
	if !preCommitBlocks(s, in) {
		t.Fatal("post-checkpoint staged change should fail coverage at pre-commit")
	}
	// Bypass flags always allow.
	in.ToolInput.Command = "git commit --no-verify -m done"
	if preCommitBlocks(s, in) {
		t.Fatal("--no-verify must bypass the coverage gate")
	}
}

// TestPreCommitIndexNotWorkingTree pins the core of § fix #2: pre-commit
// validates the STAGED index, not the working tree. Staging A then editing the
// working tree to B (unstaged) must still PASS — the commit records A, which was
// reconciled — where the old working-tree fingerprint would false-block.
func TestPreCommitIndexNotWorkingTree(t *testing.T) {
	s, repo := cliGitStore(t)
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n") // content A
	tgit(t, repo, "add", "feature.go")

	// Reconcile the staged content A.
	id := s.CoverageReceiptID([]string{"feature.go"})
	if err := s.WriteReceipt(core.ReconcileReceipt{WorkGenerationID: id, Task: core.TaskDecl{Kind: "advance", Slug: "t"}}); err != nil {
		t.Fatal(err)
	}
	// Working tree drifts to B, but the change is NOT staged — the commit still
	// records A.
	write(t, feat, "package feature\n\nfunc Later() {}\n")

	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m done"
	if preCommitBlocks(s, in) {
		t.Fatal("unstaged working-tree drift must not block: the index (A) is covered")
	}
}

// TestPreCommitShellOnlyStagedCodeCaught: code staged via the shell (never
// observed via Edit/Write, never reconciled) is caught at pre-commit.
func TestPreCommitShellOnlyStagedCodeCaught(t *testing.T) {
	s, repo := cliGitStore(t)
	// The reconcile system is in use (some unrelated receipt exists).
	if err := s.WriteReceipt(core.ReconcileReceipt{WorkGenerationID: "unrelated", Task: core.TaskDecl{Kind: "none", Reason: "x"}}); err != nil {
		t.Fatal(err)
	}
	// Shell-generated code, staged, with no covering receipt.
	write(t, filepath.Join(repo, "generated.go"), "package generated\n")
	tgit(t, repo, "add", "generated.go")

	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m gen"
	if !preCommitBlocks(s, in) {
		t.Fatal("shell-only staged code should be caught at pre-commit")
	}
}

// TestPreCommitNoReceiptsLedgerNeverBlocks: before the reconcile system is used
// in a repo, the coverage gate never blocks (safety for non-agent commits).
func TestPreCommitNoReceiptsLedgerNeverBlocks(t *testing.T) {
	s, repo := cliGitStore(t)
	write(t, filepath.Join(repo, "code.go"), "package code\n")
	tgit(t, repo, "add", "code.go")

	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m code"
	if preCommitBlocks(s, in) {
		t.Fatal("coverage gate blocked with no receipts ledger; must fail-open")
	}
}

// TestCorruptReceiptsFailOpen pins § fix #7: a corrupt receipts ledger must NOT
// block — neither the Stop gate nor the pre-commit gate. A corrupt file was
// previously read as an empty ledger ("no declaration") and blocked.
func TestCorruptReceiptsFailOpen(t *testing.T) {
	s, repo := cliGitStore(t)
	// Corrupt the receipts ledger.
	if err := os.MkdirAll(s.StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.StateDir(), "reconcile-receipts.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stop: a real unresolved edit would normally block, but a corrupt ledger
	// makes evaluation non-applicable → allow.
	feat := filepath.Join(repo, "feature.go")
	write(t, feat, "package feature\n")
	_ = s.RecordTouched("work", "tool-1", feat)
	if block, _ := stopDecision(s, hookInput{SessionID: "work"}); block {
		t.Fatal("corrupt receipts blocked a Stop; must fail open (§ fix #7)")
	}

	// Pre-commit: corrupt ledger → coverage treated as satisfied → allow.
	tgit(t, repo, "add", "feature.go")
	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m done"
	if preCommitBlocks(s, in) {
		t.Fatal("corrupt receipts blocked a commit; must fail open (§ fix #7)")
	}
}

// TestPreCommitStoreOnlyStagedAllows: staging only the katra store (bookkeeping)
// is never blocked.
func TestPreCommitStoreOnlyStagedAllows(t *testing.T) {
	s, repo := cliGitStore(t)
	if err := s.WriteReceipt(core.ReconcileReceipt{WorkGenerationID: "u", Task: core.TaskDecl{Kind: "none", Reason: "x"}}); err != nil {
		t.Fatal(err)
	}
	// Stage only a store file.
	write(t, filepath.Join(s.Dir, "note.txt"), "bookkeeping\n")
	tgit(t, repo, "add", "katra/note.txt")

	in := hookInput{ToolName: "Bash"}
	in.ToolInput.Command = "git commit -m book"
	if preCommitBlocks(s, in) {
		t.Fatal("store-only staged bookkeeping should never be blocked")
	}
}

// TestCheckpointDoesNotLandInAnUnrelatedDraft is the regression for what
// happened on the feature's first live use: a session-wide checkpoint was
// appended to an entry opened hours earlier on a different subject, because
// --entry defaulted to the active draft. A checkpoint is session-scoped; a
// draft is subject-scoped.
func TestCheckpointDoesNotLandInAnUnrelatedDraft(t *testing.T) {
	s, repo := cliGitStore(t)
	t.Setenv("KATRA_DIR", s.Dir)

	unrelated, err := s.NewEntry(core.Frontmatter{Title: "Something else entirely"}, "opened hours ago")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, "a.go"), "a1\n")

	withStdin(t, `{"session_id":"s","hook_event_name":"PreCompact"}`)
	if err := hookSnapshotRun("pre-compact"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(unrelated.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "### Checkpoint") {
		t.Error("the checkpoint landed inside an unrelated draft")
	}

	// It must still have been captured somewhere.
	entries, err := s.ListNodes("entry")
	if err != nil {
		t.Fatal(err)
	}
	var captured bool
	for _, e := range entries {
		if e.Slug == unrelated.Slug {
			continue
		}
		if body, err := os.ReadFile(e.Path); err == nil && strings.Contains(string(body), "### Checkpoint") {
			captured = true
		}
	}
	if !captured {
		t.Error("avoiding the unrelated draft dropped the checkpoint entirely")
	}
}
