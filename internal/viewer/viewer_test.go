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

func keys(m map[string]nodeJSON) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
