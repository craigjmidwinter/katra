package viewer

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// The exact header values. These literals are deliberately repeated rather than
// built from the constants: changing the policy should have to touch this file.
const (
	wantPageCSP = "default-src 'self'; script-src 'self'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob:; " +
		"connect-src 'self'; object-src 'none'; form-action 'none'; base-uri 'none'; " +
		"frame-ancestors 'none'"
	wantMediaCSP = "sandbox allow-scripts allow-pointer-lock; default-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; " +
		"object-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'self'; " +
		"script-src 'self'"
)

func secStore(t *testing.T) *core.Store {
	t.Helper()
	s, err := core.InitStore(filepath.Join(t.TempDir(), "katra"), "Sec")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeMedia(t *testing.T, s *core.Store, name, body string) {
	t.Helper()
	p := filepath.Join(s.MediaDir(), name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bothServers runs a check against serve and hub serve, since they are separate
// muxes and a route added to one is easily missed on the other. prefix is where
// the store's viewer lives.
func bothServers(t *testing.T, s *core.Store, check func(t *testing.T, h http.Handler, prefix string)) {
	t.Helper()
	t.Run("serve", func(t *testing.T) {
		check(t, serveHandler(s, &reloadHub{clients: map[chan string]struct{}{}}), "/")
	})
	t.Run("hub", func(t *testing.T) {
		set := newHubSet(func() ([]HubProject, error) { return []HubProject{{ID: "sec", Store: s}}, nil })
		set.refresh()
		check(t, hubHandler(set), "/p/sec/")
	})
}

func get(h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPageHeadersOnEveryRoute(t *testing.T) {
	s := secStore(t)
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		for _, path := range []string{prefix, prefix + "app.js", prefix + "styles.css", prefix + "livereload.js", prefix + "data.json", prefix + "no-such-file"} {
			w := get(h, path, nil)
			if got := w.Header().Get("Content-Security-Policy"); got != wantPageCSP {
				t.Errorf("%s CSP:\n got %s\nwant %s", path, got, wantPageCSP)
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("%s nosniff: %q", path, got)
			}
			if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
				t.Errorf("%s referrer: %q", path, got)
			}
			// serve and hub serve have no sessions, so the HttpOnly/SameSite
			// requirement holds vacuously. This keeps it vacuous.
			if got := w.Header().Get("Set-Cookie"); got != "" {
				t.Errorf("%s set a cookie: %q", path, got)
			}
		}
	})
}

func TestHubPagesCarryPolicy(t *testing.T) {
	s := secStore(t)
	set := newHubSet(func() ([]HubProject, error) { return []HubProject{{ID: "sec", Store: s}}, nil })
	set.refresh()
	h := hubHandler(set)
	for _, path := range []string{"/", "/board", "/roadmap", "/log", "/api/hub.json"} {
		if got := get(h, path, nil).Header().Get("Content-Security-Policy"); got != wantPageCSP {
			t.Errorf("%s CSP: %s", path, got)
		}
	}
	if body := get(h, "/", nil).Body.String(); !strings.Contains(body, metaPolicy) {
		t.Error("hub pages are also built statically, so they must carry the meta policy")
	}
}

func TestMediaHeaders(t *testing.T) {
	s := secStore(t)
	writeMedia(t, s, "chart.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	writeMedia(t, s, "noext", `<!doctype html><script>alert(1)</script>`)
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		// Script-capable formats get the sandbox and no script hash: an SVG
		// and a sniffed-as-HTML file are the ways round an extension check.
		for _, name := range []string{"chart.svg", "noext"} {
			w := get(h, prefix+"media/"+name, nil)
			if got := w.Header().Get("Content-Security-Policy"); got != wantMediaCSP {
				t.Errorf("%s CSP:\n got %s\nwant %s", name, got, wantMediaCSP)
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("%s nosniff: %q", name, got)
			}
		}
	})
}

func TestMediaPolicyHashesInlineScripts(t *testing.T) {
	js := "alert(1);\n"
	sum := sha256.Sum256([]byte(js))
	want := wantMediaCSP + " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"

	if got := MediaPolicy([]byte("<SCRIPT>alert(1);\r\n</SCRIPT><script src=a.js></script>")); got != want {
		t.Errorf("CRLF and case are normalised, external scripts skipped:\n got %s\nwant %s", got, want)
	}
	if strings.Contains(MediaPolicy([]byte("<script>x()</script>")), "unsafe-inline' 'sha") {
		t.Error("script-src must not follow 'unsafe-inline'")
	}
	for _, p := range []string{wantPageCSP, MediaPolicy([]byte("<script>x()</script>"))} {
		for _, d := range strings.Split(p, "; ") {
			if strings.HasPrefix(d, "script-src") && strings.Contains(d, "unsafe") {
				t.Errorf("script-src may not use unsafe keywords: %s", d)
			}
		}
	}
}

