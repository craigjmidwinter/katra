package core

// A checkpoint is what a session should write down when it is about to lose its
// context, as opposed to what it writes when a piece of work is finished.
//
// The distinction is the whole point. An entry is narrative: it says what
// happened. A clearing session needs status: what is in flight, what is owed,
// and where it all sits. katra already held almost all of that — `task` knows
// what is owed, `reconcile` knows what is in flight — and nothing assembled the
// two at the moment they were about to be forgotten. See
// docs/design/checkpoint.md.

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Checkpoint is the derived state of open work at a moment in time. Every field
// is computed; none of it asks the session a question, because the session may
// be out of room to answer one.
type Checkpoint struct {
	Doing      []Entry            // tasks in flight
	Specced    []Entry            // tasks owed: designed, not started
	Draft      *Entry             // the active draft, if any
	InFlight   []string           // changed code paths reconcile can see
	Undeclared bool               // that code has no covering task declaration
	Memory     []MemoryObligation // unresolved memory generations
	Branch     string
	Head       string
	At         time.Time

	// Undocumented counts commits on this branch touching non-store paths since
	// the newest commit any chronicle entry is stamped with. It is the one input
	// that does not depend on anybody having maintained a record: git counts
	// commits whether or not a task was made, an entry written, or the tree left
	// dirty. See docs/design/threshold-blindness.md.
	Undocumented int
}

// HasOpenLoops reports whether there is anything worth capturing.
//
// This is the threshold. A hook that fires every time is a hook people mute, so
// a session with nothing in flight gets silence rather than an empty ceremony.
// A `specced` task alone does not count: work that is designed and unstarted is
// already durable on disk, and losing context does not lose it.
//
// Undocumented is here because the first three inputs are all derived from the
// record this feature exists to repair. InFlight in particular measures the
// dirty working tree, which means it measures *uncommitted* work rather than
// *undocumented* work — so a session that commits as it goes empties it, and
// scores zero at exactly the moment it is fullest. Counting commits since the
// last chronicle entry is the input a busy session cannot accidentally starve.
func (c Checkpoint) HasOpenLoops() bool {
	return len(c.Doing) > 0 || len(c.InFlight) > 0 || len(c.Memory) > 0 || c.Undocumented > 0
}

// BuildCheckpoint derives the current checkpoint from the store.
func (s *Store) BuildCheckpoint() Checkpoint {
	c := Checkpoint{At: time.Now()}

	if tasks, err := s.ListNodes("task"); err == nil {
		for _, t := range tasks {
			switch t.FM.Status {
			case "doing":
				c.Doing = append(c.Doing, t)
			case "specced":
				c.Specced = append(c.Specced, t)
			}
		}
	}

	if d, _ := s.ActiveDraft(); d != nil {
		c.Draft = d
	}

	// reconcile is the existing evaluator for "what has this session changed and
	// has it been declared". Composing it here rather than re-deriving keeps one
	// answer to that question instead of two that can disagree.
	if r := s.EvaluateReconcile(); r.Applicable {
		c.InFlight = append([]string(nil), r.TouchedPaths...)
		sort.Strings(c.InFlight)
		c.Undeclared = r.Task.Kind == "" || r.Task.Kind == "unknown"
		c.Memory = r.Memory
	}

	c.Branch, _ = s.branchName()
	c.Head, _ = s.HeadHash()
	c.Undocumented = s.undocumentedCommits()
	return c
}

// undocumentedCommits counts commits on HEAD that touch anything outside the
// katra store, since the newest commit a chronicle entry is stamped with.
//
// Store-only commits are excluded deliberately, and that exclusion is what makes
// the number correct rather than merely loud: a commit that only touches katra/
// *is* chronicling, so the counter falls to zero exactly when someone does the
// thing the hook is asking for.
//
// Fail-open: any git trouble returns 0, because a katra bug must never make a
// hook noisier.
func (s *Store) undocumentedCommits() int {
	prefix := s.StoreRelPrefix()
	if prefix == "" {
		return 0
	}
	args := []string{"rev-list", "--count", "--no-merges", "HEAD"}
	if since := s.lastChronicledCommit(); since != "" {
		args = append(args, "^"+since)
	}
	args = append(args, "--", ".", ":(exclude)"+prefix)

	// gitRoot, not git: s.git runs with cwd = the store directory, so a "."
	// pathspec would mean the store itself and the exclusion would invert the
	// answer — counting chronicle commits as undocumented work and real work as
	// nothing. Pathspecs here are repo-relative and must resolve from the root.
	out, err := s.gitRoot(args...)
	if err != nil {
		return 0
	}
	n := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d", &n); err != nil {
		return 0
	}
	return n
}

