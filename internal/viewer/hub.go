package viewer

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/craigjmidwinter/katra/internal/core"
)

// projectsJSON is the sibling-project list the per-project viewer's switcher
// reads from <viewer>/projects.json. Hrefs are absolute when served by the hub
// and relative for a static build (where the reader sits in p/<id>/). Absent in
// a standalone single-project `katra serve`/`build` — the viewer then simply
// renders no switcher.
func projectsJSON(projects []HubProject, web bool) []byte {
	type projEntry struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Accent string `json:"accent"`
		Href   string `json:"href"`
	}
	out := struct {
		Hub      string      `json:"hub"`
		Projects []projEntry `json:"projects"`
	}{Hub: "/"}
	if !web {
		out.Hub = "../../index.html"
	}
	for _, p := range projects {
		title := p.Store.Config.Title
		if title == "" {
			title = p.ID
		}
		href := "/p/" + p.ID + "/"
		if !web {
			href = "../" + p.ID + "/index.html"
		}
		out.Projects = append(out.Projects, projEntry{
			ID: p.ID, Title: title, Accent: projAccent(p), Href: href,
		})
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return b
}

// HubProject is one katra served by the hub under /p/<ID>/.
type HubProject struct {
	ID    string
	Store *core.Store
}

// hubSet holds the hub's current project set. The hub normally runs as a
// login-time daemon, so the registry it was started with goes stale the moment
// a new katra is created — `katra init` in a fresh repo would not show up on
// the board until the machine was rebooted. The set is therefore re-read on an
// interval rather than snapshotted at boot, and every handler reads through
// snapshot() instead of closing over a fixed slice.
type hubSet struct {
	mu       sync.RWMutex
	projects []HubProject
	byID     map[string]*core.Store

	load     func() ([]HubProject, error)
	hub      *reloadHub
	watching map[string]bool
}

// snapshot returns the current project set. The slice is not copied — handlers
// only read it, and refresh always installs a freshly built one.
func (h *hubSet) snapshot() ([]HubProject, map[string]*core.Store) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.projects, h.byID
}

// refresh re-reads the registry and swaps in the new set, starting a watcher
// for any store we haven't seen before. A load error leaves the previous set in
// place — a transient unreadable registry must not empty the board. It reports
// whether the set of project ids changed.
func (h *hubSet) refresh() bool {
	projects, err := h.load()
	if err != nil {
		return false
	}
	byID := make(map[string]*core.Store, len(projects))
	for _, p := range projects {
		byID[p.ID] = p.Store
	}

	h.mu.Lock()
	changed := len(projects) != len(h.projects)
	if !changed {
		for _, p := range projects {
			if _, ok := h.byID[p.ID]; !ok {
				changed = true
				break
			}
		}
	}
	h.projects, h.byID = projects, byID
	var fresh []string
	for _, p := range projects {
		if !h.watching[p.Store.Dir] {
			h.watching[p.Store.Dir] = true
			fresh = append(fresh, p.Store.Dir)
		}
	}
	h.mu.Unlock()

	for _, dir := range fresh {
		go watch(dir, h.hub)
	}
	return changed
}

// hubRefreshInterval is how often the hub re-reads the registry. Creating a
// katra is a human-scale event, so this only has to beat "notice it's missing
// and restart the daemon".
const hubRefreshInterval = 10 * time.Second