func TestMediaDirectoryIndexGetsItsHashes(t *testing.T) {
	s := secStore(t)
	writeMedia(t, s, "build/index.html", "<script>go()</script>")
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		csp := get(h, prefix+"media/build/", nil).Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "'sha256-") {
			t.Errorf("a directory's index needs its hashes: %s", csp)
		}
	})
}

func TestMediaCORSAnswersOnlyTheOpaqueOrigin(t *testing.T) {
	s := secStore(t)
	writeMedia(t, s, "a.json", "{}")
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		w := get(h, prefix+"media/a.json", map[string]string{"Origin": "null"})
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "null" {
			t.Errorf("opaque origin: %q", got)
		}
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("Vary: %q", got)
		}
		w = get(h, prefix+"media/a.json", map[string]string{"Origin": "https://evil.example"})
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("a website must not get CORS access: %q", got)
		}
		if got := get(h, prefix+"data.json", map[string]string{"Origin": "null"}).Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("data.json must stay out of an embed's reach: %q", got)
		}
	})
}

func TestMediaRefusesToRunAsViewerOriginScript(t *testing.T) {
	s := secStore(t)
	writeMedia(t, s, "x.js", "evil()")
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		for _, dest := range []string{"script", "worker", "sharedworker", "serviceworker"} {
			w := get(h, prefix+"media/x.js", map[string]string{"Sec-Fetch-Site": "same-origin", "Sec-Fetch-Dest": dest})
			if w.Code != http.StatusForbidden {
				t.Errorf("same-origin %s: got %d", dest, w.Code)
			}
		}
		w := get(h, prefix+"media/x.js", map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Dest": "script"})
		if w.Code != http.StatusOK {
			t.Errorf("an embed's own script must load: got %d", w.Code)
		}
	})
}

func TestMediaStaysContained(t *testing.T) {
	s := secStore(t)
	if err := os.WriteFile(filepath.Join(s.Dir, "config.yml"), []byte("title: Sec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.URL.Path = prefix + "media/../config.yml"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "title: Sec") {
			t.Errorf("traversal reached config.yml: %d", w.Code)
		}
	})
}

// TestMetaPolicyIsTheHeaderPolicyMinusFrameAncestors: frame-ancestors in a <meta>
// is ignored and logged, so it must stay out, and everything else must agree.
func TestMetaPolicyIsTheHeaderPolicyMinusFrameAncestors(t *testing.T) {
	if strings.Contains(metaPolicy, "frame-ancestors") || strings.Contains(metaPolicy, "sandbox") {
		t.Errorf("header-only directives in the meta policy: %s", metaPolicy)
	}
	base := strings.TrimSuffix(wantPageCSP, "; frame-ancestors 'none'")
	if !strings.Contains(metaPolicy, `content="`+base+`"`) {
		t.Errorf("meta policy drifted from the header:\n%s", metaPolicy)
	}
	if !strings.Contains(metaPolicy, `<meta name="referrer" content="no-referrer">`) {
		t.Error("static builds need no-referrer in the page")
	}
}

func TestStaticBuildCarriesPolicyAheadOfContent(t *testing.T) {
	s := secStore(t)
	out := t.TempDir()
	if err := Build(s, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	csp := strings.Index(page, "Content-Security-Policy")
	if csp < 0 || csp > strings.Index(page, "<link") {
		t.Fatal("the meta policy must precede every resource it governs")
	}
	if strings.Contains(page, "<script>") {
		t.Error("the CSP forbids inline scripts, so the shell cannot have one")
	}
	if _, err := os.Stat(filepath.Join(out, "livereload.js")); !os.IsNotExist(err) {
		t.Error("livereload is dev-server only")
	}
}

// TestViewerNeverReachesAcrossTheFrame covers the embed-reach checks that are
// properties of what the viewer does not do.
func TestViewerNeverReachesAcrossTheFrame(t *testing.T) {
	for _, name := range []string{"app.js", "livereload.js", "index.html"} {
		b, err := Asset(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"postMessage", "document.cookie", "localStorage", "sessionStorage"} {
			if strings.Contains(string(b), banned) {
				t.Errorf("%s uses %s: the viewer has no channel to embeds and stores nothing in the browser", name, banned)
			}
		}
	}
}
