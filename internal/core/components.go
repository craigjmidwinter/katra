package core

import (
	"fmt"
	"html"
	"strings"

	"gopkg.in/yaml.v3"
)

// The built-in component renderers. Each takes the raw fenced-block body and
// returns an HTML fragment. The viewer's styles.css + app.js give them
// behaviour (lightbox, compare slider). Add a component by registering a new
// ComponentFunc in render.go's Registry — that's the whole extension surface.

type embedSpec struct {
	Src     string `yaml:"src"`
	Height  int    `yaml:"height"`
	Caption string `yaml:"caption"`
}

func renderEmbed(body string) (string, error) {
	var s embedSpec
	if err := yaml.Unmarshal([]byte(body), &s); err != nil {
		return "", err
	}
	if s.Src == "" {
		return "", fmt.Errorf("embed requires a src")
	}
	if s.Height == 0 {
		s.Height = 480
	}
	var b strings.Builder
	b.WriteString(`<figure class="dl-embed">`)
	fmt.Fprintf(&b, `<iframe src="%s" style="height:%dpx" loading="lazy" sandbox="allow-scripts allow-same-origin allow-pointer-lock"></iframe>`,
		attr(s.Src), s.Height)
	b.WriteString(figcap(s.Caption))
	b.WriteString(`</figure>`)
	return b.String(), nil
}

type compareSpec struct {
	Before  string `yaml:"before"`
	After   string `yaml:"after"`
	Caption string `yaml:"caption"`
}

func renderCompare(body string) (string, error) {
	var s compareSpec
	if err := yaml.Unmarshal([]byte(body), &s); err != nil {
		return "", err
	}
	if s.Before == "" || s.After == "" {
		return "", fmt.Errorf("compare requires before and after")
	}
	var b strings.Builder
	b.WriteString(`<figure class="dl-compare" data-dl-compare>`)
	b.WriteString(`<div class="dl-compare-stage">`)
	fmt.Fprintf(&b, `<img class="dl-compare-after" src="%s" alt="after">`, attr(s.After))
	fmt.Fprintf(&b, `<div class="dl-compare-before"><img src="%s" alt="before"></div>`, attr(s.Before))
	b.WriteString(`<div class="dl-compare-handle"></div>`)
	b.WriteString(`<input class="dl-compare-range" type="range" min="0" max="100" value="50" aria-label="compare">`)
	b.WriteString(`<span class="dl-compare-tag dl-compare-tag-before">before</span>`)
	b.WriteString(`<span class="dl-compare-tag dl-compare-tag-after">after</span>`)
	b.WriteString(`</div>`)
	b.WriteString(figcap(s.Caption))
	b.WriteString(`</figure>`)
	return b.String(), nil
}

type galleryItem struct {
	Src string `yaml:"src"`
	Cap string `yaml:"cap"`
}

func renderGallery(body string) (string, error) {
	var items []galleryItem
	if err := yaml.Unmarshal([]byte(body), &items); err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("gallery requires at least one item")
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="dl-gallery" data-count="%d">`, len(items))
	for _, it := range items {
		if it.Src == "" {
			continue
		}
		b.WriteString(`<figure class="dl-gallery-item">`)
		fmt.Fprintf(&b, `<img src="%s" alt="%s" loading="lazy" data-dl-lightbox>`, attr(it.Src), attr(it.Cap))
		b.WriteString(figcap(it.Cap))
		b.WriteString(`</figure>`)
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

type videoSpec struct {
	Src     string `yaml:"src"`
	Poster  string `yaml:"poster"`
	Caption string `yaml:"caption"`
	Loop    bool   `yaml:"loop"`
}

func renderVideo(body string) (string, error) {
	var s videoSpec
	if err := yaml.Unmarshal([]byte(body), &s); err != nil {
		return "", err
	}
	if s.Src == "" {
		return "", fmt.Errorf("video requires a src")
	}
	var b strings.Builder
	b.WriteString(`<figure class="dl-video">`)
	b.WriteString(`<video controls playsinline`)
	if s.Loop {
		b.WriteString(` loop muted`)
	}
	if s.Poster != "" {
		fmt.Fprintf(&b, ` poster="%s"`, attr(s.Poster))
	}
	fmt.Fprintf(&b, `><source src="%s"></video>`, attr(s.Src))
	b.WriteString(figcap(s.Caption))
	b.WriteString(`</figure>`)
	return b.String(), nil
}

func renderNote(body string) (string, error) {
	return calloutHTML("note", body), nil
}

func renderWarning(body string) (string, error) {
	return calloutHTML("warning", body), nil
}

func calloutHTML(kind, body string) string {
	return fmt.Sprintf(`<aside class="dl-callout dl-callout-%s">%s</aside>`, kind, renderPlain(body))
}

func figcap(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return `<figcaption>` + html.EscapeString(s) + `</figcaption>`
}

// attr escapes a string for use inside an HTML attribute value.
func attr(s string) string {
	return strings.NewReplacer(`&`, "&amp;", `"`, "&quot;", `<`, "&lt;", `>`, "&gt;").Replace(s)
}