// ServeHub runs the multi-tenant hub: one server, an index of every registered
// katra at /, and each project's viewer at /p/<id>/. data.json is generated per
// request; any change in any store reloads open tabs (one global livereload).
// load supplies the project set and is re-consulted periodically, so katras
// created after the daemon started appear without a restart.
func ServeHub(load func() ([]HubProject, error), port int) error {
	set := &hubSet{
		load:     load,
		hub:      &reloadHub{clients: map[chan string]struct{}{}},
		watching: map[string]bool{},
	}
	hub := set.hub
	set.refresh()
	projects, _ := set.snapshot()

	// Pick up katras registered after the daemon started, and nudge open tabs
	// when the roster itself changes (not just a store's contents).
	go func() {
		for range time.Tick(hubRefreshInterval) {
			if set.refresh() {
				hub.broadcast("reload")
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/__livereload", sseHandler(hub))

	mux.HandleFunc("/p/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/p/")
		id, sub, hasSlash := strings.Cut(rest, "/")
		projects, byID := set.snapshot()
		store, ok := byID[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if !hasSlash {
			http.Redirect(w, r, "/p/"+id+"/", http.StatusFound)
			return
		}
		switch {
		case sub == "" || sub == "index.html":
			b, err := Asset("index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			page := strings.Replace(string(b), "</body>", reloadClient+"\n</body>", 1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write([]byte(page))
		case sub == "data.json":
			b, err := BuildData(store)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(b)
		case sub == "projects.json":
			// Feeds the viewer's project switcher — every sibling katra.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(projectsJSON(projects, true))
		case strings.HasPrefix(sub, "media/"):
			relPath := strings.TrimPrefix(sub, "media/")
			p := filepath.Join(store.MediaDir(), filepath.Clean("/"+relPath))
			if !strings.HasPrefix(p, store.MediaDir()) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, p)
		default: // app.js, styles.css
			b, err := Asset(sub)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", contentType(sub))
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(b)
		}
	})

	// Portfolio snapshot as data — what native clients (the menu bar app) read
	// instead of scraping the HTML pages.
	mux.HandleFunc("/api/hub.json", func(w http.ResponseWriter, r *http.Request) {
		projects, _ := set.snapshot()
		b, err := hubAPIJSON(projects)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
	})

	page := func(html string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write([]byte(html))
		}
	}
	mux.HandleFunc("/board", func(w http.ResponseWriter, r *http.Request) {
		projects, _ := set.snapshot()
		page(hubBoardHTML(projects, true))(w, r)
	})
	mux.HandleFunc("/roadmap", func(w http.ResponseWriter, r *http.Request) {
		projects, _ := set.snapshot()
		page(hubRoadmapHTML(projects, true))(w, r)
	})
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		projects, _ := set.snapshot()
		page(hubLogHTML(projects, true))(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		projects, _ := set.snapshot()
		page(hubIndexHTML(projects, true))(w, r)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("\n  Katra hub — %d project(s)\n", len(projects))
	fmt.Printf("  ───────────────────────────────\n")
	fmt.Printf("  local:   http://localhost:%d/\n", port)
	for _, ip := range lanAddrs() {
		fmt.Printf("  network: http://%s:%d/\n", ip, port)
	}
	fmt.Printf("  serving every registered katra. Ctrl-C to stop.\n\n")
	return http.ListenAndServe(addr, mux)
}

// sseHandler returns the livereload SSE handler for a reload hub.
func sseHandler(hub *reloadHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ch := hub.add()
		defer hub.remove(ch)
		fmt.Fprint(w, "retry: 1000\n\n")
		fl.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case msg := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", msg)
				fl.Flush()
			}
		}
	}
}

// navHref returns the link for a nav key, absolute when served (web) or a
// relative .html file for a static build.
func navHref(key string, web bool) string {
	if web {
		switch key {
		case "board":
			return "/board"
		case "roadmap":
			return "/roadmap"
		case "log":
			return "/log"
		default:
			return "/"
		}
	}
	switch key {
	case "board":
		return "board.html"
	case "roadmap":
		return "roadmap.html"
	case "log":
		return "log.html"
	default:
		return "index.html"
	}
}

// projHref links into a project's viewer: absolute /p/<id>/ when served, or a
// relative p/<id>/index.html for a static build. anchor may be "".
func projHref(id, anchor string, web bool) string {
	base := "p/" + id + "/index.html"
	if web {
		base = "/p/" + id + "/"
	}
	if anchor != "" {
		return base + "#" + anchor
	}
	return base
}

