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
	if e.FM.Effort != "" {
		if _, err := NormalizeEffort(e.FM.Effort); err != nil {
			errs = append(errs, err)
		} else if !effortSet[e.FM.Effort] {
			errs = append(errs, fmt.Errorf("effort %q should be written %q", e.FM.Effort, strings.ToUpper(strings.TrimSpace(e.FM.Effort))))
		}
	}
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

// Efforts is the effort scale, smallest first. XS and XL extend the original
// S|M|L: both were already in use in hand-written katras.
var Efforts = []string{"XS", "S", "M", "L", "XL"}

var effortSet = func() map[string]bool {
	m := map[string]bool{}
	for _, e := range Efforts {
		m[e] = true
	}
	return m
}()

// NormalizeEffort returns the canonical spelling of an effort a writer was
// given ("m" and " M " become "M"), "" for none, or an error naming the scale.
func NormalizeEffort(s string) (string, error) {
	n := strings.ToUpper(strings.TrimSpace(s))
	if n == "" || effortSet[n] {
		return n, nil
	}
	return "", fmt.Errorf("effort %q is not one of %s", s, strings.Join(Efforts, "|"))
}
