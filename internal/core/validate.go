package core

import (
	"fmt"
	"strings"
)

// ValidateNode reports what is wrong with a node's frontmatter, or nil. It is
// pure: it reads only the node, touches no file and resolves no reference, so
// the CLI and the MCP server can call it before a write and `katra doctor` can
// call it over a whole store, and all three agree.
//
// It is a check for writers, never for readers. The format promises that an
// odd value degrades rather than erroring (docs/format.md), so nothing on the
// read path calls this: a node it rejects still parses, lists and renders.
func ValidateNode(e Entry) []error {
	var errs []error
	if e.FM.Owner != "" && strings.TrimSpace(e.FM.Owner) == "" {
		errs = append(errs, fmt.Errorf("owner is blank — give a name or role, or leave the key off"))
	}
	seen := map[string]bool{}
	for _, b := range e.FM.BlockedBy {
		switch {
		case strings.TrimSpace(b) == "":
			errs = append(errs, fmt.Errorf("blocked_by has an empty item — remove it"))
		case b == e.Slug:
			errs = append(errs, fmt.Errorf("blocked_by names the node itself (%s)", b))
		case seen[b]:
			errs = append(errs, fmt.Errorf("blocked_by lists %s twice", b))
		}
		seen[b] = true
	}
	return errs
}