// hubCSS is the Field Notebook × Workbench styling shared by every hub page.
const hubCSS = `
:root{
  --paper:#f4efe4;--paper-side:#efe7d6;--paper-card:#fbf8f1;--paper-sunk:#f1ebdd;
  --ink:#2c2823;--ink-2:#4a4239;--ink-dim:#6f6656;--ink-soft:#8a7f6d;--ink-faint:#a99a80;
  --line:#e5dcc9;--line-2:#e0d5bf;--line-3:#d8cbb4;
  --accent:#b5502f;--amber:#c08a3a;--amber-ink:#a9741f;--green:#4a8a5c;--green-dot:#5aa877;
  --serif:"Newsreader",Georgia,serif;--sans:"IBM Plex Sans",system-ui,-apple-system,sans-serif;
  --mono:"IBM Plex Mono",ui-monospace,Menlo,monospace;
}
*{box-sizing:border-box}
body{margin:0;background:var(--paper);color:var(--ink);font-family:var(--sans);line-height:1.5;-webkit-font-smoothing:antialiased}
a{color:inherit;text-decoration:none}
h1,h2,h3{font-family:var(--serif);font-weight:600;letter-spacing:-.01em}
.hub-top{display:flex;align-items:center;gap:16px;height:60px;padding:0 28px;background:var(--paper-card);border-bottom:1px solid var(--line);flex-wrap:wrap}
.hub-logo{display:flex;align-items:center;gap:10px}
.hub-mark{width:26px;height:26px;border-radius:50%;background:radial-gradient(circle at 35% 30%,#e0a06f,#8a3d24);display:block}
.hub-logo .nm{font-family:var(--serif);font-size:20px;font-weight:600}
.hub-logo .sub{font-family:var(--mono);font-size:10px;color:var(--ink-faint);border-left:1px solid var(--line-2);padding-left:12px}
.hub-search{margin-left:14px;display:flex;align-items:center;gap:9px;background:var(--paper-sunk);border:1px solid var(--line-2);border-radius:9px;padding:8px 14px;width:300px;max-width:34vw;color:var(--ink-faint);font-size:12.5px}
.hub-search .kbd{margin-left:auto;font-family:var(--mono);font-size:10px;border:1px solid var(--line-2);border-radius:4px;padding:0 5px}
.hub-stats{margin-left:auto;font-family:var(--mono);font-size:11px;color:var(--ink-soft);display:flex;gap:14px;flex-wrap:wrap}
.hub-stats b{color:var(--ink)}.hub-stats .amber b{color:var(--amber-ink)}.hub-stats .up{color:var(--green)}
.hub-nav{display:flex;gap:4px;padding:10px 28px 0;background:var(--paper);border-bottom:1px solid var(--line)}
.hub-nav a{font-family:var(--mono);font-size:12px;color:var(--ink-soft);padding:8px 13px;border-radius:8px 8px 0 0;border-bottom:2px solid transparent}
.hub-nav a:hover{color:var(--ink)}
.hub-nav a.on{color:var(--accent);font-weight:600;border-bottom-color:var(--accent)}
.hub-main{padding:26px 28px 60px;max-width:1240px}
.sec-head{display:flex;align-items:center;gap:10px;margin:0 0 14px}
.sec-head .dot{width:8px;height:8px;border-radius:50%}
.sec-head h2{font-size:20px;margin:0}
.sec-head .note{font-family:var(--mono);font-size:11px;color:var(--ink-faint)}
.sec-gap{margin-top:34px}
.flight-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}
.flight-card{background:var(--paper-card);border:1px solid var(--line);border-left:3px solid var(--accent);border-radius:11px;padding:12px 14px;display:block}
.flight-card:hover{border-color:var(--line-3)}
.flight-card.draft{background:#faf4e6;border:1px dashed #d8b98a}
.flight-card .pid{font-family:var(--mono);font-size:9.5px;margin-bottom:6px}
.flight-card .ft{font-family:var(--serif);font-size:15px;font-weight:600;line-height:1.25}
.flight-card .fm{font-family:var(--mono);font-size:9.5px;color:var(--ink-faint);margin-top:7px}
.flight-card .fm.draft{color:var(--amber-ink)}
.proj-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}
.proj-card{background:var(--paper-card);border:1px solid var(--line);border-radius:13px;overflow:hidden;display:block}
.proj-card:hover{border-color:var(--line-3)}
.proj-card .bar{height:4px}
.proj-card .pad{padding:15px 16px}
.proj-card .row{display:flex;justify-content:space-between;align-items:baseline}
.proj-card h3{margin:0;font-size:18px}
.proj-card .ago{font-family:var(--mono);font-size:10px;color:var(--ink-faint)}
.proj-card .desc{font-size:12.5px;color:var(--ink-soft);margin:5px 0 12px}
.spark{display:flex;align-items:flex-end;gap:3px;height:24px;margin-bottom:11px}
.spark span{flex:1;background:#ece3d1;border-radius:1px;min-height:2px}
.proj-card .counts{display:flex;gap:12px;font-family:var(--mono);font-size:10.5px;color:var(--ink-faint)}
.proj-card .counts b{color:var(--ink)}.proj-card .counts .amber b{color:var(--amber-ink)}
.hub-cols{display:grid;grid-template-columns:1.4fr 1fr;gap:18px;margin-top:34px}
.hub-cols h2{font-size:20px;margin:0 0 14px}
.road-row{display:flex;align-items:center;gap:12px;background:var(--paper-card);border:1px solid var(--line);border-radius:10px;padding:11px 14px;margin-bottom:9px}
.road-row .hz{font-family:var(--mono);font-size:9.5px;letter-spacing:.12em;text-transform:uppercase;width:44px}
.road-row .hz.now{color:var(--accent)}.road-row .hz.next{color:var(--amber-ink)}.road-row .hz.later{color:var(--ink-soft)}
.road-row .dot{width:7px;height:7px;border-radius:50%}
.road-row .rt{font-family:var(--serif);font-size:15px;font-weight:600;flex:1;min-width:0}
.road-row .rp{font-family:var(--mono);font-size:10px;color:var(--ink-faint)}
.dec-card{display:block;background:var(--paper-card);border:1px solid var(--line);border-left:2px solid var(--green-dot);border-radius:10px;padding:11px 13px;margin-bottom:9px}
.dec-card.muted{border-left-color:var(--line-3)}
.dec-card .dt{font-family:var(--serif);font-size:14px;font-weight:600;line-height:1.3}
.dec-card.muted .dt{color:var(--ink-soft)}
.dec-card .dm{font-family:var(--mono);font-size:9.5px;color:var(--ink-faint);margin-top:5px}
/* aggregate board/roadmap/log sub-pages reuse these simple card classes */
.hub-main h2{font-size:.8rem;text-transform:uppercase;letter-spacing:.08em;color:var(--ink-faint);font-family:var(--mono);margin:26px 0 10px}
.hub-main h2:first-child{margin-top:0}
a.card{display:block;background:var(--paper-card);border:1px solid var(--line);border-radius:10px;padding:12px 15px;margin:9px 0}
a.card:hover{border-color:var(--line-3)}
.card .t{font-family:var(--serif);font-weight:600;font-size:16px}
.card .m{color:var(--ink-soft);font-family:var(--mono);font-size:11px;margin-top:4px}
.card .b{display:inline-block;margin-right:.9rem}
.card .proj{color:var(--ink-faint);font-family:var(--mono);font-size:10.5px;margin-top:4px}
.eff{font-family:var(--mono);font-size:9.5px;color:var(--ink-soft);border:1px solid var(--line-2);border-radius:4px;padding:1px 5px;margin-left:.4rem;text-transform:uppercase}
.empty{color:var(--ink-faint);font-family:var(--mono);font-size:12px}
@media(max-width:980px){.flight-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.proj-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.hub-cols{grid-template-columns:1fr}}
@media(max-width:620px){.flight-grid,.proj-grid{grid-template-columns:1fr}.hub-search{display:none}}
`

