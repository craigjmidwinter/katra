package viewer_test

import (
	"encoding/json"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/craigjmidwinter/katra/internal/viewer"
)

type linkJSON struct {
	Slug string `json:"slug"`
	Type string `json:"type"`
}

type nodeJSON struct {
	Slug      string     `json:"slug"`
	Type      string     `json:"type"`
	Status    string     `json:"status"`
	Rollup    string     `json:"rollup"`
	Spec      string     `json:"spec"`
	Draft     bool       `json:"draft"`
	Links     []linkJSON `json:"links"`
	Backlinks []linkJSON `json:"backlinks"`
}

type dataJSON struct {
	Site struct {
		Title string `json:"title"`
	} `json:"site"`
	Entries []nodeJSON `json:"entries"`
}

func hasLink(refs []linkJSON, slug string) bool {
	for _, r := range refs {
		if r.Slug == slug {
			return true
		}
	}
	return false
}

// TestBuildDataContract locks the data.json shape the viewer depends on: every
// node carries its type + status, draft-ness is entry-only, and the wiki link
// graph (structured edges + body [[wikilinks]]) resolves with mirrored backlinks.
func TestBuildDataContract(t *testing.T) {
	dir := t.TempDir()
	s, err := core.InitStore(dir, "Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}

	// One node of every type. The article body wikilinks the task + decision;
	// the task carries a structured epic edge.
	if _, err := s.NewNode("epic", core.Frontmatter{Title: "Filler recall", Status: "planned", Horizon: "now"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Fade removal", Status: "todo", Effort: "M", Epic: "filler-recall"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("decision", core.Frontmatter{Title: "Signed speed", Status: "accepted"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("article", core.Frontmatter{Title: "Ffmpeg notes"}, "See [[fade-removal]] and [[signed-speed]]."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewEntry(core.Frontmatter{Title: "A day of work"}, "Body."); err != nil {
		t.Fatal(err)
	}

	raw, err := viewer.BuildData(s)
	if err != nil {
		t.Fatalf("BuildData: %v", err)
	}
	var data dataJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal data.json: %v", err)
	}
	if data.Site.Title != "Test" {
		t.Errorf("site.title = %q, want Test", data.Site.Title)
	}

	byslug := map[string]nodeJSON{}
	for _, n := range data.Entries {
		byslug[n.Slug] = n
	}
	for _, want := range []string{"filler-recall", "fade-removal", "signed-speed", "ffmpeg-notes", "a-day-of-work"} {
		if _, ok := byslug[want]; !ok {
			t.Fatalf("data.json missing node %q; got %v", want, keys(byslug))
		}
	}

	// Types + statuses.
	if got := byslug["fade-removal"]; got.Type != "task" || got.Status != "todo" {
		t.Errorf("task node = {type:%q status:%q}, want {task todo}", got.Type, got.Status)
	}
	if got := byslug["filler-recall"].Type; got != "epic" {
		t.Errorf("epic node type = %q, want epic", got)
	}
	if got := byslug["signed-speed"]; got.Type != "decision" || got.Status != "accepted" {
		t.Errorf("decision node = {type:%q status:%q}, want {decision accepted}", got.Type, got.Status)
	}

	// Draft-ness is entry-only (the IsDraft fix): only the entry is a draft.
	if !byslug["a-day-of-work"].Draft {
		t.Error("entry should be a draft (no commit hash)")
	}
	for _, slug := range []string{"filler-recall", "fade-removal", "signed-speed", "ffmpeg-notes"} {
		if byslug[slug].Draft {
			t.Errorf("non-entry node %q must not be a draft", slug)
		}
	}

	// Structured edge: task -> epic.
	if !hasLink(byslug["fade-removal"].Links, "filler-recall") {
		t.Errorf("task should link to its epic; links=%v", byslug["fade-removal"].Links)
	}
	// Freeform wikilinks: article -> task + decision.
	if !hasLink(byslug["ffmpeg-notes"].Links, "fade-removal") || !hasLink(byslug["ffmpeg-notes"].Links, "signed-speed") {
		t.Errorf("article should wikilink task+decision; links=%v", byslug["ffmpeg-notes"].Links)
	}
	// Backlinks are the mirror.
	if !hasLink(byslug["filler-recall"].Backlinks, "fade-removal") {
		t.Errorf("epic should be backlinked by its task; backlinks=%v", byslug["filler-recall"].Backlinks)
	}
	if !hasLink(byslug["signed-speed"].Backlinks, "ffmpeg-notes") {
		t.Errorf("decision should be backlinked by the article; backlinks=%v", byslug["signed-speed"].Backlinks)
	}
}

// TestBuildDataServesComputedEpicStatus is the anti-drift guarantee: the stored
// epic status is a cache written at stamp time, so data.json serves the status
// computed from the child tasks instead. Without this the board reads whatever
// the last stamp happened to write — this repo's own epic sat "planned" for
// three weeks while its tasks were plainly under way.
func TestBuildDataServesComputedEpicStatus(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	// Stored status is stale: the epic says planned, a child task is doing.
	if _, err := s.NewNode("epic", core.Frontmatter{Title: "Stale epic", Status: "planned"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Underway", Status: "doing", Epic: "stale-epic"}, ""); err != nil {
		t.Fatal(err)
	}
	// An epic with no child tasks has nothing to compute from: the stored
	// status is all there is, and it must survive.
	if _, err := s.NewNode("epic", core.Frontmatter{Title: "Empty epic", Status: "planned"}, ""); err != nil {
		t.Fatal(err)
	}

	raw, err := viewer.BuildData(s)
	if err != nil {
		t.Fatalf("BuildData: %v", err)
	}
	var data dataJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal data.json: %v", err)
	}
	byslug := map[string]nodeJSON{}
	for _, n := range data.Entries {
		byslug[n.Slug] = n
	}

	if got := byslug["stale-epic"].Status; got != "active" {
		t.Errorf("epic status = %q, want the computed %q (the stored field is stale)", got, "active")
	}
	if got := byslug["stale-epic"].Rollup; got != "active" {
		t.Errorf("epic rollup = %q, want %q", got, "active")
	}
	if got := byslug["empty-epic"].Status; got != "planned" {
		t.Errorf("childless epic status = %q, want the stored %q", got, "planned")
	}
	// The task's own status is untouched by any of this.
	if got := byslug["underway"].Status; got != "doing" {
		t.Errorf("task status = %q, want doing", got)
	}
}

// TestBuildDataResolvesTaskSpec covers the wikilink-identity rule for spec:
// refs — a node ref (even filename-shaped) resolves to the node's canonical
// slug so the client's plain bySlug lookup finds it, while a ref that isn't a
// known node (a repo path, or garbage) passes through unresolved for the
// client to render as plain code text.
func TestBuildDataResolvesTaskSpec(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if _, err := s.NewNode("decision", core.Frontmatter{Title: "Use Go", Status: "accepted"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Node Ref Task", Status: "specced", Spec: "use-go"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Path Ref Task", Status: "specced", Spec: "docs/design/foo.md"}, ""); err != nil {
		t.Fatal(err)
	}

	raw, err := viewer.BuildData(s)
	if err != nil {
		t.Fatalf("BuildData: %v", err)
	}
	var data dataJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal data.json: %v", err)
	}
	byslug := map[string]nodeJSON{}
	for _, n := range data.Entries {
		byslug[n.Slug] = n
	}

	if got := byslug["node-ref-task"].Spec; got != "use-go" {
		t.Errorf("node-ref spec = %q, want the resolved slug use-go", got)
	}
	if got := byslug["path-ref-task"].Spec; got != "docs/design/foo.md" {
		t.Errorf("path-ref spec = %q, want the ref unchanged (docs/design/foo.md)", got)
	}
}

func keys(m map[string]nodeJSON) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
