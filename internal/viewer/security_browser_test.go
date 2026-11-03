package viewer

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/craigjmidwinter/katra/internal/core"
)

// These tests run hostile fixtures in a real browser, because the claim under
// test ("an embed cannot read parent.document") is browser behaviour that no
// assertion over HTML can establish. Each fixture reports what it managed to do
// by loading /probe?<name>; the test then reads the server's log. CI sets
// KATRA_REQUIRE_BROWSER so that a missing browser fails rather than skips there.

func findBrowser(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("KATRA_BROWSER"); p != "" {
		return p
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if os.Getenv("KATRA_REQUIRE_BROWSER") != "" {
		t.Fatal("KATRA_REQUIRE_BROWSER is set but no browser was found — install Chrome or set KATRA_BROWSER")
	}
	t.Skip("no browser found — set KATRA_BROWSER to run the embed isolation tests")
	return ""
}

// report is the fixture's only way out. An image request works from an opaque
// origin and is what the embed policy permits, so a failure to report means the
// script did not run, not that the channel was closed.
const report = `function report(n){new Image().src="/probe?"+n}`

// probeWidget is the embed: a single file whose inline script tries every
// reach Bosun's checklist names.
const probeWidget = `<!doctype html><title>probe</title>
<img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" onerror="report('LEAK-inline-handler-ran')" onload="report('LEAK-inline-handler-ran')">
<script>` + report + `
report("PAGE-script-ran");
report(self.origin==="null"?"PAGE-origin-opaque":"LEAK-origin-"+encodeURIComponent(self.origin));
try{var t=parent.document.title;if(window!==parent)report("LEAK-parent-document");else report("PAGE-toplevel")}
catch(e){report("PAGE-parent-blocked-"+e.name)}
try{document.cookie;report("LEAK-cookie")}catch(e){report("PAGE-cookie-blocked")}
try{localStorage.length;report("LEAK-storage")}catch(e){report("PAGE-storage-blocked")}
fetch("sibling.json").then(function(r){return r.json()}).then(function(j){report("PAGE-sibling-"+j.ok)},function(e){report("PAGE-sibling-failed")});
fetch(location.pathname.replace(/media\/.*/,"data.json")).then(function(r){return r.text()}).then(function(){report("LEAK-data-json")},function(){report("PAGE-data-json-blocked")});
fetch("https://example.invalid/x",{mode:"no-cors"}).then(function(){report("LEAK-external")},function(){report("PAGE-external-blocked")});
</script>
<script src="sibling.js"></script>`

const entryBody = "viewer-rendered-marker\n\n" +
	"```embed\nsrc: media/widget.html\nheight: 40\n```\n\n" +
	// Raw HTML attacks on the viewer origin.
	"<img src=\"x\" onerror=\"new Image().src='/probe?LEAK-raw-handler'\">\n\n" +
	"<iframe srcdoc=\"<script>new Image().src='/probe?LEAK-srcdoc-inline'</script>\" height=\"1\"></iframe>\n\n" +
	"<iframe srcdoc=\"<script src='/media/evil.js'></script><script src='media/evil.js'></script>\" height=\"1\"></iframe>\n\n" +
	"<iframe src=\"media/unsandboxed.html\" height=\"1\"></iframe>\n\n" +
	"<a id=\"j\" href=\"javascript:new Image().src='/probe?LEAK-js-link'\">x</a>\n"

const unsandboxedWidget = `<!doctype html><script>` + report + `
try{parent.document.title;report("LEAK-unsandboxed-parent")}catch(e){report("PAGE-unsandboxed-blocked")}
</script>`

func browserStore(t *testing.T) *core.Store {
	t.Helper()
	s := secStore(t)
	writeMedia(t, s, "widget.html", probeWidget)
	writeMedia(t, s, "unsandboxed.html", unsandboxedWidget)
	writeMedia(t, s, "sibling.json", `{"ok":"loaded"}`)
	writeMedia(t, s, "sibling.js", report+`;report("PAGE-sibling-script-ran")`)
	writeMedia(t, s, "evil.js", `new Image().src="/probe?LEAK-media-script-on-origin"`)
	entry := "---\ntitle: Probe\ndate: \"2026-11-03\"\nhash: abc1234\n---\n" + entryBody
	if err := os.WriteFile(filepath.Join(s.EntriesDir(), "2026-11-03-probe.md"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	return s
}

// probeServer records /probe hits and hands everything else to h. Probes come
// from fixtures at /media/..., so it sits at the root for both servers.
func probeServer(t *testing.T, h http.Handler) (*httptest.Server, func() string) {
	t.Helper()
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/probe" {
			mu.Lock()
			hits = append(hits, r.URL.RawQuery)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, func() string {
		mu.Lock()
		defer mu.Unlock()
		return strings.Join(hits, "\n")
	}
}

func dumpDOM(t *testing.T, browser, url string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, browser, "--headless", "--no-sandbox", "--disable-gpu",
		"--user-data-dir="+t.TempDir(), "--window-size=1200,3000",
		"--virtual-time-budget=6000", "--dump-dom", url)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("browser: %v\n%s", err, errb.String())
	}
	return out.String()
}

func assertProbes(t *testing.T, log string, want ...string) {
	t.Helper()
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "LEAK") {
			t.Errorf("escaped: %s", line)
		}
	}
	for _, w := range want {
		if !strings.Contains(log, w) {
			t.Errorf("fixture never reported %s, so this run proves nothing about it. Log:\n%s", w, log)
		}
	}
}

