package viewer

import (
	"crypto/sha1"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/craigjmidwinter/devlog/internal/core"
)

// reloadClient is injected before </body> on the served page. It opens an SSE
// stream and reloads the tab whenever the watcher reports a change.
const reloadClient = `
<script>
(function(){
  if (location.search.indexOf('noreload') !== -1) return;
  var es = new EventSource('/__livereload');
  es.onmessage = function(e){ if (e.data === 'reload') location.reload(); };
})();
</script>`

// Serve runs a live-reloading dev server for the store. data.json is generated
// per request (always fresh); a poll-based watcher reloads open tabs when any
// entry or media file changes — no external watcher dependency.
func Serve(s *core.Store, port int) error {
	hub := &reloadHub{clients: map[chan string]struct{}{}}
	go watch(s.Dir, hub)

	mux := http.NewServeMux()

	mux.HandleFunc("/__livereload", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		b, err := BuildData(s)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/media/")
		p := filepath.Join(s.MediaDir(), filepath.Clean("/"+rel))
		if !strings.HasPrefix(p, s.MediaDir()) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, p)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		b, err := Asset(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ct := contentType(name)
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-cache")
		if strings.HasSuffix(name, ".html") {
			page := string(b)
			page = strings.Replace(page, "</body>", reloadClient+"\n</body>", 1)
			_, _ = w.Write([]byte(page))
			return
		}
		_, _ = w.Write(b)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("\n  %s — live dev log\n", s.Config.Title)
	fmt.Printf("  ───────────────────────────────\n")
	fmt.Printf("  local:   http://localhost:%d/\n", port)
	for _, ip := range lanAddrs() {
		fmt.Printf("  network: http://%s:%d/\n", ip, port)
	}
	fmt.Printf("  watching %s — open tabs reload on change. Ctrl-C to stop.\n\n", s.Dir)
	return http.ListenAndServe(addr, mux)
}

func contentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

type reloadHub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func (h *reloadHub) add() chan string {
	ch := make(chan string, 4)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *reloadHub) remove(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *reloadHub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// watch polls the devlog dir's mtimes and broadcasts a reload on any change.
func watch(dir string, hub *reloadHub) {
	last := ""
	for {
		sig := snapshot(dir)
		if last != "" && sig != last {
			hub.broadcast("reload")
		}
		last = sig
		time.Sleep(500 * time.Millisecond)
	}
}

func snapshot(dir string) string {
	h := sha1.New()
	var paths []string
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		paths = append(paths, fmt.Sprintf("%s:%d:%d", p, fi.Size(), fi.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(paths)
	for _, p := range paths {
		_, _ = h.Write([]byte(p))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func lanAddrs() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
				out = append(out, ipn.IP.String())
			}
		}
	}
	return out
}
