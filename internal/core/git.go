package core

import (
	"bufio"
	"crypto/sha1"
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

// gitRoot runs a git command with cwd = the repo root, so repo-relative
// pathspecs resolve correctly (the store often lives in a subdirectory, where
// git's own git commands otherwise interpret pathspecs relative to that subdir).
func (s *Store) gitRoot(args ...string) (string, error) {
	root, err := s.RepoRoot()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
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

// DirtyEntry is one working-tree change: its repo-relative path and whether git
// considers it untracked ("??").
type DirtyEntry struct {
	Path      string
	Untracked bool
}

// DirtyEntries returns the repo-root-relative paths that currently differ from
// the index/HEAD in the working tree — staged, unstaged, and untracked — each
// tagged with its untracked-ness. Reverted files (identical to HEAD again) do
// not appear, so an edit-then-revert nets nothing.
//
// Untracked-ness matters because a repo may carry a large untracked scratch tree
// (build artifacts, caches, media) that is emphatically not "work the agent did".
// Callers reconstructing a unit of work from an agent's *observed* edits keep
// untracked files (a newly written source file is untracked but is real work);
// callers falling back to a repo-wide guess must not (see reconcile).
func (s *Store) DirtyEntries() ([]DirtyEntry, error) {
	out, err := s.git("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var entries []DirtyEntry
	for _, l := range strings.Split(out, "\n") {
		if len(l) < 4 {
			continue
		}
		untracked := strings.HasPrefix(l, "??")
		// Porcelain v1: "XY <path>" or "XY <old> -> <new>" for renames.
		path := strings.TrimSpace(l[3:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		path = strings.Trim(path, "\"")
		if path != "" {
			entries = append(entries, DirtyEntry{Path: path, Untracked: untracked})
		}
	}
	return entries, nil
}

// DirtyPaths returns every dirty path (see DirtyEntries), untracked included.
// It is the coverage-verification counterpart to StagedFiles.
func (s *Store) DirtyPaths() ([]string, error) {
	entries, err := s.DirtyEntries()
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		files = append(files, e.Path)
	}
	return files, nil
}

// IsWorkProduct reports whether a dirty path is plausibly the work being logged.
// It excludes the katra store itself (bookkeeping) and the agent's own local
// config (.claude/ — hooks and skills churn on `katra setup`, and are not work).
func (s *Store) IsWorkProduct(repoRel string) bool {
	p := filepath.ToSlash(repoRel)
	if s.IsStorePath(p) {
		return false
	}
	return !strings.HasPrefix(p, ".claude/")
}

// StoreRelPrefix returns the store directory's path relative to the repo root,
// as a slash-terminated prefix (e.g. "katra/"), for filtering store-only paths
// out of a code-change set. It resolves symlinks on both ends so the comparison
// holds on macOS (/var vs /private/var). Returns "" when it cannot be computed.
func (s *Store) StoreRelPrefix() string {
	root, err := s.RepoRoot()
	if err != nil {
		return ""
	}
	if real, e := filepath.EvalSymlinks(root); e == nil {
		root = real
	}
	dir := s.Dir
	if real, e := filepath.EvalSymlinks(dir); e == nil {
		dir = real
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel) + "/"
}

// IsStorePath reports whether a repo-relative path lives inside the katra store.
func (s *Store) IsStorePath(repoRel string) bool {
	prefix := s.StoreRelPrefix()
	if prefix == "" {
		return false
	}
	return strings.HasPrefix(filepath.ToSlash(repoRel), prefix)
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

// CommitStamp stages and commits every file the publish mutated — the stamped
// entry plus any closed-task and rolled-up-epic files — as one bookkeeping commit
// (§ fix #6), so those edits don't linger dirty in the working tree. paths must
// be absolute (git runs with cwd = the katra dir, so repo-root-relative
// pathspecs would not resolve); e.Path is always included as a fallback.
func (s *Store) CommitStamp(e *Entry, paths []string) error {
	seen := map[string]bool{}
	var files []string
	for _, p := range append([]string{e.Path}, paths...) {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		files = append(files, p)
	}
	if _, err := s.git(append([]string{"add", "--"}, files...)...); err != nil {
		return err
	}
	msg := fmt.Sprintf("%s stamp %s (%s)", s.Config.commitPrefix(), e.Slug, strings.Join(e.AllHashes(), ", "))
	_, err := s.git(append([]string{"commit", "-m", msg, "--"}, files...)...)
	return err
}

func (s *Store) repoDirOr(fallback string) string {
	if r, err := s.RepoRoot(); err == nil {
		return r
	}
	return fallback
}

const hookMarker = "# >>> katra post-commit >>>"

// hookPath resolves where the post-commit hook belongs, honouring
// core.hooksPath (husky and friends) as well as stores that live in a subdir.
func (s *Store) hookPath() (string, error) {
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
	gitDir = huskyHookDir(gitDir)
	return filepath.Join(gitDir, "post-commit"), nil
}

// huskyHookDir redirects husky's generated shim directory to the tracked one
// beside it. husky points core.hooksPath at `.husky/_`, which it *regenerates*
// on every `husky install` (i.e. every `npm install`) — anything we append
// there is silently wiped. The shims in `_` exec the same-named script in the
// parent `.husky/`, which is user-owned and committed, so that's where our
// block has to live to survive. If the layout doesn't look like husky, the
// directory is returned unchanged.
func huskyHookDir(dir string) string {
	if filepath.Base(dir) != "_" {
		return dir
	}
	parent := filepath.Dir(dir)
	if filepath.Base(parent) != ".husky" {
		return dir
	}
	// The shim runner (`h` in husky v9, `husky.sh` in v8) confirms it's really
	// husky's directory and not a coincidentally-named one.
	for _, marker := range []string{"h", "husky.sh"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return parent
		}
	}
	return dir
}

// InstallHook writes (or refreshes) a post-commit hook that auto-stamps the
// active draft. It appends to any existing hook, guarded by a marker block so
// it can be cleanly removed again.
func (s *Store) InstallHook() (string, error) {
	hookPath, err := s.hookPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return "", err
	}
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
	hookPath, err := s.hookPath()
	if err != nil {
		return "", err
	}
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

// --- change-record sourcing (fingerprint inputs) ----------------------------

// gitBlobID returns the git blob object id (sha1 of "blob <len>\x00<content>")
// for the given bytes — the same id git stores in the index/tree, so a
// working-tree fingerprint and a staged-index fingerprint of identical content
// agree.
func gitBlobID(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d", len(content))
	h.Write([]byte{0})
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// headBlobIDs maps each of the given repo-relative paths to its blob id at HEAD
// (absent paths are omitted). A repo with no commits (or any git error) yields an
// empty map — callers then treat every path as newly added.
func (s *Store) headBlobIDs(paths []string) map[string]string {
	out := map[string]string{}
	if len(paths) == 0 {
		return out
	}
	args := append([]string{"ls-tree", "-z", "HEAD", "--"}, paths...)
	res, err := s.gitRoot(args...)
	if err != nil {
		return out
	}
	for _, rec := range strings.Split(res, "\x00") {
		// "<mode> <type> <object>\t<path>"
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		meta := strings.Fields(rec[:tab])
		if len(meta) < 3 || meta[1] != "blob" {
			continue
		}
		out[strings.TrimSpace(rec[tab+1:])] = meta[2]
	}
	return out
}

// indexEntry is a staged index entry's mode + blob id.
type indexEntry struct {
	Mode string
	Blob string
}

// indexEntries maps each given repo-relative path present in the index to its
// staged mode + blob id (paths staged for deletion are omitted).
func (s *Store) indexEntries(paths []string) map[string]indexEntry {
	out := map[string]indexEntry{}
	if len(paths) == 0 {
		return out
	}
	args := append([]string{"ls-files", "-s", "-z", "--"}, paths...)
	res, err := s.gitRoot(args...)
	if err != nil {
		return out
	}
	for _, rec := range strings.Split(res, "\x00") {
		// "<mode> <object> <stage>\t<path>"
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		meta := strings.Fields(rec[:tab])
		if len(meta) < 3 {
			continue
		}
		out[strings.TrimSpace(rec[tab+1:])] = indexEntry{Mode: meta[0], Blob: meta[1]}
	}
	return out
}

// workingTreeChangeRecords builds the canonical change record for each path from
// its current working-tree content (the Stop-gate view).
func (s *Store) workingTreeChangeRecords(root string, paths []string) []changeRecord {
	base := s.headBlobIDs(paths)
	recs := make([]changeRecord, 0, len(paths))
	for _, p := range paths {
		bb := base[p]
		content, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			if bb != "" {
				// Present at HEAD, gone from the working tree → deletion.
				recs = append(recs, changeRecord{Path: p, Op: "D", BaseBlob: bb})
			}
			continue
		}
		op := "M"
		if bb == "" {
			op = "A"
		}
		recs = append(recs, changeRecord{
			Path:     p,
			Op:       op,
			Mode:     workingTreeMode(filepath.Join(root, p)),
			BaseBlob: bb,
			Blob:     gitBlobID(content),
		})
	}
	return recs
}

// indexChangeRecords builds the canonical change record for each path from its
// STAGED index blob (the pre-commit view of what will actually be committed).
func (s *Store) indexChangeRecords(paths []string) []changeRecord {
	base := s.headBlobIDs(paths)
	idx := s.indexEntries(paths)
	recs := make([]changeRecord, 0, len(paths))
	for _, p := range paths {
		bb := base[p]
		if e, ok := idx[p]; ok {
			op := "M"
			if bb == "" {
				op = "A"
			}
			recs = append(recs, changeRecord{Path: p, Op: op, Mode: e.Mode, BaseBlob: bb, Blob: e.Blob})
			continue
		}
		if bb != "" {
			// In HEAD, not in the index → staged deletion.
			recs = append(recs, changeRecord{Path: p, Op: "D", BaseBlob: bb})
		}
	}
	return recs
}

// workingTreeMode returns git's file mode for a working-tree path (executable
// bit only; git tracks 100644 vs 100755 for regular files).
func workingTreeMode(abs string) string {
	fi, err := os.Stat(abs)
	if err != nil {
		return "100644"
	}
	if fi.Mode()&0o111 != 0 {
		return "100755"
	}
	return "100644"
}
