package core

import (
	"errors"
	"fmt"
	"strings"
)

// BlockTask marks a task blocked and adds what it waits on to blocked_by,
// keeping existing blockers and skipping repeats. Each item is a node slug or
// free text; nothing here resolves them (`katra doctor` reports a slug-shaped
// blocker that names no node). The task is validated before it is written, so
// a refused block leaves the file as it was.
func (s *Store) BlockTask(slug string, by []string) (Entry, error) {
	n, err := s.taskForStatus(slug)
	if err != nil {
		return n, err
	}
	if !hasNonBlank(by) {
		return n, fmt.Errorf("say what %s is blocked by — pass --by <slug or text>", slug)
	}
	for _, b := range by {
		b = strings.TrimSpace(b)
		if b != "" && !contains(n.FM.BlockedBy, b) {
			n.FM.BlockedBy = append(n.FM.BlockedBy, b)
		}
	}
	n.FM.Status = "blocked"
	return n, s.saveValidated(n)
}

// UnblockTask returns a blocked or waiting task to where it would stand had it
// never stopped: specced when it has a spec, else todo. Starting it again is a
// separate, deliberate step. blocked_by is cleared, since what it named no
// longer holds; the entry or commit that cleared it is the record.
func (s *Store) UnblockTask(slug string) (Entry, error) {
	n, err := s.taskForStatus(slug)
	if err != nil {
		return n, err
	}
	if n.FM.Status != "blocked" && n.FM.Status != "waiting_on_owner" {
		return n, fmt.Errorf("%s is %s, not blocked or waiting_on_owner — nothing to unblock", slug, orNone(n.FM.Status))
	}
	n.FM.BlockedBy = nil
	n.FM.Status = "todo"
	if n.FM.Spec != "" {
		n.FM.Status = "specced"
	}
	return n, s.saveValidated(n)
}

// WaitOnOwner marks a task waiting_on_owner, setting the owner first when one
// is given. A task cannot wait on nobody, so it is refused without an owner.
func (s *Store) WaitOnOwner(slug, owner string) (Entry, error) {
	n, err := s.taskForStatus(slug)
	if err != nil {
		return n, err
	}
	if o := strings.TrimSpace(owner); o != "" {
		n.FM.Owner = o
	}
	n.FM.Status = "waiting_on_owner"
	return n, s.saveValidated(n)
}

// SetStatus is the validated general setter the CLI and MCP share: it refuses
// a status the node's type does not have, and a blocked or waiting status the
// node cannot support, before writing anything.
func (s *Store) SetStatus(slug, status string) (Entry, error) {
	p, err := s.GetNode(slug)
	if err != nil {
		return Entry{}, err
	}
	n := *p
	n.FM.Status = status
	return n, s.saveValidated(n)
}

func (s *Store) taskForStatus(slug string) (Entry, error) {
	p, err := s.GetNode(slug)
	if err != nil {
		return Entry{}, err
	}
	if p.Kind() != "task" {
		return *p, errNotATask(slug, p.Kind())
	}
	return *p, nil
}

// saveValidated writes n unless the change made it invalid. It compares
// against the node as it is on disk and refuses only problems the change
// introduced: a legacy node with an odd effort can still change status, and
// cleaning it up is doctor's report, not a precondition of every write.
//
// Epic rollup is deliberately not done here. Views compute an epic's status
// from its tasks (EpicDisplayStatus), and `task start` never rolled up, so a
// status change does not rewrite the epic file as a side effect.
func (s *Store) saveValidated(n Entry) error {
	before := map[string]bool{}
	if orig, err := ParseEntry(n.Path); err == nil {
		for _, e := range ValidateNode(orig) {
			before[e.Error()] = true
		}
	}
	var introduced []error
	for _, e := range ValidateNode(n) {
		if !before[e.Error()] {
			introduced = append(introduced, e)
		}
	}
	if introduced != nil {
		return errors.Join(introduced...)
	}
	return n.Save()
}

func orNone(s string) string {
	if s == "" {
		return "(no status)"
	}
	return s
}
