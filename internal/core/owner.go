package core

import (
	"fmt"
	"regexp"
	"strings"
)

// CreationOwner returns the owner a new task or epic should be written with:
// owner trimmed, or an error when it is empty and config.yml sets
// requireOwner. The CLI and MCP both call it before creating the node, so a
// refused creation writes nothing.
func (s *Store) CreationOwner(owner string) (string, error) {
	o := strings.TrimSpace(owner)
	if o == "" && s.Config.RequireOwner {
		return "", fmt.Errorf("this katra requires an owner (requireOwner in config.yml) — pass --owner <who>")
	}
	return o, nil
}

// OwnerGaps lists open tasks and epics with no owner, in list order. Open
// means not done or cut: a finished node's owner is history, not a gap.
func OwnerGaps(nodes []Entry) []Entry {
	var out []Entry
	for _, n := range nodes {
		if (n.Kind() == "task" || n.Kind() == "epic") && strings.TrimSpace(n.FM.Owner) == "" &&
			n.FM.Status != "done" && n.FM.Status != "cut" {
			out = append(out, n)
		}
	}
	return out
}

// BlockerProblem is one finding about a blocked task's blockers.
type BlockerProblem struct {
	Slug, Message string
}

// slugShaped matches what Slugify can produce; a blocker of this shape is
// taken to name a node, and anything else is free text.
var slugShaped = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// BlockerProblems reports, for each blocked task, blockers that look like a
// node slug but name no node (a typo, or a node since renamed), and the case
// where every blocker is a node and all of them are done or cut, so the task
// is blocked on nothing and should be unblocked. Free-text blockers cannot be
// checked and are left alone.
func BlockerProblems(nodes []Entry) []BlockerProblem {
	bySlug := make(map[string]Entry, len(nodes))
	for _, n := range nodes {
		bySlug[n.Slug] = n
	}
	var out []BlockerProblem
	for _, n := range nodes {
		if n.Kind() != "task" || n.FM.Status != "blocked" || len(n.FM.BlockedBy) == 0 {
			continue
		}
		allFinished := true
		for _, b := range n.FM.BlockedBy {
			target, ok := bySlug[b]
			switch {
			case ok:
				if target.FM.Status != "done" && target.FM.Status != "cut" {
					allFinished = false
				}
			case slugShaped.MatchString(b):
				allFinished = false
				out = append(out, BlockerProblem{n.Slug, fmt.Sprintf("blocked by %s, which names no node", b)})
			default:
				allFinished = false // free text: an outside fact katra cannot see
			}
		}
		if allFinished {
			out = append(out, BlockerProblem{n.Slug, "every blocker is done or cut — `katra task unblock " + n.Slug + "`"})
		}
	}
	return out
}
