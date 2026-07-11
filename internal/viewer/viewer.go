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

	"github.com/craigjmidwinter/devlog/internal/core"
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
}

// BuildData renders the store to the JSON payload the viewer reads.
func BuildData(s *core.Store) ([]byte, error) {
	entries, err := s.List()
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
	for _, e := range entries {
		htmlBody, err := core.RenderMarkdown(e.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Slug, err)
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
