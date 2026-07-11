package viewer

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
)

// HubProject is one katra served by the hub under /p/<ID>/. When LogDir is set,
// the hub serves that custom static site instead of the built-in viewer.
type HubProject struct {
	ID     string
	Store  *core.Store
	LogDir string // absolute path to a custom static log site, or "" for the built-in viewer
}

// ServeHub runs the multi-tenant hub: one server, an index of every registered
// katra at /, and each project's viewer at /p/<id>/. data.json is generated per
// request; any change in any store reloads open tabs (one global livereload).
func ServeHub(projects []HubProject, port int) error {
	byID := make(map[string]HubProject, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}

	hub := &reloadHub{clients: map[chan string]struct{}{}}
	for _, p := range projects {
		go watch(p.Store.Dir, hub)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/__livereload", sseHandler(hub))

	mux.HandleFunc("/p/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/p/")
		id, sub, hasSlash := strings.Cut(rest, "/")
		proj, ok := byID[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		store := proj.Store
		if !hasSlash {
			http.Redirect(w, r, "/p/"+id+"/", http.StatusFound)
			return
		}
		// Custom log site: serve the project's own static files verbatim.
		if proj.LogDir != "" {
			name := sub
			if name == "" {
				name = "index.html"
			}
			p := filepath.Join(proj.LogDir, filepath.Clean("/"+name))
			if !strings.HasPrefix(p, proj.LogDir) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, p)
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

	page := func(html string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write([]byte(html))
		}
	}
	mux.HandleFunc("/board", func(w http.ResponseWriter, r *http.Request) {
		page(hubBoardHTML(projects, true))(w, r)
	})
	mux.HandleFunc("/roadmap", func(w http.ResponseWriter, r *http.Request) {
		page(hubRoadmapHTML(projects, true))(w, r)
	})
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		page(hubLogHTML(projects, true))(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
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

// hubShell wraps inner HTML in the common page chrome + nav. active is the
// current section ("projects"|"board"|"roadmap") for highlighting.
func hubShell(active, inner string, web bool) string {
	nav := ""
	for _, l := range []struct{ label, key string }{
		{"Projects", "projects"}, {"Log", "log"}, {"Board", "board"}, {"Roadmap", "roadmap"},
	} {
		cls := "nav"
		if l.key == active {
			cls = "nav on"
		}
		nav += fmt.Sprintf(`<a class="%s" href="%s">%s</a>`, cls, navHref(l.key, web), l.label)
	}
	return `<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>Katra hub</title><style>` +
		`:root{color-scheme:light dark}body{font:16px/1.5 system-ui,sans-serif;max-width:820px;margin:2.2rem auto;padding:0 1.25rem}` +
		`h1{font-size:1.3rem;margin:0 0 .3rem}h2{font-size:.8rem;text-transform:uppercase;letter-spacing:.05em;opacity:.55;margin:1.6rem 0 .5rem}` +
		`nav{margin:.2rem 0 1.4rem}a.nav{text-decoration:none;color:inherit;opacity:.55;margin-right:1rem;font-size:.95rem}a.nav.on{opacity:1;font-weight:600;border-bottom:2px solid currentColor;padding-bottom:2px}` +
		`a.card{display:block;text-decoration:none;color:inherit;border:1px solid #8883;border-radius:10px;padding:.7rem .95rem;margin:.5rem 0}a.card:hover{border-color:#8886;background:#8881}` +
		`.t{font-weight:600}.m{opacity:.55;font-size:.82rem;margin-top:.15rem}.b{display:inline-block;margin-right:.8rem}.proj{opacity:.55;font-size:.8rem}` +
		`.eff{opacity:.55;font-size:.78rem;border:1px solid #8884;border-radius:4px;padding:0 .3rem;margin-left:.4rem}` +
		`</style></head><body><h1>Katra hub</h1><nav>` + nav + `</nav>` + inner + `</body></html>`
}

// hubIndexHTML renders the project index page (cards, per-project counts).
func hubIndexHTML(projects []HubProject, web bool) string {
	var b strings.Builder
	if len(projects) == 0 {
		b.WriteString(`<p>No katras registered yet. Run <code>katra init</code> in a repo.</p>`)
	}
	for _, p := range projects {
		entries, _ := p.Store.ListNodes("entry")
		tasks, _ := p.Store.ListNodes("task")
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
		title := p.Store.Config.Title
		if title == "" {
			title = p.ID
		}
		badge := ""
		if p.LogDir != "" {
			badge = ` <span class="eff">custom log</span>`
		}
		fmt.Fprintf(&b, `<a class="card" href="%s"><div class="t">%s%s</div>`,
			projHref(p.ID, "", web), html.EscapeString(title), badge)
		fmt.Fprintf(&b, `<div class="m"><span class="b">%d entries</span><span class="b">%d drafts</span><span class="b">%d doing</span><span class="b">%s</span></div></a>`,
			len(entries), drafts, doing, html.EscapeString(p.Store.Dir))
	}
	return hubShell("projects", b.String(), web)
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

// hubBoardHTML renders one board of Doing + Todo tasks across every project.
func hubBoardHTML(projects []HubProject, web bool) string {
	var doing, todo strings.Builder
	nDoing, nTodo := 0, 0
	for _, p := range projects {
		tasks, _ := p.Store.ListNodes("task")
		for _, t := range tasks {
			card := hubTaskCard(p.ID, t.FM.Title, t.FM.Effort, t.Slug, web)
			switch t.FM.Status {
			case "doing":
				doing.WriteString(card)
				nDoing++
			case "todo", "":
				todo.WriteString(card)
				nTodo++
			}
		}
	}
	inner := fmt.Sprintf(`<h2>Doing — %d</h2>`, nDoing) + orEmpty(doing.String(), "Nothing in progress.") +
		fmt.Sprintf(`<h2>Todo — %d</h2>`, nTodo) + orEmpty(todo.String(), "Nothing queued.")
	return hubShell("board", inner, web)
}

// hubRoadmapHTML renders epics grouped by horizon across every project.
func hubRoadmapHTML(projects []HubProject, web bool) string {
	buckets := map[string]*strings.Builder{"now": {}, "next": {}, "later": {}, "": {}}
	for _, p := range projects {
		epics, _ := p.Store.ListNodes("epic")
		for _, e := range epics {
			h := e.FM.Horizon
			if _, ok := buckets[h]; !ok {
				h = ""
			}
			fmt.Fprintf(buckets[h], `<a class="card" href="%s"><div class="t">%s <span class="eff">%s</span></div><div class="proj">%s</div></a>`,
				projHref(p.ID, e.Slug, web), html.EscapeString(e.FM.Title),
				html.EscapeString(orDash(e.FM.Status)), html.EscapeString(p.ID))
		}
	}
	var b strings.Builder
	for _, h := range []struct{ key, label string }{{"now", "Now"}, {"next", "Next"}, {"later", "Later"}, {"", "Unscheduled"}} {
		if buckets[h.key].Len() == 0 {
			continue
		}
		fmt.Fprintf(&b, `<h2>%s</h2>%s`, h.label, buckets[h.key].String())
	}
	return hubShell("roadmap", orEmpty(b.String(), "No epics yet."), web)
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
	return hubShell("log", b.String(), web)
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
		if err := Build(p.Store, filepath.Join(outDir, "p", p.ID)); err != nil {
			return fmt.Errorf("%s: %w", p.ID, err)
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
