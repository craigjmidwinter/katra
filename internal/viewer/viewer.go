// Package viewer turns a Store into a static, dependency-free website: a single
// index.html + app.js + styles.css that read a generated data.json of
// pre-rendered entries. Markdown (and the rich components) are rendered in Go,
// so the browser never needs a markdown parser and the page works offline.
package viewer

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/craigjmidwinter/katra/internal/core"
)

//go:embed assets/*
var assets embed.FS

// siteData is the shape app.js consumes.
type siteData struct {
	Site    siteMeta    `json:"site"`
	Entries []entryData `json:"entries"`
}

type siteMeta struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Accent      string `json:"accent"`
}

type entryData struct {
	Slug     string     `json:"slug"`
	Title    string     `json:"title"`
	Date     string     `json:"date"`
	Time     string     `json:"time,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	Hashes   []string   `json:"hashes,omitempty"`
	Stat     *core.Stat `json:"stat,omitempty"`
	Summary  string     `json:"summary,omitempty"`
	Cover    string     `json:"cover,omitempty"`
	Draft    bool       `json:"draft"`
	Featured bool       `json:"featured"`
	HTML     string     `json:"html"`

	// Node-model fields (Katra). "type" is always set (Kind()); the rest
	// are omitted when empty so entry records stay unchanged in shape.
	Type         string   `json:"type"`
	Status       string   `json:"status,omitempty"`
	Rollup       string   `json:"rollup,omitempty"` // epics: status computed from child tasks (suggestion)
	Spec         string   `json:"spec,omitempty"`   // task -> spec ref, resolved to a node slug when it is one (see ResolveSpec)
	Effort       string   `json:"effort,omitempty"`
	Horizon      string   `json:"horizon,omitempty"`
	Epic         string   `json:"epic,omitempty"`
	Entry        string   `json:"entry,omitempty"`
	Supersedes   []string `json:"supersedes,omitempty"`
	SupersededBy []string `json:"supersededBy,omitempty"`

	// Resolved link graph (Katra wiki). Each ref is {slug,title,type}.
	// Links = outbound (body [[wikilinks]] + structured edges); Backlinks =
	// "what links here". Only refs to real nodes appear; omitted when empty.
	Links     []linkRef `json:"links,omitempty"`
	Backlinks []linkRef `json:"backlinks,omitempty"`
}

type linkRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

func toLinkRefs(refs []core.LinkRef) []linkRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]linkRef, len(refs))
	for i, r := range refs {
		out[i] = linkRef{Slug: r.Slug, Title: r.Title, Type: r.Type}
	}
	return out
}

// BuildData renders the store to the JSON payload the viewer reads.
func BuildData(s *core.Store) ([]byte, error) {
	entries, err := s.ListNodes(core.NodeTypes...)
	if err != nil {
		return nil, err
	}
	data := siteData{
		Site: siteMeta{
			Title:       s.Config.Title,
			Description: s.Config.Description,
			Accent:      s.Config.Accent,
		},
	}
	known := core.KnownSlugs(entries)
	renderer := core.NewRenderer(func(slug string) bool { return known[slug] })
	graph := core.BuildLinkGraph(entries)
	// Best-effort: outside a git repo (or a static-build sandbox), path-style
	// specs just never resolve — they still render, as plain code text.
	repoRoot, _ := s.RepoRoot()
	for _, e := range entries {
		htmlBody, err := renderer.Render(e.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Slug, err)
		}
		// Epic status is served computed, not stored: the stored field is a
		// cache written at stamp time and drifts between stamps, and a view
		// can't drift. Rollup stays alongside it so a client can still tell
		// the two apart.
		var rollup string
		if e.Kind() == "epic" {
			rollup = core.EpicRollupStatus(entries, e.Slug)
		}
		status := core.EpicDisplayStatus(entries, e)
		// A node ref resolves to its canonical slug (a filename-shaped spec
		// value normalizes the same way a [[wikilink]] does) so the client's
		// plain bySlug lookup finds it; anything else is left as-is and the
		// client renders it as plain code text — it cannot serve arbitrary
		// repo files, and must not try.
		spec := e.FM.Spec
		if kind, slug := core.ResolveSpec(entries, repoRoot, spec); kind == "node" {
			spec = slug
		}
		data.Entries = append(data.Entries, entryData{
			Slug:     e.Slug,
			Title:    e.FM.Title,
			Date:     e.FM.Date,
			Time:     e.FM.Time,
			Tags:     e.FM.Tags,
			Hashes:   e.AllHashes(),
			Stat:     e.FM.Stat,
			Summary:  e.FM.Summary,
			Cover:    e.FM.Cover,
			Draft:    e.IsDraft(),
			Featured: e.FM.Featured,
			HTML:     htmlBody,

			Type:         e.Kind(),
			Status:       status,
			Rollup:       rollup,
			Spec:         spec,
			Effort:       e.FM.Effort,
			Horizon:      e.FM.Horizon,
			Epic:         e.FM.Epic,
			Entry:        e.FM.Entry,
			Supersedes:   e.FM.Supersedes,
			SupersededBy: e.FM.SupersededBy,

			Links:     toLinkRefs(graph.Out[e.Slug]),
			Backlinks: toLinkRefs(graph.Back[e.Slug]),
		})
	}
	return json.MarshalIndent(data, "", "  ")
}

// Asset returns an embedded viewer asset by name (index.html, app.js, styles.css).
func Asset(name string) ([]byte, error) {
	return assets.ReadFile("assets/" + name)
}

// Build writes a complete static site into outDir: the viewer assets, the
// generated data.json, and a copy of the media directory.
func Build(s *core.Store, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"index.html", "app.js", "styles.css"} {
		b, err := Asset(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outDir, name), b, 0o644); err != nil {
			return err
		}
	}
	data, err := BuildData(s)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "data.json"), data, 0o644); err != nil {
		return err
	}
	return copyTree(s.MediaDir(), filepath.Join(outDir, "media"))
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
