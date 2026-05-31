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