// hubShell wraps inner HTML in the common hub chrome: a top bar (logo + search +
// portfolio stats) and a tab nav. active is the current section for highlighting.
func hubShell(active, inner string, web bool, projects []HubProject) string {
	var inFlight int
	for _, p := range projects {
		entries, _ := p.Store.ListNodes("entry")
		tasks, _ := p.Store.ListNodes("task")
		for _, e := range entries {
			if e.IsDraft() {
				inFlight++
			}
		}
		for _, t := range tasks {
			if t.FM.Status == "doing" {
				inFlight++
			}
		}
	}
	daemon := ""
	if web {
		daemon = `<span class="up">● daemon up</span>`
	}
	header := `<div class="hub-top">` +
		`<div class="hub-logo"><span class="hub-mark"></span><span class="nm">Katra</span>` +
		`<span class="sub">all projects</span></div>` +
		`<div class="hub-search">⌕ Search every project…<span class="kbd">⌘K</span></div>` +
		fmt.Sprintf(`<div class="hub-stats"><span><b>%d</b> projects</span><span class="amber"><b>%d</b> in flight</span>%s</div>`,
			len(projects), inFlight, daemon) +
		`</div>`

	nav := `<div class="hub-nav">`
	for _, l := range []struct{ label, key string }{
		{"Projects", "projects"}, {"Log", "log"}, {"Board", "board"}, {"Roadmap", "roadmap"},
	} {
		cls := ""
		if l.key == active {
			cls = " class=\"on\""
		}
		nav += fmt.Sprintf(`<a%s href="%s">%s</a>`, cls, navHref(l.key, web), l.label)
	}
	nav += `</div>`

	return `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<link rel="preconnect" href="https://fonts.googleapis.com">` +
		`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>` +
		`<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&family=Newsreader:opsz,wght@6..72,400;6..72,500;6..72,600&display=swap" rel="stylesheet">` +
		`<title>Katra hub</title><style>` + hubCSS + `</style></head><body>` +
		header + nav + `<div class="hub-main">` + inner + `</div></body></html>`
}

