package core

import (
	"os"
	"strings"
)

// Identity in katra has two lifetimes, and conflating them destroys the durable
// one. This file holds the durable half; see claim.go for the ephemeral half.
//
// The first version of this read a single token and wrote it to both `author`
// and `claimed_by`. Once the runtime exports a pane nonce into that variable,
// every author field becomes unreadable hex the moment a pane closes — and it
// still looks recorded. katra is the across-time half of the seam; a field that
// turns to garbage when a pane closes fails the guarantee katra exists for.
const (
	// AuthorEnv carries a durable, human-legible identity for whoever created
	// a node. It must still mean something years later.
	AuthorEnv = "KATRA_AUTHOR"

	// AuthorRoleEnv carries the role that identity held AT CREATION TIME.
	//
	// Captured rather than resolved, because resolving a name to a role later
	// answers what the role is *today*, not what it was at authorship. Roles
	// change, and "who ranked this, in what capacity, at the time" is the fact
	// worth keeping.
	AuthorRoleEnv = "KATRA_AUTHOR_ROLE"

	// ClaimEnv carries the EPHEMERAL half: a runtime nonce identifying the pane
	// holding a claim. It is expected to stop resolving when that pane dies —
	// that is the abandoned-work signal, not a leak, and it is why no expiry
	// logic or garbage collection exists.
	//
	// Declared here, beside the durable names, so the separation is visible in
	// one place and a test can assert the two never collapse into one variable.
	// Nothing in authorship may ever read this.
	ClaimEnv = "KATRA_CLAIM_TOKEN"
)

// Author returns the durable author identity, or "" when unset.
//
// Opaque: katra does not parse it, check a prefix, assume a length, or validate
// a shape. Validation is interpretation, and if the runtime changes its format
// katra must not notice.
func Author() string { return envToken(AuthorEnv) }

// AuthorRole returns the role held at creation, or "" when unset.
//
// Absent is a value. An unset role is absent, never inferred and never
// defaulted — a person running `katra task new` by hand holds no role, and
// guessing one flatters whoever forgot to set a variable.
func AuthorRole() string { return envToken(AuthorRoleEnv) }

// envToken reads an opaque token, trimming only whitespace so that an
// empty-but-set variable reads as absent rather than as an identity whose name
// is a space.
func envToken(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// UnattributedNodes returns the nodes of the given types that carry no author.
//
// Reported rather than inferred. A count of authored nodes without the count of
// unauthored ones measures whoever remembered to set a variable, so anything
// surfacing authorship has to surface this beside it.
func (s *Store) UnattributedNodes(types ...string) ([]Entry, error) {
	nodes, err := s.ListNodes(types...)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, n := range nodes {
		if strings.TrimSpace(n.FM.Author) == "" {
			out = append(out, n)
		}
	}
	return out, nil
}
