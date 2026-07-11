package core

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseWikilinks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"none", "just prose, no links", nil},
		{"simple", "see [[hub]] for more", []string{"hub"}},
		{"labeled", "see [[hub|the hub design]]", []string{"hub"}},
		{"multiple", "[[a]] and [[b]] and [[a]] again", []string{"a", "b"}},
		{"date-prefixed", "[[2026-07-09-ws-a-driving-overhaul]]", []string{"ws-a-driving-overhaul"}},
		{"dot-md-suffix", "[[foo.md]]", []string{"foo"}},
		{"spaces-trimmed", "[[  hub  |  Hub  ]]", []string{"hub"}},
		{"ignore-fenced-code", "```\n[[not-a-link]]\n```\nbut [[real]] here", []string{"real"}},
		{"ignore-inline-code", "`[[nope]]` but [[yes]]", []string{"yes"}},
		{"empty-brackets", "[[]] and [[ok]]", []string{"ok"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseWikilinks(c.body)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParseWikilinks(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

// mkNode builds an in-memory Entry with a given slug so link tests don't need
// filenames to line up.
func mkNode(slug, nodeType, body string, fm Frontmatter) Entry {
	fm.Type = nodeType
	if nodeType == "entry" {
		fm.Type = ""
	}
	return Entry{Slug: slug, FM: fm, Body: body}
}

func TestBuildBacklinks(t *testing.T) {
	nodes := []Entry{
		// entry that others point at
		mkNode("ws-a-overhaul", "entry", "Narrated the work. See [[filler-recall]].", Frontmatter{Title: "WS-A overhaul"}),
		// epic
		mkNode("filler-recall", "epic", "Group of tasks.", Frontmatter{Title: "Filler recall"}),
		// task: structured edges epic + entry, plus a body wikilink
		mkNode("fade-removal", "task", "Approach references [[filler-recall]] too.", Frontmatter{
			Title: "Fade removal", Epic: "filler-recall", Entry: "ws-a-overhaul",
		}),
		// decision chain: d2 supersedes d1, occasioned by the entry
		mkNode("signed-speed", "decision", "Context.", Frontmatter{Title: "Signed speed", Entry: "ws-a-overhaul"}),
		mkNode("velocity-vector", "decision", "Newer.", Frontmatter{Title: "Velocity vector", Supersedes: []string{"signed-speed"}}),
	}

	got := BuildBacklinks(nodes)

	want := map[string][]string{
		"filler-recall": {"fade-removal", "ws-a-overhaul"}, // body wikilinks + epic edge
		"ws-a-overhaul": {"fade-removal", "signed-speed"},  // task.entry + decision.entry
		"signed-speed":  {"velocity-vector"},               // supersedes edge
	}
	// normalize (map iteration order irrelevant; slices are already sorted)
	for k := range got {
		sort.Strings(got[k])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildBacklinks mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestBuildLinkGraphResolves(t *testing.T) {
	nodes := []Entry{
		mkNode("epic-x", "epic", "", Frontmatter{Title: "Epic X"}),
		mkNode("task-y", "task", "links to [[missing-node]] and [[epic-x]]", Frontmatter{Title: "Task Y", Epic: "epic-x"}),
	}
	g := BuildLinkGraph(nodes)

	// task-y outbound resolves epic-x once (wikilink + edge dedupe), drops missing.
	out := g.Out["task-y"]
	if len(out) != 1 || out[0].Slug != "epic-x" || out[0].Title != "Epic X" || out[0].Type != "epic" {
		t.Fatalf("task-y out = %+v, want single resolved epic-x", out)
	}
	// epic-x backlink is task-y.
	back := g.Back["epic-x"]
	if len(back) != 1 || back[0].Slug != "task-y" || back[0].Type != "task" {
		t.Fatalf("epic-x back = %+v, want single task-y", back)
	}
	// dangling target is not a graph key.
	if _, ok := g.Back["missing-node"]; ok {
		t.Errorf("missing-node should not appear as a resolved backlink key")
	}
}

func TestBuildLinkGraphFromStore(t *testing.T) {
	s, err := InitStore(t.TempDir(), "Test")
	if err != nil {
		t.Fatal(err)
	}
	epic, err := s.NewNode("epic", Frontmatter{Title: "Filler Recall"}, "The epic body.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", Frontmatter{Title: "Fade Removal", Epic: epic.Slug}, "See [["+epic.Slug+"]] and [[nope]]."); err != nil {
		t.Fatal(err)
	}

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	g := BuildLinkGraph(nodes)
	if refs := g.Back[epic.Slug]; len(refs) != 1 || refs[0].Type != "task" {
		t.Fatalf("epic backlinks = %+v, want one task", refs)
	}
}
