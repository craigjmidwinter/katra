package core

import (
	"regexp"
	"sort"
	"strings"
)

// wikilinkRe matches [[slug]] and [[slug|label]] occurrences. Group 1 is the
// slug (target) part, group 2 (optional) is the display label.
var wikilinkRe = regexp.MustCompile(`\[\[\s*([^\]|]+?)\s*(?:\|\s*([^\]]*?)\s*)?\]\]`)

var (
	fenceRe      = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")
)

// stripCode removes fenced and inline code spans so wikilink scanning ignores
// [[...]] that merely appears inside code (matches the rendered behavior, where
// the markdown parser never linkifies code).
func stripCode(s string) string {
	s = fenceRe.ReplaceAllString(s, "")
	s = inlineCodeRe.ReplaceAllString(s, "")
	return s
}

// normalizeRef normalizes a link target to a node slug: it drops any directory,
// a trailing ".md", and a leading YYYY-MM-DD- date prefix, so both plain slugs
// ("filler-recall") and filename-shaped edge values ("2026-07-09-foo.md")
// resolve to the same node Slug.
func normalizeRef(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return slugFromFilename(s)
}

// ParseWikilinks extracts the target slugs from [[slug]] / [[slug|label]]
// occurrences in a markdown body, ignoring links inside code. Targets are
// normalized to node slugs and returned unique, in first-seen order.
func ParseWikilinks(body string) []string {
	src := stripCode(body)
	matches := wikilinkRe.FindAllStringSubmatch(src, -1)
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		slug := normalizeRef(m[1])
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	return out
}

// outboundRefs returns the deduped, normalized target slugs a node links to,
// combining freeform [[wikilinks]] in the body with the structured frontmatter
// edges (epic, entry, supersedes, superseded-by).
func outboundRefs(n Entry) []string {
	var refs []string
	seen := map[string]bool{}
	add := func(raw string) {
		s := normalizeRef(raw)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		refs = append(refs, s)
	}
	for _, s := range ParseWikilinks(n.Body) {
		add(s)
	}
	add(n.FM.Epic)
	add(n.FM.Entry)
	for _, s := range n.FM.Supersedes {
		add(s)
	}
	for _, s := range n.FM.SupersededBy {
		add(s)
	}
	return refs
}

// BuildBacklinks builds the "what links here" index across all nodes: it maps a
// target slug to the sorted, unique list of slugs that link to it, counting both
// freeform [[wikilinks]] and the structured frontmatter edges. Targets that no
// node actually has (dangling links) still appear as keys; callers may ignore
// keys that are not real node slugs.
func BuildBacklinks(nodes []Entry) map[string][]string {
	back := map[string]map[string]bool{}
	for _, n := range nodes {
		for _, tgt := range outboundRefs(n) {
			if tgt == n.Slug {
				continue
			}
			if back[tgt] == nil {
				back[tgt] = map[string]bool{}
			}
			back[tgt][n.Slug] = true
		}
	}
	out := make(map[string][]string, len(back))
	for tgt, set := range back {
		list := make([]string, 0, len(set))
		for s := range set {
			list = append(list, s)
		}
		sort.Strings(list)
		out[tgt] = list
	}
	return out
}

// LinkRef is a resolved link to a node: enough for the viewer to render a chip
// without recomputing anything.
type LinkRef struct {
	Slug  string
	Title string
	Type  string
}

// LinkGraph holds, per node slug, its resolved outbound links and its resolved
// backlinks. Only links whose target is a real node are included.
type LinkGraph struct {
	Out  map[string][]LinkRef // slug -> nodes this node links to
	Back map[string][]LinkRef // slug -> nodes that link to this node
}

// BuildLinkGraph resolves the outbound links and backlinks for every node in a
// single pass over the set. Unresolved (dangling) links are dropped from the
// resolved arrays; the rendered body still marks them with dl-wikilink-missing.
func BuildLinkGraph(nodes []Entry) LinkGraph {
	bySlug := make(map[string]Entry, len(nodes))
	for _, n := range nodes {
		bySlug[n.Slug] = n
	}
	ref := func(slug string) (LinkRef, bool) {
		n, ok := bySlug[slug]
		if !ok {
			return LinkRef{}, false
		}
		return LinkRef{Slug: n.Slug, Title: n.FM.Title, Type: n.Kind()}, true
	}

	out := map[string][]LinkRef{}
	for _, n := range nodes {
		var refs []LinkRef
		for _, tgt := range outboundRefs(n) {
			if tgt == n.Slug {
				continue
			}
			if r, ok := ref(tgt); ok {
				refs = append(refs, r)
			}
		}
		if len(refs) > 0 {
			out[n.Slug] = refs
		}
	}

	back := map[string][]LinkRef{}
	for tgt, srcs := range BuildBacklinks(nodes) {
		if _, ok := bySlug[tgt]; !ok {
			continue // dangling target: not a real node
		}
		var refs []LinkRef
		for _, s := range srcs {
			if r, ok := ref(s); ok {
				refs = append(refs, r)
			}
		}
		if len(refs) > 0 {
			back[tgt] = refs
		}
	}
	return LinkGraph{Out: out, Back: back}
}

// KnownSlugs returns the set of node slugs, for wikilink resolution during
// rendering.
func KnownSlugs(nodes []Entry) map[string]bool {
	set := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		set[n.Slug] = true
	}
	return set
}
