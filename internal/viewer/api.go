package viewer

import (
	"encoding/json"
	"path/filepath"
	"sort"

	"github.com/craigjmidwinter/katra/internal/core"
)

// The hub's HTML pages are built for a browser. Native clients (the macOS menu
// bar app) need the same portfolio snapshot as data, so /api/hub.json exposes
// it: every registered katra, everything in flight, and the recent log — each
// row carrying both a viewer URL and the on-disk dir, so a client can either
// open a page or shell out to the CLI in the right repo.

// APIProject is one registered katra in the snapshot.
type APIProject struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Accent      string    `json:"accent"`
	Dir         string    `json:"dir"`  // the katra/ store directory
	Root        string    `json:"root"` // the repo holding it — cwd for `katra new`
	URL         string    `json:"url"`  // /p/<id>/
	Latest      string    `json:"latest,omitempty"`
	Ago         string    `json:"ago"`
	Counts      APICounts `json:"counts"`
	Spark       []int     `json:"spark"`
}

// APICounts is a project's node tally, matching the hub project card.
type APICounts struct {
	Entries   int `json:"entries"`
	Drafts    int `json:"drafts"`
	Doing     int `json:"doing"`
	Tasks     int `json:"tasks"`
	Epics     int `json:"epics"`
	Decisions int `json:"decisions"`
}

// APIItem is one row in the in-flight or recent list. Kind is "draft", "task"
// or "entry"; Status is only meaningful for tasks.
type APIItem struct {
	Project      string `json:"project"`
	ProjectTitle string `json:"projectTitle"`
	Accent       string `json:"accent"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Slug         string `json:"slug"`
	Summary      string `json:"summary,omitempty"`
	Date         string `json:"date,omitempty"`
	Time         string `json:"time,omitempty"`
	Ago          string `json:"ago"`
	Status       string `json:"status,omitempty"`
	Spec         string `json:"spec,omitempty"`
	Hash         string `json:"hash,omitempty"`
	URL          string `json:"url"`
	Root         string `json:"root"`
}

// APISnapshot is the whole payload served at /api/hub.json.
type APISnapshot struct {
	Projects []APIProject `json:"projects"`
	InFlight []APIItem    `json:"inFlight"`
	Recent   []APIItem    `json:"recent"`
}

// recentLimit caps the recent-log list — a menu bar client shows a handful.
const recentLimit = 30

// HubSnapshot assembles the cross-project snapshot served at /api/hub.json.
// Projects sort by last activity, in-flight and recent both newest-first.
func HubSnapshot(projects []HubProject) APISnapshot {
	out := APISnapshot{
		Projects: []APIProject{},
		InFlight: []APIItem{},
		Recent:   []APIItem{},
	}

	for _, p := range projects {
		entries, _ := p.Store.ListNodes("entry")
		tasks, _ := p.Store.ListNodes("task")
		epics, _ := p.Store.ListNodes("epic")
		decisions, _ := p.Store.ListNodes("decision")

		title := p.Store.Config.Title
		if title == "" {
			title = p.ID
		}
		accent := projAccent(p)
		root := filepath.Dir(p.Store.Dir)

		// One row builder so in-flight and recent stay structurally identical.
		row := func(kind string, e core.Entry) APIItem {
			return APIItem{
				Project:      p.ID,
				ProjectTitle: title,
				Accent:       accent,
				Kind:         kind,
				Title:        e.FM.Title,
				Slug:         e.Slug,
				Summary:      e.FM.Summary,
				Date:         e.FM.Date,
				Time:         e.FM.Time,
				Ago:          agoLabel(e.FM.Date),
				Status:       e.FM.Status,
				Spec:         e.FM.Spec,
				Hash:         e.FM.Hash,
				URL:          projHref(p.ID, e.Slug, true),
				Root:         root,
			}
		}

		var drafts, doing int
		for _, e := range entries {
			if e.IsDraft() {
				drafts++
				out.InFlight = append(out.InFlight, row("draft", e))
				continue
			}
			out.Recent = append(out.Recent, row("entry", e))
		}
		for _, t := range tasks {
			if t.FM.Status == "doing" {
				doing++
				out.InFlight = append(out.InFlight, row("task", t))
			}
		}

		out.Projects = append(out.Projects, APIProject{
			ID:          p.ID,
			Title:       title,
			Description: p.Store.Config.Description,
			Accent:      accent,
			Dir:         p.Store.Dir,
			Root:        root,
			URL:         projHref(p.ID, "", true),
			Latest:      latestDate(entries),
			Ago:         agoLabel(latestDate(entries)),
			Counts: APICounts{
				Entries:   len(entries),
				Drafts:    drafts,
				Doing:     doing,
				Tasks:     len(tasks),
				Epics:     len(epics),
				Decisions: len(decisions),
			},
			Spark: sparkBars(entries),
		})
	}

	sort.SliceStable(out.Projects, func(i, j int) bool {
		return out.Projects[i].Latest > out.Projects[j].Latest
	})
	sort.SliceStable(out.InFlight, byRecency(out.InFlight))
	sort.SliceStable(out.Recent, byRecency(out.Recent))
	if len(out.Recent) > recentLimit {
		out.Recent = out.Recent[:recentLimit]
	}
	return out
}

// byRecency orders rows newest-first, using time to break same-day ties. Undated
// rows (a draft started before any date was stamped) sort to the top, since
// they're the ones the client most wants to surface.
func byRecency(items []APIItem) func(i, j int) bool {
	return func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date > items[j].Date
		}
		return items[i].Time > items[j].Time
	}
}

// hubAPIJSON marshals the snapshot for the wire.
func hubAPIJSON(projects []HubProject) ([]byte, error) {
	return json.MarshalIndent(HubSnapshot(projects), "", "  ")
}
