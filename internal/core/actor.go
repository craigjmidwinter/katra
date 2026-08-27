package core

import (
	"os"
	"strings"
)

// ActorEnv is the environment variable carrying the actor token.
//
// An environment variable rather than anything katra looks up, because katra
// must answer "who did this" under Codex, in CI, and on a machine with none of
// this fleet's infrastructure installed. Every harness can set a variable; a
// harness that sets none produces honest absence.
const ActorEnv = "KATRA_ACTOR"

// ActorToken returns the current actor token, or "" when unset.
//
// The token is opaque. katra deliberately does not validate its shape, because
// validation is interpretation: a rule about what a token may look like is a
// rule about what tokens mean, and the moment katra holds one of those it has
// started resolving identity — which belongs to whoever reads the store, not to
// katra. Whitespace is trimmed so an empty-but-set variable reads as absent
// rather than as an author whose name is a space.
//
// Notably NOT sourced from the Claude Code hook payload's session_id. That is a
// harness UUID: absent under Codex, never written to a node, and adopting it
// would tie katra's durable record to one vendor's hook shape.
func ActorToken() string {
	return strings.TrimSpace(os.Getenv(ActorEnv))
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
