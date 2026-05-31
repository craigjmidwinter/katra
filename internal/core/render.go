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

var md = newMarkdown()

func newMarkdown() goldmark.Markdown {
	m := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&componentRenderer{}, 1),
	))
	return m
}

// RenderMarkdown converts an entry body (markdown + components) to HTML.
func RenderMarkdown(src string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderPlain is used by components that contain nested markdown (note/warning).
// It deliberately uses a fresh, component-free converter to avoid recursion.
var plainMD = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

func renderPlain(src string) string {
	var buf bytes.Buffer
	if err := plainMD.Convert([]byte(strings.TrimSpace(src)), &buf); err != nil {
		return html.EscapeString(src)
	}
	return buf.String()
}

// componentRenderer overrides fenced-code-block rendering: registered languages
// become components, everything else falls back to a plain <pre><code>.
type componentRenderer struct{}

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
		out, err := fn(body)
		if err != nil {
			out = "<!-- devlog component error (" + html.EscapeString(lang) + "): " + html.EscapeString(err.Error()) + " -->"
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
