package core

import (
	"strings"
	"testing"
)

// TestHeadHashNoCommits covers Finding 7 from the ergonomics pass: `katra
// stamp` in a freshly-initialised repo with zero commits must not surface
// git's own "Needed a single revision" verbatim — nothing in that sentence
// tells a non-git-fluent reader what to do. HeadHash is the one place every
// caller (stamp, the post-commit hook, the MCP server) resolves HEAD, so the
// translation belongs here.
func TestHeadHashNoCommits(t *testing.T) {
	_, s := initRepoWithStore(t) // git init, no commit yet

	_, err := s.HeadHash()
	if err == nil {
		t.Fatal("HeadHash: want error in a repo with no commits, got nil")
	}
	if !strings.Contains(err.Error(), "no commit to stamp against — make a commit first") {
		t.Errorf("HeadHash error = %q, want a katra-level sentence naming the fix", err)
	}
	// The underlying git error must still be visible after the sentence.
	if !strings.Contains(err.Error(), "Needed a single revision") {
		t.Errorf("HeadHash error = %q, want the underlying git error preserved (%%w)", err)
	}
}

// TestIsGitNotFound covers Finding 5: a real git repo, but with the `git`
// binary missing from PATH, must be distinguishable from a directory that
// simply isn't a git repository at all.
func TestIsGitNotFound(t *testing.T) {
	root, s := initRepoWithStore(t)

	// PATH with no `git` on it — the repo is real, only the binary is gone.
	t.Setenv("PATH", t.TempDir())

	_, err := s.RepoRoot()
	if err == nil {
		t.Fatal("RepoRoot: want error when git is not on PATH, got nil")
	}
	if !IsGitNotFound(err) {
		t.Errorf("IsGitNotFound(%v) = false, want true", err)
	}
	if !strings.Contains(err.Error(), "git not found on PATH — install git") {
		t.Errorf("RepoRoot error = %q, want it to name the real problem and the fix", err)
	}
	_ = root
}

// TestIsGitNotFoundFalseForOrdinaryGitError: a genuine "not a git repository"
// error (git present, no .git anywhere above) must not be reported as
// git-not-found — the two need different fixes.
func TestIsGitNotFoundFalseForOrdinaryGitError(t *testing.T) {
	s, err := InitStore(t.TempDir(), "Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	_, err = s.RepoRoot()
	if err == nil {
		t.Fatal("RepoRoot: want error outside any git repo, got nil")
	}
	if IsGitNotFound(err) {
		t.Errorf("IsGitNotFound(%v) = true, want false — git is present, the directory just isn't a repo", err)
	}
}
