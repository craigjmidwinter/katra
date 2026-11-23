package viewer

import (
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// TestHubBoardShowsSpeccedColumnAndKeepsSpeccedTasks guards the regression the
// design calls out explicitly: before the Specced column existed, hubBoardHTML's
// switch matched only doing/todo/"" and would silently drop a specced task from
// the board. The column must sit between Doing and Todo, and a specced task
// must land in it (and only it).
func TestHubBoardShowsSpeccedColumnAndKeepsSpeccedTasks(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Board Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Doing Task", Status: "doing"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Specced Task", Status: "specced", Spec: "some-node"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Todo Task", Status: "todo"}, ""); err != nil {
		t.Fatal(err)
	}

	html := hubBoardHTML([]HubProject{{ID: "board-test", Store: s}}, false)

	if !strings.Contains(html, "<h2>Specced — 1</h2>") {
		t.Errorf("missing Specced column heading:\n%s", html)
	}
	if !strings.Contains(html, "Specced Task") {
		t.Errorf("specced task dropped from the board entirely:\n%s", html)
	}

	doingIdx := strings.Index(html, "<h2>Doing")
	speccedIdx := strings.Index(html, "<h2>Specced")
	todoIdx := strings.Index(html, "<h2>Todo")
	if doingIdx < 0 || speccedIdx < 0 || todoIdx < 0 || doingIdx >= speccedIdx || speccedIdx >= todoIdx {
		t.Fatalf("Specced column not between Doing and Todo: doing=%d specced=%d todo=%d", doingIdx, speccedIdx, todoIdx)
	}
	speccedSection := html[speccedIdx:todoIdx]
	if !strings.Contains(speccedSection, "Specced Task") {
		t.Errorf("Specced Task not inside the Specced section:\n%s", speccedSection)
	}
	if strings.Contains(speccedSection, "Doing Task") || strings.Contains(speccedSection, "Todo Task") {
		t.Errorf("Specced section leaked another task:\n%s", speccedSection)
	}
}

// TestHubBoardShowsBlockedColumn: blocked and waiting_on_owner tasks land in
// one Blocked column, each card saying what it waits on, between Doing and
// Specced.
func TestHubBoardShowsBlockedColumn(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Board Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	for _, fm := range []core.Frontmatter{
		{Title: "Doing Task", Status: "doing"},
		{Title: "Stuck Task", Status: "blocked", BlockedBy: []string{"licence <decision>"}},
		{Title: "Waiting Task", Status: "waiting_on_owner", Owner: "craig"},
		{Title: "Specced Task", Status: "specced", Spec: "x"},
	} {
		if _, err := s.NewNode("task", fm, ""); err != nil {
			t.Fatal(err)
		}
	}
	html := hubBoardHTML([]HubProject{{ID: "board-test", Store: s}}, false)
	if !strings.Contains(html, "<h2>Blocked — 2</h2>") {
		t.Fatalf("missing Blocked column with both tasks:\n%s", html)
	}
	bi, si := strings.Index(html, "<h2>Blocked"), strings.Index(html, "<h2>Specced")
	if di := strings.Index(html, "<h2>Doing"); !(di < bi && bi < si) {
		t.Fatalf("Blocked not between Doing and Specced: %d %d %d", di, bi, si)
	}
	section := html[bi:si]
	for _, want := range []string{"Stuck Task", "blocked by licence &lt;decision&gt;", "Waiting Task", "waiting on craig"} {
		if !strings.Contains(section, want) {
			t.Errorf("Blocked section missing %q:\n%s", want, section)
		}
	}
}
