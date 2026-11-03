package core

import (
	"bytes"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ComponentFunc renders a fenced block of a registered language to HTML.
// body is the raw text between the fences.
type ComponentFunc func(body string) (string, error)

// Registry maps a fenced-code language to its component renderer. Authors embed
// rich components in plain markdown by fencing them with a registered language:
//
//	```compare
//	before: media/a.png
//	after:  media/b.png
//	```
//
// Unregistered languages render as ordinary code blocks.
var Registry = map[string]ComponentFunc{
	"embed":   renderEmbed,
	"compare": renderCompare,
	"gallery": renderGallery,
	"video":   renderVideo,
	"note":    renderNote,
	"warning": renderWarning,
}

var md = newMarkdownWith(nil, false)

// rendererOptions is where raw HTML is decided. Without WithUnsafe goldmark
// omits raw HTML and refuses dangerous link schemes, which is the whole of
// rawHTML: sanitize. Components are unaffected, being katra's own output.
func rendererOptions(sanitize bool) []goldmark.Option {
	if sanitize {
		return nil
	}
	return []goldmark.Option{goldmark.WithRendererOptions(gmhtml.WithUnsafe())}
}

// newMarkdownWith builds a converter. When resolver is non-nil, [[wikilinks]]
// whose target slug fails resolver() render with the dl-wikilink-missing class;
// a nil resolver renders every wikilink as resolved (we don't know otherwise).
// sanitize drops raw HTML, in callouts as well as in the body.
func newMarkdownWith(resolver func(slug string) bool, sanitize bool) goldmark.Markdown {
	opts := append([]goldmark.Option{
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	}, rendererOptions(sanitize)...)
	m := goldmark.New(opts...)
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&wikilinkParser{resolver: resolver}, 1),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&componentRenderer{sanitize: sanitize}, 1),
		util.Prioritized(&wikilinkRenderer{}, 1),
	))
	return m
}

// RenderMarkdown converts an entry body (markdown + components) to HTML.
// Wikilinks render as anchors but are not resolved against a node set; use a
// Renderer from NewRenderer to mark missing links.
func RenderMarkdown(src string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Renderer is a converter bound to a wikilink resolver, so [[links]] to unknown
// nodes get the dl-wikilink-missing class. Build one per data build and reuse it
// across every node's body.
type Renderer struct{ md goldmark.Markdown }

// NewRenderer returns a Renderer whose wikilinks are resolved against resolver
// (typically membership in the set of known node slugs). sanitize strips raw
// HTML; pass Config.SanitizeRawHTML.
func NewRenderer(resolver func(slug string) bool, sanitize bool) *Renderer {
	return &Renderer{md: newMarkdownWith(resolver, sanitize)}
}

// Render converts a markdown body to HTML with resolved wikilinks.
func (r *Renderer) Render(src string) (string, error) {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- wikilinks: [[slug]] and [[slug|label]] -------------------------------

var kindWikilink = ast.NewNodeKind("Wikilink")

type wikilinkNode struct {
	ast.BaseInline
	Slug     string // normalized target node slug
	Label    string // display text (defaults to Slug)
	Resolved bool   // whether the target is a known node
}

func (n *wikilinkNode) Kind() ast.NodeKind { return kindWikilink }
func (n *wikilinkNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// wikilinkParser turns [[slug]] / [[slug|label]] into wikilinkNode. It triggers
// on '[' and returns nil for a lone '[' so ordinary [text](url) links still
// parse. Running inline, it never fires inside code spans or fenced blocks.
type wikilinkParser struct{ resolver func(slug string) bool }

func (p *wikilinkParser) Trigger() []byte { return []byte{'['} }

func (p *wikilinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 5 || line[0] != '[' || line[1] != '[' {
		return nil
	}
	end := bytes.Index(line, []byte("]]"))
	if end < 3 { // need at least one char between [[ and ]]
		return nil
	}
	inner := line[2:end]
	if bytes.IndexByte(inner, '\n') >= 0 || bytes.Contains(inner, []byte("[[")) {
		return nil
	}
	rawSlug := string(inner)
	label := ""
	if i := strings.IndexByte(rawSlug, '|'); i >= 0 {
		label = strings.TrimSpace(rawSlug[i+1:])
		rawSlug = rawSlug[:i]
	}
	slug := normalizeRef(rawSlug)
	if slug == "" {
		return nil
	}
	block.Advance(end + 2)
	resolved := true
	if p.resolver != nil {
		resolved = p.resolver(slug)
	}
	return &wikilinkNode{Slug: slug, Label: label, Resolved: resolved}
}

type wikilinkRenderer struct{}

func (r *wikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikilink, r.render)
}

func (r *wikilinkRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*wikilinkNode)
	class := "dl-wikilink"
	if !n.Resolved {
		class = "dl-wikilink dl-wikilink-missing"
	}
	label := n.Label
	if label == "" {
		label = n.Slug
	}
	_, _ = w.WriteString(`<a href="#` + html.EscapeString(n.Slug) + `" class="` + class + `">`)
	_, _ = w.WriteString(html.EscapeString(label))
	_, _ = w.WriteString("</a>")
	return ast.WalkSkipChildren, nil
}

// renderPlain is used by components that contain nested markdown (note/warning).
// It deliberately uses a fresh, component-free converter to avoid recursion.
var (
	plainMD     = goldmark.New(append([]goldmark.Option{goldmark.WithExtensions(extension.GFM)}, rendererOptions(false)...)...)
	safePlainMD = goldmark.New(append([]goldmark.Option{goldmark.WithExtensions(extension.GFM)}, rendererOptions(true)...)...)
)

func renderPlain(src string, conv goldmark.Markdown) string {
	var buf bytes.Buffer
	if err := conv.Convert([]byte(strings.TrimSpace(src)), &buf); err != nil {
		return html.EscapeString(src)
	}
	return buf.String()
}

// componentRenderer overrides fenced-code-block rendering: registered languages
// become components, everything else falls back to a plain <pre><code>.
type componentRenderer struct{ sanitize bool }

func (r *componentRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFenced)
}

func (r *componentRenderer) renderFenced(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	lang := codeLang(n, source)
	body := codeText(n, source)

	if fn, ok := Registry[lang]; ok {
		// Callouts are the one component whose body is author markdown, so
		// they are the one a sanitizing renderer has to intercept.
		if r.sanitize && (lang == "note" || lang == "warning") {
			_, _ = w.WriteString(calloutHTML(lang, body, safePlainMD))
			return ast.WalkSkipChildren, nil
		}
		out, err := fn(body)
		if err != nil {
			out = "<!-- katra component error (" + html.EscapeString(lang) + "): " + html.EscapeString(err.Error()) + " -->"
		}
		_, _ = w.WriteString(out)
		return ast.WalkSkipChildren, nil
	}

	// default code block
	_, _ = w.WriteString("<pre class=\"dl-code\"><code")
	if lang != "" {
		_, _ = w.WriteString(" class=\"language-" + html.EscapeString(lang) + "\"")
	}
	_, _ = w.WriteString(">")
	_, _ = w.WriteString(html.EscapeString(body))
	_, _ = w.WriteString("</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

func codeLang(n *ast.FencedCodeBlock, source []byte) string {
	if n.Info == nil {
		return ""
	}
	info := string(n.Info.Segment.Value(source))
	fields := strings.Fields(info)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func codeText(n ast.Node, source []byte) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(source))
	}
	return sb.String()
}
