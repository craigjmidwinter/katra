package viewer

import (
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// TestHubRoadmapShowsComputedEpicStatus: the cross-project roadmap is a view of
// the task files, so it shows the status computed from them — not the stored
// epic field, which is only refreshed at stamp time and drifts in between.
func TestHubRoadmapShowsComputedEpicStatus(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Roadmap Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if _, err := s.NewNode("epic", core.Frontmatter{Title: "Stale epic", Status: "planned", Horizon: "now"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Underway", Status: "doing", Epic: "stale-epic"}, ""); err != nil {
		t.Fatal(err)
	}

	html := hubRoadmapHTML([]HubProject{{ID: "roadmap-test", Store: s}}, false)
	card := cardFor(t, html, "Stale epic")
	if !strings.Contains(card, ">active<") {
		t.Errorf("roadmap card should show the computed status:\n%s", card)
	}
	if strings.Contains(card, ">planned<") {
		t.Errorf("roadmap card shows the stale stored status:\n%s", card)
	}
}

// cardFor returns the slice of html around the named epic's card.
func cardFor(t *testing.T, html, title string) string {
	t.Helper()
	i := strings.Index(html, title)
	if i < 0 {
		t.Fatalf("epic %q missing from the roadmap:\n%s", title, html)
	}
	end := i + 200
	if end > len(html) {
		end = len(html)
	}
	return html[i:end]
}
