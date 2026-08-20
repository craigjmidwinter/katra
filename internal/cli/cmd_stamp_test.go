package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
)

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

// TestWarnNoVisual: the stamp-time nudge fires for an entry with nothing to
// look at and stays quiet otherwise. It is a note on stderr — the stamp has
// already happened, so it must never be mistaken for a failure.
func TestWarnNoVisual(t *testing.T) {
	cases := []struct {
		name string
		e    core.Entry
		warn bool
	}{
		{"prose only", core.Entry{Body: "Wrote a lot of words about the renderer."}, true},
		{"has an image", core.Entry{Body: "![shot](media/shot.png)"}, false},
		{"has a cover", core.Entry{FM: core.Frontmatter{Cover: "media/hero.png"}, Body: "Words."}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := captureStderr(t, func() { warnNoVisual(c.e) })
			if got := strings.Contains(out, "no visual"); got != c.warn {
				t.Errorf("warned = %v, want %v (stderr: %q)", got, c.warn, out)
			}
		})
	}
}
