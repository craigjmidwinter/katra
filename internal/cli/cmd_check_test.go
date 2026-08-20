package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// stageCode writes and stages a non-store (code) file, which is the only thing
// that arms the gate.
func stageCode(t *testing.T, repo, name string) {
	t.Helper()
	write(t, filepath.Join(repo, name), "package main\n")
	tgit(t, repo, "add", name)
}

// TestGateBlocksWithoutADraft is the baseline: staged code, no draft anywhere.
func TestGateBlocksWithoutADraft(t *testing.T) {
	s, repo := cliGitStore(t)
	stageCode(t, repo, "main.go")

	if got := gateFor(s); got != gateNoDraft {
		t.Fatalf("gateFor = %v, want gateNoDraft", got)
	}
	if msg := gateNoDraft.message(); !strings.Contains(msg, "katra new") {
		t.Errorf("no-draft message should point at `katra new`:\n%s", msg)
	}
}

// TestGateBlocksPlaceholderOnlyDraft is the regression this file exists for: a
// draft whose body is still `katra new`'s prompt records nothing, so it must
// not satisfy the gate — with its own message, since "open a draft" is unhelpful
// advice to someone who already has one.
func TestGateBlocksPlaceholderOnlyDraft(t *testing.T) {
	s, repo := cliGitStore(t)
	if _, err := s.NewEntry(core.Frontmatter{Title: "Untouched Draft"}, core.DraftPlaceholderBody); err != nil {
		t.Fatal(err)
	}
	stageCode(t, repo, "main.go")

	if got := gateFor(s); got != gateEmptyDraft {
		t.Fatalf("gateFor with placeholder-only draft = %v, want gateEmptyDraft", got)
	}
	msg := gateEmptyDraft.message()
	if !strings.Contains(msg, "placeholder") || !strings.Contains(msg, "katra append") {
		t.Errorf("placeholder message should name the placeholder and point at `katra append`:\n%s", msg)
	}
	if msg == gateNoDraft.message() {
		t.Error("placeholder message is identical to the no-draft message")
	}
}

// TestGateBlocksEmptyDraft: a draft hand-edited down to nothing is the same
// situation as the placeholder.
func TestGateBlocksEmptyDraft(t *testing.T) {
	s, repo := cliGitStore(t)
	if _, err := s.NewEntry(core.Frontmatter{Title: "Blank Draft"}, "\n  \n"); err != nil {
		t.Fatal(err)
	}
	stageCode(t, repo, "main.go")

	if got := gateFor(s); got != gateEmptyDraft {
		t.Fatalf("gateFor with empty draft = %v, want gateEmptyDraft", got)
	}
}

// TestGateOpensOnceTheDraftIsWritten covers the pass case, including the one
// that used to be a false negative: a draft that still *contains* the
// placeholder string but has real prose too is written.
func TestGateOpensOnceTheDraftIsWritten(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"prose", "Ripped out the old solver because it lied about convergence."},
		{"placeholder plus prose", core.DraftPlaceholder + "\n\nThen I actually wrote something."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, repo := cliGitStore(t)
			if _, err := s.NewEntry(core.Frontmatter{Title: "Written Draft"}, c.body); err != nil {
				t.Fatal(err)
			}
			stageCode(t, repo, "main.go")

			if got := gateFor(s); got != gateOpen {
				t.Fatalf("gateFor with a written draft = %v, want gateOpen", got)
			}
		})
	}
}

// TestGateFailsOpen pins the philosophy: every uncertainty allows the commit.
// Nothing staged, and only-the-store staged, must both pass even with no draft.
func TestGateFailsOpen(t *testing.T) {
	s, repo := cliGitStore(t)

	if got := gateFor(s); got != gateOpen {
		t.Fatalf("nothing staged: gateFor = %v, want gateOpen", got)
	}

	// Only the store staged → bookkeeping commit, allowed with no draft.
	tgit(t, repo, "add", "katra")
	if got := gateFor(s); got != gateOpen {
		t.Fatalf("store-only staged: gateFor = %v, want gateOpen", got)
	}

	// A store outside any git repo: StagedFiles errors, so the gate opens.
	loose, err := core.InitStore(t.TempDir(), "No Repo")
	if err != nil {
		t.Fatal(err)
	}
	if got := gateFor(loose); got != gateOpen {
		t.Fatalf("non-repo store: gateFor = %v, want gateOpen", got)
	}
}
