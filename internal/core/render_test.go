package core

import (
	"strings"
	"testing"
)

func TestComponents(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // substring the rendered HTML must contain
	}{
		{"note", "```note\nThis ships **soon**.\n```\n", `class="dl-callout dl-callout-note"`},
		{"note-md", "```note\nThis ships **soon**.\n```\n", "<strong>soon</strong>"},
		{"warning", "```warning\nBe careful.\n```\n", `class="dl-callout dl-callout-warning"`},
		{"compare", "```compare\nbefore: media/a.png\nafter: media/b.png\n```\n", `class="dl-compare"`},
		{"embed", "```embed\nsrc: media/x.html\nheight: 300\n```\n", `<iframe src="media/x.html"`},
		{"video", "```video\nsrc: media/v.mp4\n```\n", "<video"},
		{"gallery", "```gallery\n- src: media/a.png\n  cap: one\n- src: media/b.png\n```\n", `class="dl-gallery"`},
		{"plaincode", "```go\nfmt.Println(1)\n```\n", `class="dl-code"`},
		{"image", "![cap](media/a.png)\n", `<img src="media/a.png"`},
		{"heading", "## Title\n\ntext\n", "<h2"},
	}
	for _, c := range cases {
		out, err := RenderMarkdown(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: want substring %q, got:\n%s", c.name, c.want, out)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	src := []byte("---\ntitle: Hi\ndate: \"2026-01-01\"\ntags:\n    - a\nhash: abc1234\nstat:\n    f: 2\n    a: 10\n    d: 3\n---\n\nBody **here**.\n")
	e, err := parseEntryBytes("/x/2026-01-01-hi.md", src)
	if err != nil {
		t.Fatal(err)
	}
	if e.FM.Title != "Hi" || e.FM.Hash != "abc1234" || e.FM.Stat == nil || e.FM.Stat.A != 10 {
		t.Fatalf("bad parse: %+v", e.FM)
	}
	if e.IsDraft() {
		t.Fatal("should not be a draft (has hash)")
	}
	if e.Slug != "hi" {
		t.Fatalf("slug = %q", e.Slug)
	}
	if strings.TrimSpace(e.Body) != "Body **here**." {
		t.Fatalf("body = %q", e.Body)
	}
}

// TestEmbedSandboxIsExact pins the attribute itself. allow-same-origin beside
// allow-scripts lets a frame remove its own sandbox, so a change here is a
// security decision and has to show up as a failing test.
func TestEmbedSandboxIsExact(t *testing.T) {
	const want = `sandbox="allow-scripts allow-pointer-lock"`
	out, err := RenderMarkdown("```embed\nsrc: media/x.html\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("embed lost its exact sandbox %s:\n%s", want, out)
	}
	if strings.Contains(out, "allow-same-origin") {
		t.Fatalf("embed must stay in an opaque origin:\n%s", out)
	}
}

// TestUnknownEmbedKeyCannotWidenSandbox: nothing an author writes in the fence
// reaches the attribute.
func TestUnknownEmbedKeyCannotWidenSandbox(t *testing.T) {
	out, err := RenderMarkdown("```embed\nsrc: media/x.html\nsandbox: allow-same-origin\nsameOrigin: true\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "allow-same-origin") {
		t.Fatalf("fence keys must not reach the sandbox:\n%s", out)
	}
}

func TestRawHTMLSwitch(t *testing.T) {
	src := "<div onclick=\"x()\">raw</div>\n\n[bad](javascript:alert(1))\n\n" +
		"```note\n<img src=x onerror=y()>\n\n**kept**\n```\n\n" +
		"```embed\nsrc: media/x.html\n```\n"
	render := func(sanitize bool) string {
		t.Helper()
		out, err := NewRenderer(func(string) bool { return true }, sanitize).Render(src)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	allowed := render(false)
	for _, want := range []string{`onclick="x()"`, `onerror=y()`} {
		if !strings.Contains(allowed, want) {
			t.Errorf("allow must pass raw HTML through unchanged; missing %s in:\n%s", want, allowed)
		}
	}

	safe := render(true)
	for _, banned := range []string{"onclick", "onerror", "javascript:"} {
		if strings.Contains(safe, banned) {
			t.Errorf("sanitize left %q in:\n%s", banned, safe)
		}
	}
	for _, want := range []string{"<iframe", "<strong>kept</strong>", "dl-callout-note"} {
		if !strings.Contains(safe, want) {
			t.Errorf("sanitize must keep components and markdown; missing %q in:\n%s", want, safe)
		}
	}
}

func TestSanitizeRawHTMLFailsClosed(t *testing.T) {
	for value, want := range map[string]bool{
		"": false, "allow": false, " Allow ": false,
		"sanitize": true, "SANITIZE": true, "alow": true, "off": true,
	} {
		if got := (Config{RawHTML: value}).SanitizeRawHTML(); got != want {
			t.Errorf("RawHTML %q: got %v, want %v", value, got, want)
		}
	}
}
