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
}

// HasOpenLoops reports whether there is anything worth capturing.
//
// This is the threshold. A hook that fires every time is a hook people mute, so
// a session with nothing in flight gets silence rather than an empty ceremony.
// Note that a `specced` task alone does not count: work that is designed and
// unstarted is already durable on disk, and losing context does not lose it.
func (c Checkpoint) HasOpenLoops() bool {
	return len(c.Doing) > 0 || len(c.InFlight) > 0 || len(c.Memory) > 0
}

// BuildCheckpoint derives the current checkpoint from the store.
func (s *Store) BuildCheckpoint() Checkpoint {
	c := Checkpoint{At: time.Now()}

	if tasks, err := s.ListNodes("task"); err == nil {
		for _, t := range tasks {
			switch t.EffectiveStatus() {
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
	return c
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

// CheckpointTitle is the title for a draft created by a checkpoint when no
// draft is active. A clearing session must not be asked to invent one.
func CheckpointTitle(at time.Time) string {
	return "Checkpoint — " + at.Format("2006-01-02")
}
