package viewer

import (
	"fmt"
	"html"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/craigjmidwinter/devlog/internal/core"
)

// HubProject is one katra served by the hub under /p/<ID>/.
type HubProject struct {
	ID    string
	Store *core.Store
}

// ServeHub runs the multi-tenant hub: one server, an index of every registered
// katra at /, and each project's viewer at /p/<id>/. data.json is generated per
// request; any change in any store reloads open tabs (one global livereload).
func ServeHub(projects []HubProject, port int) error {
	byID := make(map[string]*core.Store, len(projects))
	for _, p := range projects {
		byID[p.ID] = p.Store
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(hubIndexHTML(projects)))
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

// hubIndexHTML renders the self-contained project index page.
func hubIndexHTML(projects []HubProject) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString(`<title>Katra hub</title><style>`)
	b.WriteString(`:root{color-scheme:light dark}body{font:16px/1.5 system-ui,sans-serif;max-width:760px;margin:3rem auto;padding:0 1.25rem}`)
	b.WriteString(`h1{font-size:1.4rem;margin:0 0 1.5rem}a.card{display:block;text-decoration:none;color:inherit;border:1px solid #8883;border-radius:10px;padding:.9rem 1.1rem;margin:.6rem 0}`)
	b.WriteString(`a.card:hover{border-color:#8886;background:#8881}.t{font-weight:600;font-size:1.05rem}.m{opacity:.6;font-size:.85rem;margin-top:.2rem}.b{display:inline-block;margin-right:.8rem}`)
	b.WriteString(`</style></head><body><h1>Katra hub</h1>`)
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
		fmt.Fprintf(&b, `<a class="card" href="/p/%s/"><div class="t">%s</div>`,
			html.EscapeString(p.ID), html.EscapeString(title))
		fmt.Fprintf(&b, `<div class="m"><span class="b">%d entries</span><span class="b">%d drafts</span><span class="b">%d doing</span><span class="b">%s</span></div></a>`,
			len(entries), drafts, doing, html.EscapeString(p.Store.Dir))
	}
	b.WriteString(`</body></html>`)
	return b.String()
}