// lastChronicledCommit returns the newest commit hash a chronicle entry is
// stamped with and that HEAD can still reach, or "" when nothing has been
// chronicled on this history. Entries arrive newest-first, so the first
// reachable hash is the most recent one.
func (s *Store) lastChronicledCommit() string {
	entries, err := s.List()
	if err != nil {
		return ""
	}
	for _, e := range entries {
		for _, h := range e.AllHashes() {
			if h == "" {
				continue
			}
			// An unreachable hash is one from another branch or a rewritten
			// history; using it as a floor would produce a nonsense count.
			if _, err := s.git("merge-base", "--is-ancestor", h, "HEAD"); err == nil {
				return h
			}
		}
	}
	return ""
}

// branchName returns the current branch, or "" when detached or not a repo.
func (s *Store) branchName() (string, error) {
	out, err := s.git("rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

// Render writes the checkpoint as the markdown block that goes into the
// chronicle. Empty sections are omitted rather than printed empty, so the block
// stays readable at a glance — which is the only way it gets read by whoever
// picks the work up.
func (c Checkpoint) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Checkpoint — %s\n", c.At.Format("2006-01-02 15:04"))

	if len(c.Doing) > 0 {
		b.WriteString("\n**In flight**\n")
		for _, t := range c.Doing {
			fmt.Fprintf(&b, "- `%s` — %s\n", t.Slug, t.FM.Title)
		}
	}
	if len(c.Specced) > 0 {
		b.WriteString("\n**Owed** (specced, not started)\n")
		for _, t := range c.Specced {
			fmt.Fprintf(&b, "- `%s` — %s\n", t.Slug, t.FM.Title)
		}
	}
	if len(c.InFlight) > 0 {
		state := "declared"
		if c.Undeclared {
			state = "**not yet declared** — `katra reconcile --advance/--close <slug>`"
		}
		fmt.Fprintf(&b, "\n**Changed code** (%d, %s)\n", len(c.InFlight), state)
		for _, p := range c.InFlight {
			fmt.Fprintf(&b, "- `%s`\n", p)
		}
	}
	if len(c.Memory) > 0 {
		// Deduped by path: a file can have several unresolved generations, and
		// listing the same name three times reads as three problems.
		b.WriteString("\n**Memory awaiting resolution**\n")
		seen := map[string]bool{}
		for _, m := range c.Memory {
			if seen[m.Path] {
				continue
			}
			seen[m.Path] = true
			fmt.Fprintf(&b, "- `%s` (%s)\n", m.Path, m.State)
		}
	}

	if c.Undocumented > 0 {
		plural := "s"
		if c.Undocumented == 1 {
			plural = ""
		}
		fmt.Fprintf(&b, "\n**Undocumented work**\n- %d commit%s since anything was written down\n",
			c.Undocumented, plural)
	}

	b.WriteString("\n**Where**\n")
	if c.Draft != nil {
		fmt.Fprintf(&b, "- draft: `%s`\n", c.Draft.Slug)
	} else {
		b.WriteString("- draft: none\n")
	}
	if c.Branch != "" {
		head := c.Head
		if head != "" {
			head = " at `" + head + "`"
		}
		fmt.Fprintf(&b, "- branch: `%s`%s\n", c.Branch, head)
	}
	return b.String()
}

// CheckpointEntry returns today's checkpoint entry, or nil when there is none.
//
// A checkpoint is session-scoped; a draft is subject-scoped. Defaulting to the
// active draft conflated the two, and on the feature's first live use a
// session-wide checkpoint landed inside an entry opened hours earlier on an
// unrelated subject. A *stale* active draft is the normal state of exactly the
// long session this targets.
//
// Reusing today's checkpoint entry keeps one per day rather than a scatter of
// them across a long session.
func (s *Store) CheckpointEntry(at time.Time) *Entry {
	entries, err := s.List()
	if err != nil {
		return nil
	}
	want := CheckpointTitle(at)
	for i := range entries {
		if entries[i].FM.Title == want && entries[i].IsDraft() {
			return &entries[i]
		}
	}
	return nil
}

// CheckpointTitle is the title for a draft created by a checkpoint when no
// draft is active. A clearing session must not be asked to invent one.
func CheckpointTitle(at time.Time) string {
	return "Checkpoint — " + at.Format("2006-01-02")
}
