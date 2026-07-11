package core

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// git runs a git command inside the store's repo and returns trimmed stdout.
func (s *Store) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.Dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoRoot returns the git working-tree root containing the katra.
func (s *Store) RepoRoot() (string, error) {
	return s.git("rev-parse", "--show-toplevel")
}

// StagedFiles returns the repo-root-relative paths currently staged for commit.
func (s *Store) StagedFiles() ([]string, error) {
	out, err := s.git("diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// HeadHash returns the short hash of HEAD.
func (s *Store) HeadHash() (string, error) {
	return s.git("rev-parse", "--short", "HEAD")
}

// ShortHash normalises any revision to its short form.
func (s *Store) ShortHash(rev string) (string, error) {
	return s.git("rev-parse", "--short", rev)
}

// StatFor returns the cumulative diffstat for one commit (its diff vs. its parent).
func (s *Store) StatFor(hash string) (Stat, error) {
	// --numstat: "added<TAB>deleted<TAB>path" per file; binary files use "-".
	out, err := s.git("show", "--numstat", "--format=", hash)
	if err != nil {
		return Stat{}, err
	}
	return parseNumstat(out), nil
}

// StatForAll sums diffstats across several commits (a chapter).
func (s *Store) StatForAll(hashes []string) (Stat, error) {
	var total Stat
	for _, h := range hashes {
		st, err := s.StatFor(h)
		if err != nil {
			return total, err
		}
		total.Add(st)
	}
	return total, nil
}

func parseNumstat(out string) Stat {
	var st Stat
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		st.F++
		if n, err := strconv.Atoi(fields[0]); err == nil {
			st.A += n
		}
		if n, err := strconv.Atoi(fields[1]); err == nil {
			st.D += n
		}
	}
	return st
}

// Stamp marks an entry as logged: sets its hash(es) and computes the diffstat.
// If multiple hashes are given it becomes a chapter (hashes:[...]).
func (s *Store) Stamp(e *Entry, hashes []string) error {
	short := make([]string, 0, len(hashes))
	for _, h := range hashes {
		sh, err := s.ShortHash(h)
		if err != nil {
			return err
		}
		short = append(short, sh)
	}
	st, err := s.StatForAll(short)
	if err != nil {
		return err
	}
	if len(short) == 1 {
		e.FM.Hash = short[0]
		e.FM.Hashes = nil
	} else {
		e.FM.Hashes = short
		e.FM.Hash = ""
	}
	e.FM.Stat = &st
	return e.Save()
}

// CommitStamp stages and commits the (just stamped) entry file.
func (s *Store) CommitStamp(e *Entry) error {
	// Use the absolute path: git commands run with cwd = the katra dir,
	// so a repo-root-relative pathspec would not resolve.
	if _, err := s.git("add", "--", e.Path); err != nil {
		return err
	}
	msg := fmt.Sprintf("%s stamp %s (%s)", s.Config.commitPrefix(), e.Slug, strings.Join(e.AllHashes(), ", "))
	_, err := s.git("commit", "-m", msg, "--", e.Path)
	return err
}

func (s *Store) repoDirOr(fallback string) string {
	if r, err := s.RepoRoot(); err == nil {
		return r
	}
	return fallback
}

const hookMarker = "# >>> katra post-commit >>>"

// InstallHook writes (or refreshes) a post-commit hook that auto-stamps the
// active draft. It appends to any existing hook, guarded by a marker block so
// it can be cleanly removed again.
func (s *Store) InstallHook() (string, error) {
	root, err := s.RepoRoot()
	if err != nil {
		return "", err
	}
	gitDir, err := s.git("rev-parse", "--git-path", "hooks")
	if err != nil {
		gitDir = filepath.Join(root, ".git", "hooks")
	}
	if !filepath.IsAbs(gitDir) {
		// `rev-parse --git-path` is relative to git's working dir (s.Dir), not
		// the repo root — joining against root leaks the hook up a level for
		// stores in a subdir and for custom core.hooksPath (e.g. husky).
		gitDir = filepath.Join(s.Dir, gitDir)
	}
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return "", err
	}
	hookPath := filepath.Join(gitDir, "post-commit")
	block := hookBlock()

	existing, _ := os.ReadFile(hookPath)
	content := stripHookBlock(string(existing))
	if content == "" {
		content = "#!/bin/sh\n"
	} else if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += block
	if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
		return "", err
	}
	return hookPath, nil
}

// UninstallHook removes the katra block from the post-commit hook.
func (s *Store) UninstallHook() (string, error) {
	root, err := s.RepoRoot()
	if err != nil {
		return "", err
	}
	hookPath := filepath.Join(root, ".git", "hooks", "post-commit")
	existing, err := os.ReadFile(hookPath)
	if err != nil {
		return hookPath, nil // nothing to do
	}
	stripped := stripHookBlock(string(existing))
	if strings.TrimSpace(stripped) == "#!/bin/sh" || strings.TrimSpace(stripped) == "" {
		_ = os.Remove(hookPath)
		return hookPath, nil
	}
	return hookPath, os.WriteFile(hookPath, []byte(stripped), 0o755)
}

func hookBlock() string {
	return hookMarker + `
# Auto-stamps the active katra draft with the commit you just made.
# Skips its own stamp commits and commits that only touch the katra.
if command -v katra >/dev/null 2>&1; then
  katra hook run --quiet || true
fi
# <<< katra post-commit <<<
`
}

func stripHookBlock(s string) string {
	start := strings.Index(s, hookMarker)
	if start < 0 {
		return s
	}
	end := strings.Index(s, "# <<< katra post-commit <<<")
	if end < 0 {
		return strings.TrimRight(s[:start], "\n") + "\n"
	}
	end += len("# <<< katra post-commit <<<")
	rest := s[end:]
	return strings.TrimRight(s[:start], "\n") + "\n" + strings.TrimLeft(rest, "\n")
}

// HookShouldSkip reports whether the post-commit hook should do nothing for the
// current HEAD: either it's a katra stamp commit, or it only touched the
// katra directory (avoids stamping our own bookkeeping commits in a loop).
func (s *Store) HookShouldSkip() (bool, error) {
	msg, err := s.git("log", "-1", "--format=%s")
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(strings.TrimSpace(msg), s.Config.commitPrefix()) {
		return true, nil
	}
	files, err := s.git("show", "--name-only", "--format=", "HEAD")
	if err != nil {
		return false, err
	}
	root, err := s.RepoRoot()
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(root, s.Dir)
	if err != nil {
		return false, nil
	}
	onlyDevlog := true
	any := false
	for _, f := range strings.Split(files, "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		any = true
		if !strings.HasPrefix(f, rel+"/") {
			onlyDevlog = false
			break
		}
	}
	return any && onlyDevlog, nil
}