// projAccent returns a project's configured accent, or a warm default.
func projAccent(p HubProject) string {
	if a := strings.TrimSpace(p.Store.Config.Accent); a != "" {
		return a
	}
	return "#c86a3f"
}

// latestDate returns a project's most recent entry date ("" if none).
func latestDate(entries []core.Entry) string {
	best := ""
	for _, e := range entries {
		if e.FM.Date > best {
			best = e.FM.Date
		}
	}
	return best
}

// agoLabel turns a YYYY-MM-DD into a coarse "2h/3d/…" relative to now.
func agoLabel(date string) string {
	if date == "" {
		return "—"
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "1d ago"
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return date
	}
}

// sparkBars buckets entry dates into 7 slots ending at the latest date, so a
// project card shows its recent cadence. Empty when there are no dated entries.
func sparkBars(entries []core.Entry) []int {
	bars := make([]int, 7)
	var maxT time.Time
	var ts []time.Time
	for _, e := range entries {
		t, err := time.Parse("2006-01-02", e.FM.Date)
		if err != nil {
			continue
		}
		ts = append(ts, t)
		if t.After(maxT) {
			maxT = t
		}
	}
	for _, t := range ts {
		days := int(maxT.Sub(t).Hours() / 24)
		if days >= 0 && days < 7 {
			bars[6-days]++
		}
	}
	return bars
}

