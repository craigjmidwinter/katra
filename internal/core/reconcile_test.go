package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitTestStore builds a store inside a real, initialized git repo under a temp
// HOME (so Claude memory resolution stays empty/controlled). It returns the
// store and the repo root. An initial commit gives HEAD a value.
func gitTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	// Resolve symlinks so paths match git's view (macOS /var vs /private/var).
	if real, err := filepath.EvalSymlinks(repo); err == nil {
		repo = real
	}
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "t@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "commit", "--allow-empty", "-m", "init")

	s, err := InitStore(filepath.Join(repo, "katra"), "Git Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return s, repo
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMemoryResolvesByIDNotFilename pins the §C fix: an older pending generation
// of a rewritten file resolves by its own id and is not stranded by resolving
// the newest generation via filename.
func TestMemoryResolvesByIDNotFilename(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "note.md", projectMem("first version"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	writeMem(t, memDir, "note.md", projectMem("second totally different version"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	gens, _ := s.MemoryGenerations()
	if len(gens) != 2 {
		t.Fatalf("generations = %d, want 2", len(gens))
	}
	older := gens[0]

	// Resolve the OLDER generation by its id.
	if _, err := s.ResolveMemory(older.ID, MemImported, "authored"); err != nil {
		t.Fatalf("resolve by id: %v", err)
	}
	gens, _ = s.MemoryGenerations()
	if gens[0].State != MemImported {
		t.Errorf("older gen state = %q, want imported (resolved by id)", gens[0].State)
	}
	if gens[1].State != MemPending {
		t.Errorf("newer gen state = %q, want still pending", gens[1].State)
	}

	// Basename fallback still resolves the newest generation.
	if _, err := s.ResolveMemory("note.md", MemIgnored, "noise"); err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	gens, _ = s.MemoryGenerations()
	if gens[1].State != MemIgnored {
		t.Errorf("newest gen state = %q, want ignored (resolved by name)", gens[1].State)
	}
}

// TestEvaluateReconcileMemoryObligation checks unresolved memory blocks a
// checkpoint, that offering moves it pending → offered (still blocking), and
// that resolving it clears the obligation.
func TestEvaluateReconcileMemoryObligation(t *testing.T) {
	s, repo := gitTestStore(t)
	// Seed a pending memory generation for this repo.
	memDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", encodeProjectPath(repo), "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(memDir, "note.md"), projectMem("a project note worth authoring"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatal(err)
	}
	// Real changed code so the checkpoint applies.
	writeFile(t, filepath.Join(repo, "feature.go"), "package feature\n")

	r := s.EvaluateReconcile()
	if len(r.Memory) != 1 || r.Memory[0].State != MemPending {
		t.Fatalf("memory obligations = %+v, want one pending", r.Memory)
	}
	hasMem, hasTask := false, false
	for _, b := range r.Blocking {
		if b.Kind == "memory" {
			hasMem = true
		}
		if b.Kind == "task" {
			hasTask = true
		}
	}
	if !hasMem || !hasTask {
		t.Fatalf("blocking = %+v, want both task and memory", r.Blocking)
	}

	// Offer it → pending becomes offered, still an obligation.
	if err := s.OfferMemory([]string{r.Memory[0].ID}, r.WorkGenerationID); err != nil {
		t.Fatal(err)
	}
	r2 := s.EvaluateReconcile()
	if len(r2.Memory) != 1 || r2.Memory[0].State != MemOffered {
		t.Fatalf("after offer, memory = %+v, want one offered", r2.Memory)
	}

	// Resolve it + declare a skip → checkpoint clears.
	if _, err := s.ResolveMemory(r2.Memory[0].ID, MemImported, "authored"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteReceipt(ReconcileReceipt{WorkGenerationID: r2.WorkGenerationID, Skip: true, SkipReason: "x"}); err != nil {
		t.Fatal(err)
	}
	r3 := s.EvaluateReconcile()
	if len(r3.Blocking) != 0 {
		t.Fatalf("after resolve+skip, blocking = %+v, want none", r3.Blocking)
	}
}

// TestReconcileAdvanceRollsUpEpic checks advance moves a todo task to doing and
// rolls up its epic (planned → active) as an invariant.
func TestReconcileAdvanceRollsUpEpic(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("epic", Frontmatter{Title: "Big Epic", Status: "planned"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "Do Thing", Status: "todo", Epic: "big-epic"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileAdvance("do-thing"); err != nil {
		t.Fatalf("ReconcileAdvance: %v", err)
	}
	task, _ := s.GetNode("do-thing")
	if task.FM.Status != "doing" {
		t.Errorf("task status = %q, want doing", task.FM.Status)
	}
	epic, _ := s.GetNode("big-epic")
	if epic.FM.Status != "active" {
		t.Errorf("epic status = %q, want active (rolled up)", epic.FM.Status)
	}
	// Advancing a non-task errors.
	if err := s.ReconcileAdvance("big-epic"); err == nil {
		t.Error("advancing an epic: want error, got nil")
	}
}

// TestReconcileAdvanceFromSpecced checks a specced task (a spec exists but
// implementation hasn't started) also advances to doing — the design's
// "" | todo | specced -> doing rule.
func TestReconcileAdvanceFromSpecced(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("task", Frontmatter{Title: "Specced Thing", Status: "specced", Spec: "docs/design/x.md"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileAdvance("specced-thing"); err != nil {
		t.Fatalf("ReconcileAdvance: %v", err)
	}
	task, _ := s.GetNode("specced-thing")
	if task.FM.Status != "doing" {
		t.Errorf("task status = %q, want doing", task.FM.Status)
	}
}

// TestPublishEntryClosesAndRollsUp verifies the shared publish op stamps, closes
// declared tasks, links them, and rolls up their epic — all in one call.
func TestPublishEntryClosesAndRollsUp(t *testing.T) {
	s, repo := gitTestStore(t)
	if _, err := s.NewNode("epic", Frontmatter{Title: "Ship It", Status: "active"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "Last Task", Status: "doing", Epic: "ship-it"}, ""); err != nil {
		t.Fatal(err)
	}
	entry, err := s.NewEntry(Frontmatter{Title: "Shipped", Closes: []string{"last-task"}}, "body")
	if err != nil {
		t.Fatal(err)
	}
	// A real commit touching code so Stamp has a hash + stat.
	writeFile(t, filepath.Join(repo, "main.go"), "package main\n")
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "code")
	head, err := s.HeadHash()
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.PublishEntry(entry, []string{head})
	if err != nil {
		t.Fatalf("PublishEntry: %v", err)
	}
	if !res.Stamped || len(res.Closed) != 1 || res.Closed[0] != "last-task" {
		t.Fatalf("res = %+v, want stamped + closed last-task", res)
	}
	if len(res.Epics) != 1 || res.Epics[0] != "ship-it" {
		t.Fatalf("rolled epics = %v, want [ship-it]", res.Epics)
	}
	task, _ := s.GetNode("last-task")
	if task.FM.Status != "done" || task.FM.Entry != entry.Slug {
		t.Errorf("task = %+v, want done + linked to %s", task.FM, entry.Slug)
	}
	epic, _ := s.GetNode("ship-it")
	if epic.FM.Status != "done" {
		t.Errorf("epic status = %q, want done (all children done)", epic.FM.Status)
	}
	// The stamped entry is no longer a draft, so a repeat publish path (post-commit)
	// would find no active draft — no double close.
	if entry.IsDraft() {
		t.Error("entry still a draft after publish")
	}
}

// TestWorkGenStableAndContentSensitive checks the work-generation id is stable
// for unchanged work and changes when the code content changes.
func TestWorkGenStableAndContentSensitive(t *testing.T) {
	s, repo := gitTestStore(t)
	writeFile(t, filepath.Join(repo, "app.go"), "package app\n")

	r1 := s.EvaluateReconcile()
	if !r1.Applicable || len(r1.TouchedPaths) != 1 || r1.TouchedPaths[0] != "app.go" {
		t.Fatalf("r1 = %+v, want applicable with app.go touched", r1)
	}
	r2 := s.EvaluateReconcile()
	if r1.WorkGenerationID != r2.WorkGenerationID {
		t.Errorf("workgen not stable: %s vs %s", r1.WorkGenerationID, r2.WorkGenerationID)
	}
	// Change content → id changes.
	writeFile(t, filepath.Join(repo, "app.go"), "package app\n\nfunc F() {}\n")
	r3 := s.EvaluateReconcile()
	if r3.WorkGenerationID == r1.WorkGenerationID {
		t.Error("workgen unchanged after content change; want different id")
	}
	// Store-only changes never count as touched code.
	if _, err := s.NewNode("task", Frontmatter{Title: "noise"}, ""); err != nil {
		t.Fatal(err)
	}
	r4 := s.EvaluateReconcile()
	for _, p := range r4.TouchedPaths {
		if s.IsStorePath(p) {
			t.Errorf("store path leaked into touched: %s", p)
		}
	}
}
