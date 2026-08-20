package core

import (
	"strings"
	"testing"
)

// TestAppendBodyReplacesDraftPlaceholder pins the fix for the bug that left
// "Start writing here." stranded above the real prose in every entry: the first
// append to a fresh draft overwrites the placeholder, later appends stack up
// underneath as usual.
func TestAppendBodyReplacesDraftPlaceholder(t *testing.T) {
	s := newTestStore(t)
	e, err := s.NewEntry(Frontmatter{Title: "Hello World"}, DraftPlaceholderBody)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}

	if err := s.AppendBody(e, "First real paragraph."); err != nil {
		t.Fatalf("AppendBody: %v", err)
	}
	if strings.Contains(e.Body, DraftPlaceholder) {
		t.Errorf("placeholder survived the first append:\n%s", e.Body)
	}
	if got, want := strings.TrimSpace(e.Body), "First real paragraph."; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}

	if err := s.AppendBody(e, "Second paragraph."); err != nil {
		t.Fatalf("AppendBody: %v", err)
	}
	if got, want := strings.TrimSpace(e.Body), "First real paragraph.\n\nSecond paragraph."; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}

	// The saved file must match what's in memory — the bug was only ever
	// visible on disk, since the viewer stripped the prefix at render time.
	reread, err := s.Get(e.Slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if strings.Contains(reread.Body, DraftPlaceholder) {
		t.Errorf("placeholder persisted to disk:\n%s", reread.Body)
	}
}

// TestIsDraftPlaceholder covers the whitespace variants a hand-edited draft or
// a round-trip through Save() can produce.
func TestIsDraftPlaceholder(t *testing.T) {
	yes := []string{DraftPlaceholder, DraftPlaceholderBody, "\n" + DraftPlaceholder + "\n\n", "  Start writing here.  "}
	for _, b := range yes {
		if !IsDraftPlaceholder(b) {
			t.Errorf("IsDraftPlaceholder(%q) = false, want true", b)
		}
	}
	no := []string{"", "Start writing here. And then I did.", "## Heading\n\nStart writing here."}
	for _, b := range no {
		if IsDraftPlaceholder(b) {
			t.Errorf("IsDraftPlaceholder(%q) = true, want false", b)
		}
	}
}

// TestMediaRefsAndHasVisual pins what counts as an entry "having a picture":
// any media/ reference in the body — markdown image or a component fence's
// src: line — or a cover in the frontmatter.
func TestMediaRefsAndHasVisual(t *testing.T) {
	body := "Intro.\n\n![shot](media/shot.png)\n\n```video\nsrc: media/clip.mp4\n```\n"
	refs := MediaRefs(body)
	if len(refs) != 2 || refs[0] != "media/shot.png" || refs[1] != "media/clip.mp4" {
		t.Errorf("MediaRefs = %v, want [media/shot.png media/clip.mp4]", refs)
	}

	cases := []struct {
		name string
		e    Entry
		want bool
	}{
		{"markdown image", Entry{Body: "![x](media/shot.png)"}, true},
		{"embed fence", Entry{Body: "```embed\nsrc: media/chart.html\n```"}, true},
		{"cover only", Entry{FM: Frontmatter{Cover: "media/hero.png"}, Body: "Prose."}, true},
		{"prose only", Entry{Body: "Nothing but words about media and pictures."}, false},
		{"empty", Entry{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HasVisual(c.e); got != c.want {
				t.Errorf("HasVisual = %v, want %v", got, c.want)
			}
		})
	}
}

// TestIsUnwrittenBody: the question a gate asks — empty and placeholder-only
// bodies are both "nothing written yet", anything else is writing.
func TestIsUnwrittenBody(t *testing.T) {
	yes := []string{"", "   \n\n", DraftPlaceholder, DraftPlaceholderBody, "\n" + DraftPlaceholder + "\n"}
	for _, b := range yes {
		if !IsUnwrittenBody(b) {
			t.Errorf("IsUnwrittenBody(%q) = false, want true", b)
		}
	}
	no := []string{"A sentence.", DraftPlaceholder + "\n\nAnd then real prose.", "![x](media/a.png)"}
	for _, b := range no {
		if IsUnwrittenBody(b) {
			t.Errorf("IsUnwrittenBody(%q) = true, want false", b)
		}
	}
}