// hubIndexHTML renders the portfolio pulse: everything in flight across repos, a
// project index, and a rolled-up roadmap + recent decisions.
func hubIndexHTML(projects []HubProject, web bool) string {
	if len(projects) == 0 {
		return hubShell("projects",
			`<p class="empty">No katras registered yet. Run <code>katra init</code> in a repo.</p>`,
			web, projects)
	}

	type proj struct {
		p                       HubProject
		entries                 []core.Entry
		tasks, epics, decisions []core.Entry
		drafts, doing           int
		latest                  string
	}
	var ps []proj
	for _, p := range projects {
		entries, _ := p.Store.ListNodes("entry")
		tasks, _ := p.Store.ListNodes("task")
		epics, _ := p.Store.ListNodes("epic")
		decisions, _ := p.Store.ListNodes("decision")
		var drafts, doing int
		for _, e := range entries {
			if e.IsDraft() {
				drafts++
			}
		}
		for _, t := range tasks {
			if t.FM.Status == "doing" {
				doing++
			}
		}
		ps = append(ps, proj{p, entries, tasks, epics, decisions, drafts, doing, latestDate(entries)})
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].latest > ps[j].latest })

	var b strings.Builder

	// ── In flight everywhere: doing tasks + open drafts across repos ──────────
	var flight strings.Builder
	nFlight := 0
	for _, x := range ps {
		accent := projAccent(x.p)
		for _, t := range x.tasks {
			if t.FM.Status != "doing" {
				continue
			}
			fmt.Fprintf(&flight, `<a class="flight-card" href="%s" style="border-left-color:%s">`+
				`<div class="pid" style="color:%s">%s</div><div class="ft">%s</div><div class="fm">task · doing</div></a>`,
				projHref(x.p.ID, t.Slug, web), attr(accent), attr(accent),
				html.EscapeString(x.p.ID), html.EscapeString(t.FM.Title))
			nFlight++
		}
		for _, e := range x.entries {
			if !e.IsDraft() {
				continue
			}
			fmt.Fprintf(&flight, `<a class="flight-card draft" href="%s">`+
				`<div class="pid" style="color:%s">%s</div><div class="ft">%s</div><div class="fm draft">✎ draft entry</div></a>`,
				projHref(x.p.ID, e.Slug, web), attr(accent),
				html.EscapeString(x.p.ID), html.EscapeString(e.FM.Title))
			nFlight++
		}
	}
	b.WriteString(`<div class="sec-head"><span class="dot" style="background:var(--amber)"></span>` +
		`<h2>In flight everywhere</h2><span class="note">every doing task &amp; open draft, all repos</span></div>`)
	if nFlight == 0 {
		b.WriteString(`<p class="empty">Nothing in flight — every repo is at a clean stopping point.</p>`)
	} else {
		b.WriteString(`<div class="flight-grid">` + flight.String() + `</div>`)
	}

	// ── Projects grid ────────────────────────────────────────────────────────
	b.WriteString(`<div class="sec-head sec-gap"><h2>Projects</h2>` +
		`<span class="note">sorted by last activity</span></div><div class="proj-grid">`)
	for _, x := range ps {
		accent := projAccent(x.p)
		title := x.p.Store.Config.Title
		if title == "" {
			title = x.p.ID
		}
		fmt.Fprintf(&b, `<a class="proj-card" href="%s"><div class="bar" style="background:%s"></div><div class="pad">`,
			projHref(x.p.ID, "", web), attr(accent))
		fmt.Fprintf(&b, `<div class="row"><h3>%s</h3><span class="ago">%s</span></div>`,
			html.EscapeString(title), html.EscapeString(agoLabel(x.latest)))
		if d := strings.TrimSpace(x.p.Store.Config.Description); d != "" {
			fmt.Fprintf(&b, `<div class="desc">%s</div>`, html.EscapeString(d))
		} else {
			b.WriteString(`<div class="desc">&nbsp;</div>`)
		}
		b.WriteString(sparkHTML(sparkBars(x.entries), accent))
		fmt.Fprintf(&b, `<div class="counts"><span class="amber"><b>%d</b> doing</span>`+
			`<span><b>%d</b> draft%s</span><span><b>%d</b> entries</span></div></div></a>`,
			x.doing, x.drafts, plural(x.drafts), len(x.entries))
	}
	b.WriteString(`</div>`)

	// ── Portfolio roadmap + Recent decisions ─────────────────────────────────
	b.WriteString(`<div class="hub-cols"><div><h2>Portfolio roadmap</h2>`)
	horizons := []struct{ key, label string }{{"now", "now"}, {"next", "next"}, {"later", "later"}}
	roadRows := 0
	for _, h := range horizons {
		for _, x := range ps {
			for _, e := range x.epics {
				if e.FM.Horizon != h.key {
					continue
				}
				fmt.Fprintf(&b, `<a class="road-row" href="%s"><span class="hz %s">%s</span>`+
					`<span class="dot" style="background:%s"></span><span class="rt">%s</span>`+
					`<span class="rp">%s</span></a>`,
					projHref(x.p.ID, e.Slug, web), h.key, h.label, attr(projAccent(x.p)),
					html.EscapeString(e.FM.Title), html.EscapeString(x.p.ID))
				roadRows++
			}
		}
	}
	if roadRows == 0 {
		b.WriteString(`<p class="empty">No scheduled epics yet.</p>`)
	}
	b.WriteString(`</div><div><h2>Recent decisions</h2>`)

	type dec struct {
		pid, title, status, date string
		slug                     string
	}
	var decs []dec
	for _, x := range ps {
		for _, d := range x.decisions {
			decs = append(decs, dec{x.p.ID, d.FM.Title, d.FM.Status, d.FM.Date, d.Slug})
		}
	}
	sort.SliceStable(decs, func(i, j int) bool { return decs[i].date > decs[j].date })
	if len(decs) == 0 {
		b.WriteString(`<p class="empty">No decisions recorded yet.</p>`)
	}
	for i, d := range decs {
		if i >= 6 {
			break
		}
		muted := ""
		if d.status == "superseded" || d.status == "deprecated" {
			muted = " muted"
		}
		status := d.status
		if status == "" {
			status = "accepted"
		}
		fmt.Fprintf(&b, `<a class="dec-card%s" href="%s"><div class="dt">%s</div>`+
			`<div class="dm">%s · %s</div></a>`,
			muted, projHref(d.pid, d.slug, web), html.EscapeString(d.title),
			html.EscapeString(d.pid), html.EscapeString(status))
	}
	b.WriteString(`</div></div>`)

	return hubShell("projects", b.String(), web, projects)
}

