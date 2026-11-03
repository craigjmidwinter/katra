package viewer

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The viewer's security policy, written once. The reasoning, and what each
// serving surface can and cannot enforce, is in
// docs/design/playable-embed-isolation.md.
//
// style-src keeps 'unsafe-inline' because components and hub pages use style
// attributes and the hub inlines its stylesheet; scripts never get it. The two
// font hosts are the only third parties, and only for stylesheets and fonts.
const pagePolicyBase = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data: blob:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"form-action 'none'; " +
	"base-uri 'none'"

// PagePolicy is the header form of the page CSP. frame-ancestors is header-only:
// browsers ignore it in a <meta>.
const PagePolicy = pagePolicyBase + "; frame-ancestors 'none'"

// metaPolicy carries what a static build can state for itself, since it has no
// response headers of its own.
const metaPolicy = `<meta http-equiv="Content-Security-Policy" content="` + pagePolicyBase + `">` +
	`<meta name="referrer" content="no-referrer">`

// mediaPolicyPrefix sandboxes the response itself, so media never runs on the
// viewer origin however it is reached: opened directly, or framed by raw HTML
// that left the sandbox attribute off. The directive matches core.EmbedSandbox.
// connect-src 'self' leaves a multi-file build able to read its siblings and
// nothing beyond this server.
const mediaPolicyPrefix = "sandbox allow-scripts allow-pointer-lock; " +
	"default-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"form-action 'none'; " +
	"base-uri 'none'; " +
	"frame-ancestors 'self'; " +
	"script-src 'self'"

// MediaPolicy returns the CSP for a media response. A widget is usually one
// self-contained file, so inline scripts in the HTML being served are allowed
// by hash rather than with 'unsafe-inline'.
func MediaPolicy(body []byte) string {
	policy := mediaPolicyPrefix
	for _, script := range inlineScripts(body) {
		sum := sha256.Sum256(script)
		policy += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	return policy
}

var (
	scriptTag = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	srcAttr   = regexp.MustCompile(`(?i)(^|\s)src\s*=`)
)

// inlineScripts returns the bodies of the document's inline scripts as a browser
// will hash them, with line endings normalised. This is a scan, not a parse,
// which is safe because the mistake only runs one way: a script it misreads
// gets a hash that does not match, and stays blocked.
func inlineScripts(doc []byte) [][]byte {
	var out [][]byte
	for _, m := range scriptTag.FindAllSubmatch(doc, -1) {
		if srcAttr.Match(m[1]) {
			continue
		}
		s := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(string(m[2]))
		out = append(out, []byte(s))
	}
	return out
}

// securityHeaders sets the headers every response shares. Media replaces the
// CSP with its own. It does not wrap the ResponseWriter, so the SSE handlers
// keep their http.Flusher.
//
// No cookie is set anywhere: serve and hub serve have no sessions, by design.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", PagePolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// serveMedia serves one file from a store's media directory. Both servers come
// through here, so no route can miss the sandbox.
func serveMedia(w http.ResponseWriter, r *http.Request, mediaDir, rel string) {
	p := filepath.Join(mediaDir, filepath.Clean("/"+rel))
	if !strings.HasPrefix(p, mediaDir) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if runsOnViewerOrigin(r) {
		http.Error(w, "media cannot run as viewer-origin script — load it from inside an embed", http.StatusForbidden)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-cache")
	h.Set("Content-Security-Policy", MediaPolicy(htmlToServe(p)))
	// A sandboxed embed's origin is opaque and serialises as "null". Answering
	// only that stops a plain cross-site fetch, but it is not a boundary: any
	// site can sandbox a frame of its own and send the same Origin. Treat media
	// as readable by whoever can reach the server, which is already the no-auth
	// model. See the design note.
	h.Add("Vary", "Origin")
	if r.Header.Get("Origin") == "null" {
		h.Set("Access-Control-Allow-Origin", "null")
	}
	http.ServeFile(w, r, p)
}

// runsOnViewerOrigin spots a media file being loaded as code by a page on the
// viewer origin. script-src 'self' cannot tell app.js from a committed media/x.js,
// and a raw <iframe srcdoc> inherits the viewer origin, so this refusal is what
// keeps media code off it. An embed is opaque, so its fetches are not same-origin.
func runsOnViewerOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") != "same-origin" {
		return false
	}
	switch r.Header.Get("Sec-Fetch-Dest") {
	case "script", "worker", "sharedworker", "serviceworker":
		return true
	}
	return false
}

// htmlToServe returns the HTML behind a media path, or nil for anything else. A
// directory is what its index.html serves as. Files that http.ServeFile would
// sniff as HTML get no hashes and so run no inline script.
func htmlToServe(p string) []byte {
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		p = filepath.Join(p, "index.html")
	}
	switch strings.ToLower(filepath.Ext(p)) {
	case ".html", ".htm":
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		return b
	}
	return nil
}