// The reports that prove the widget really ran, and ran usefully. Without them
// a blanket block would pass every LEAK check.
var widgetRan = []string{
	"PAGE-script-ran", "PAGE-origin-opaque", "PAGE-parent-blocked-SecurityError",
	"PAGE-cookie-blocked", "PAGE-storage-blocked", "PAGE-sibling-loaded",
	"PAGE-sibling-script-ran", "PAGE-data-json-blocked", "PAGE-external-blocked",
}

func TestEmbedCannotReachViewerInBrowser(t *testing.T) {
	browser := findBrowser(t)
	s := browserStore(t)
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		srv, log := probeServer(t, h)
		// noreload: an open SSE stream would keep the page from ever settling.
		dom := dumpDOM(t, browser, srv.URL+prefix+"?noreload#/node/probe")
		// The viewer itself has to survive its own policy.
		if !strings.Contains(dom, "viewer-rendered-marker") {
			t.Fatalf("app.js did not render under the CSP:\n%s", dom)
		}
		if !strings.Contains(dom, `sandbox="`+core.EmbedSandbox+`"`) {
			t.Fatalf("the rendered iframe lost its sandbox:\n%s", dom)
		}
		assertProbes(t, log(), append([]string{"PAGE-unsandboxed-blocked"}, widgetRan...)...)
	})
}

// TestMediaOpenedDirectlyStaysOpaque is the path the iframe attribute cannot
// cover: someone follows a link straight to the file on the viewer origin.
func TestMediaOpenedDirectlyStaysOpaque(t *testing.T) {
	browser := findBrowser(t)
	s := browserStore(t)
	bothServers(t, s, func(t *testing.T, h http.Handler, prefix string) {
		srv, log := probeServer(t, h)
		dumpDOM(t, browser, srv.URL+prefix+"media/widget.html")
		assertProbes(t, log(), "PAGE-script-ran", "PAGE-origin-opaque", "PAGE-toplevel")
	})
}

// TestStaticBuildMetaPolicyInBrowser: a static site has no headers, so the meta
// policy and the iframe attribute are all there is. Serving the build from a
// plain file server reproduces that.
func TestStaticBuildMetaPolicyInBrowser(t *testing.T) {
	browser := findBrowser(t)
	s := browserStore(t)
	out := t.TempDir()
	if err := Build(s, out); err != nil {
		t.Fatal(err)
	}
	srv, log := probeServer(t, http.FileServer(http.Dir(out)))
	dom := dumpDOM(t, browser, srv.URL+"/#/node/probe")
	if !strings.Contains(dom, "viewer-rendered-marker") {
		t.Fatalf("app.js did not render under the meta policy:\n%s", dom)
	}
	got := log()
	for _, must := range []string{"PAGE-parent-blocked-SecurityError", "PAGE-origin-opaque"} {
		if !strings.Contains(got, must) {
			t.Errorf("missing %s:\n%s", must, got)
		}
	}
	for _, banned := range []string{"LEAK-parent-document", "LEAK-raw-handler", "LEAK-srcdoc-inline", "LEAK-js-link"} {
		if strings.Contains(got, banned) {
			t.Errorf("static build: %s\n%s", banned, got)
		}
	}
}
