package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memTestStore builds a store whose Claude memory dir is a temp dir we control,
// so tests never touch the real ~/.claude. It stubs MemoryDir resolution by
// pointing HOME at a temp tree and encoding the store's repo path.
//
// Rather than fight git + $HOME encoding, tests call the scan internals through
// a small seam: we write memory files into a dir and drive ScanMemory via a
// store whose MemoryDir resolves there. We achieve that by setting HOME to a
// temp dir and creating the encoded project path for the store's parent dir.
func memTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := t.TempDir()
	storeDir := filepath.Join(repo, "katra")
	s, err := InitStore(storeDir, "Mem Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	// The store has no git repo, so MemoryDir falls back to filepath.Dir(s.Dir)
	// == repo. Build the encoded memory dir under our temp HOME for that path.
	memDir := filepath.Join(home, ".claude", "projects", encodeProjectPath(repo), "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir memDir: %v", err)
	}
	return s, memDir
}

func writeMem(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func projectMem(body string) string {
	return "---\nname: x\nmetadata:\n  node_type: memory\n  type: project\n---\n\n" + body
}

func typedMem(typ, body string) string {
	return "---\nname: x\nmetadata:\n  node_type: memory\n  type: " + typ + "\n---\n\n" + body
}

func TestScanNewAndIdempotent(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "alpha.md", projectMem("first note"))
	writeMem(t, memDir, "MEMORY.md", "- index, must be ignored")

	res, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if res.New != 1 || res.Appended != 0 || res.Rewritten != 0 {
		t.Fatalf("first scan = %+v, want New=1", res)
	}
	gens, _ := s.MemoryGenerations()
	if len(gens) != 1 {
		t.Fatalf("generations = %d, want 1 (MEMORY.md must be skipped)", len(gens))
	}
	if gens[0].State != MemPending || gens[0].Change != ChangeNew {
		t.Fatalf("gen = %+v, want pending/new", gens[0])
	}

	// Re-scan with no changes: idempotent, no new generations.
	res2, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if res2.New != 0 || res2.Appended != 0 || res2.Rewritten != 0 {
		t.Fatalf("rescan = %+v, want all zero", res2)
	}
	gens2, _ := s.MemoryGenerations()
	if len(gens2) != 1 {
		t.Fatalf("after rescan generations = %d, want 1", len(gens2))
	}
}

func TestScanAppendedVsRewritten(t *testing.T) {
	s, memDir := memTestStore(t)
	base := projectMem("line one\nline two\n")
	writeMem(t, memDir, "note.md", base)
	if _, err := s.ScanMemory(); err != nil {
		t.Fatalf("scan1: %v", err)
	}

	// Append: same prefix, larger size.
	writeMem(t, memDir, "note.md", base+"line three appended to grow the file well past the prefix boundary\n")
	res, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan2: %v", err)
	}
	if res.Appended != 1 || res.Rewritten != 0 || res.New != 0 {
		t.Fatalf("append scan = %+v, want Appended=1", res)
	}

	// Rewrite: change the beginning so the prefix hash differs.
	writeMem(t, memDir, "note.md", projectMem("completely different opening content that changes the prefix\n"))
	res2, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan3: %v", err)
	}
	if res2.Rewritten != 1 || res2.Appended != 0 {
		t.Fatalf("rewrite scan = %+v, want Rewritten=1", res2)
	}

	gens, _ := s.MemoryGenerations()
	if len(gens) != 3 {
		t.Fatalf("generations = %d, want 3 (new, appended, rewritten)", len(gens))
	}
}

func TestScanTypeFilter(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "proj.md", typedMem("project", "keep me"))
	writeMem(t, memDir, "ref.md", typedMem("reference", "skip me"))
	writeMem(t, memDir, "user.md", typedMem("user", "skip me"))
	writeMem(t, memDir, "fb.md", typedMem("feedback", "skip me"))
	writeMem(t, memDir, "none.md", "---\nname: x\nmetadata:\n  node_type: memory\n---\n\nno type, skip")

	res, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if res.New != 1 {
		t.Fatalf("scan = %+v, want New=1 (only project admitted)", res)
	}
	gens, _ := s.MemoryGenerations()
	if len(gens) != 1 || gens[0].Path != "proj.md" {
		t.Fatalf("generations = %+v, want only proj.md", gens)
	}
}

func TestQuarantineSecretAndTerm(t *testing.T) {
	s, memDir := memTestStore(t)
	s.Config.Memory.SensitiveTerms = []string{"ProjectRedstone"}

	writeMem(t, memDir, "aws.md", projectMem("here is a key AKIAIOSFODNN7EXAMPLE in text"))
	writeMem(t, memDir, "env.md", projectMem("API_KEY=sk_live_abcdef0123456789xyz"))
	writeMem(t, memDir, "code.md", projectMem("the codename is ProjectRedstone internally"))
	writeMem(t, memDir, "clean.md", projectMem("just a normal note about refactoring"))

	res, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if res.Quarantined != 3 {
		t.Fatalf("quarantined = %d, want 3", res.Quarantined)
	}
	gens, _ := s.MemoryGenerations()
	states := map[string]string{}
	for _, g := range gens {
		states[g.Path] = g.State
	}
	for _, name := range []string{"aws.md", "env.md", "code.md"} {
		if states[name] != MemQuarantined {
			t.Errorf("%s state = %q, want quarantined", name, states[name])
		}
	}
	if states["clean.md"] != MemPending {
		t.Errorf("clean.md state = %q, want pending", states["clean.md"])
	}
}

func TestResolveAndIgnore(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "a.md", projectMem("alpha"))
	writeMem(t, memDir, "b.md", projectMem("beta"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if _, err := s.ResolveMemory("a.md", MemImported, "authored"); err != nil {
		t.Fatalf("resolve imported: %v", err)
	}
	if _, err := s.ResolveMemory("b.md", MemIgnored, "noise"); err != nil {
		t.Fatalf("resolve ignored: %v", err)
	}
	if _, err := s.ResolveMemory("missing.md", MemImported, ""); err != os.ErrNotExist {
		t.Fatalf("resolve missing err = %v, want ErrNotExist", err)
	}

	gens, _ := s.MemoryGenerations()
	got := map[string]string{}
	for _, g := range gens {
		got[g.Path] = g.State
	}
	if got["a.md"] != MemImported || got["b.md"] != MemIgnored {
		t.Fatalf("states = %+v, want a imported / b ignored", got)
	}
}

func TestScanSkipsWhenNoMemoryDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	s, err := InitStore(filepath.Join(repo, "katra"), "No Mem")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	res, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan should be fail-open, got err: %v", err)
	}
	if !res.Skipped {
		t.Fatalf("res = %+v, want Skipped=true when no memory dir", res)
	}
}

func TestScanDisabled(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "a.md", projectMem("alpha"))
	no := false
	s.Config.Memory.Enabled = &no
	res, err := s.ScanMemory()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !res.Skipped {
		t.Fatalf("res = %+v, want Skipped when disabled", res)
	}
}

func TestLedgerGitIgnored(t *testing.T) {
	s, memDir := memTestStore(t)
	writeMem(t, memDir, "a.md", projectMem("alpha"))
	if _, err := s.ScanMemory(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Ledger file exists in .state.
	if _, err := os.Stat(s.ledgerPath()); err != nil {
		t.Fatalf("ledger missing: %v", err)
	}
	// .state/ is git-ignored via the store .gitignore.
	gi, err := os.ReadFile(filepath.Join(s.Dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), ".state/") {
		t.Fatalf(".gitignore = %q, want to contain .state/", string(gi))
	}
}