// sparkHTML renders a normalized 7-bar sparkline; bars with activity use accent.
func sparkHTML(bars []int, accent string) string {
	max := 1
	for _, v := range bars {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	b.WriteString(`<div class="spark">`)
	for _, v := range bars {
		h := 15 + (v*85)/max
		color := "#ece3d1"
		if v > 0 {
			color = attr(accent)
		}
		fmt.Fprintf(&b, `<span style="height:%d%%;background:%s"></span>`, h, color)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// hubTaskCard renders one task as a card linking into its project's viewer.
func hubTaskCard(pid, title, effort, slug string, web bool) string {
	eff := ""
	if effort != "" {
		eff = fmt.Sprintf(`<span class="eff">%s</span>`, html.EscapeString(effort))
	}
	return fmt.Sprintf(`<a class="card" href="%s"><div class="t">%s%s</div><div class="proj">%s</div></a>`,
		projHref(pid, slug, web), html.EscapeString(title), eff, html.EscapeString(pid))
}

// hubBoardHTML renders one board of Doing + Specced + Todo tasks across every
// project. Specced sits between Doing and Todo — a task there has a design but
// no work started yet.
func hubBoardHTML(projects []HubProject, web bool) string {
	var doing, specced, todo strings.Builder
	nDoing, nSpecced, nTodo := 0, 0, 0
	for _, p := range projects {
		tasks, _ := p.Store.ListNodes("task")
		for _, t := range tasks {
			card := hubTaskCard(p.ID, t.FM.Title, t.FM.Effort, t.Slug, web)
			switch t.FM.Status {
			case "doing":
				doing.WriteString(card)
				nDoing++
			case "specced":
				specced.WriteString(card)
				nSpecced++
			case "todo", "":
				todo.WriteString(card)
				nTodo++
			}
		}
	}
	inner := fmt.Sprintf(`<h2>Doing — %d</h2>`, nDoing) + orEmpty(doing.String(), "Nothing in progress.") +
		fmt.Sprintf(`<h2>Specced — %d</h2>`, nSpecced) + orEmpty(specced.String(), "Nothing specced.") +
		fmt.Sprintf(`<h2>Todo — %d</h2>`, nTodo) + orEmpty(todo.String(), "Nothing queued.")
	return hubShell("board", inner, web, projects)
}

// hubRoadmapHTML renders epics grouped by horizon across every project.
func hubRoadmapHTML(projects []HubProject, web bool) string {
	buckets := map[string]*strings.Builder{"now": {}, "next": {}, "later": {}, "": {}}
	for _, p := range projects {
		epics, _ := p.Store.ListNodes("epic")
		// Tasks, because the status shown is the one computed from them — the
		// stored epic field is a stamp-time cache and drifts between stamps.
		tasks, _ := p.Store.ListNodes("task")
		for _, e := range epics {
			h := e.FM.Horizon
			if _, ok := buckets[h]; !ok {
				h = ""
			}
			fmt.Fprintf(buckets[h], `<a class="card" href="%s"><div class="t">%s <span class="eff">%s</span></div><div class="proj">%s</div></a>`,
				projHref(p.ID, e.Slug, web), html.EscapeString(e.FM.Title),
				html.EscapeString(orDash(core.EpicDisplayStatus(tasks, e))), html.EscapeString(p.ID))
		}
	}
	var b strings.Builder
	for _, h := range []struct{ key, label string }{{"now", "Now"}, {"next", "Next"}, {"later", "Later"}, {"", "Unscheduled"}} {
		if buckets[h.key].Len() == 0 {
			continue
		}
		fmt.Fprintf(&b, `<h2>%s</h2>%s`, h.label, buckets[h.key].String())
	}
	return hubShell("roadmap", orEmpty(b.String(), "No epics yet."), web, projects)
}

// hubLogHTML renders one reverse-chronological feed of every project's log
// entries (blog posts), newest first, each linking into its project's viewer.
func hubLogHTML(projects []HubProject, web bool) string {
	type row struct {
		pid, title, date, tm, slug string
		draft                      bool
	}
	var rows []row
	for _, p := range projects {
		entries, _ := p.Store.ListNodes("entry")
		for _, e := range entries {
			rows = append(rows, row{p.ID, e.FM.Title, e.FM.Date, e.FM.Time, e.Slug, e.IsDraft()})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].date != rows[j].date {
			return rows[i].date > rows[j].date
		}
		return rows[i].tm > rows[j].tm
	})
	var b strings.Builder
	if len(rows) == 0 {
		b.WriteString(`<p class="proj">No log entries yet across any project.</p>`)
	}
	for _, r := range rows {
		draft := ""
		if r.draft {
			draft = ` <span class="eff">draft</span>`
		}
		fmt.Fprintf(&b, `<a class="card" href="%s"><div class="t">%s%s</div><div class="m"><span class="b">%s</span><span class="proj">%s</span></div></a>`,
			projHref(r.pid, r.slug, web), html.EscapeString(r.title), draft, html.EscapeString(r.date), html.EscapeString(r.pid))
	}
	return hubShell("log", b.String(), web, projects)
}

// BuildHub writes a static aggregate site to outDir: the hub index/log/board/
// roadmap as .html files, plus each project's full static site under p/<id>/.
func BuildHub(projects []HubProject, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	pages := map[string]string{
		"index.html":   hubIndexHTML(projects, false),
		"log.html":     hubLogHTML(projects, false),
		"board.html":   hubBoardHTML(projects, false),
		"roadmap.html": hubRoadmapHTML(projects, false),
	}
	for name, html := range pages {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(html), 0o644); err != nil {
			return err
		}
	}
	for _, p := range projects {
		dir := filepath.Join(outDir, "p", p.ID)
		if err := Build(p.Store, dir); err != nil {
			return fmt.Errorf("%s: %w", p.ID, err)
		}
		// Sibling list for the viewer's project switcher (relative hrefs).
		if err := os.WriteFile(filepath.Join(dir, "projects.json"), projectsJSON(projects, false), 0o644); err != nil {
			return fmt.Errorf("%s: projects.json: %w", p.ID, err)
		}
	}
	return nil
}

func orEmpty(s, empty string) string {
	if s == "" {
		return `<p class="proj">` + empty + `</p>`
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// attr escapes a (config-controlled) value for use inside an HTML attribute,
// e.g. an accent color dropped into a style="" attribute.
func attr(s string) string {
	return strings.NewReplacer(`&`, "&amp;", `"`, "&quot;", `<`, "&lt;", `>`, "&gt;").Replace(s)
}
