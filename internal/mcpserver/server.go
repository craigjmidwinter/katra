// Package mcpserver exposes the devlog core operations as MCP tools so any
// agent or tooling can drive the dev log structurally (not just by shelling out
// to the CLI). It resolves the devlog from $DEVLOG_DIR or the process cwd.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/craigjmidwinter/devlog/internal/core"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Run starts the MCP server over stdio.
func Run(version string) error {
	s := server.NewMCPServer("devlog", version)

	s.AddTool(mcp.NewTool("devlog_list",
		mcp.WithDescription("List dev log entries (newest first). Returns slug, title, date, draft status, hashes and tags."),
		mcp.WithBoolean("drafts_only", mcp.Description("Only return unstamped drafts.")),
	), handleList)

	s.AddTool(mcp.NewTool("devlog_get",
		mcp.WithDescription("Get one entry's frontmatter and raw markdown body by slug."),
		mcp.WithString("slug", mcp.Required(), mcp.Description("The entry slug.")),
	), handleGet)

	s.AddTool(mcp.NewTool("devlog_new",
		mcp.WithDescription("Create a new draft entry. Returns its slug. Write the *why*, not a paraphrased diff."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Entry title.")),
		mcp.WithString("body", mcp.Description("Initial markdown body.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags.")),
		mcp.WithString("summary", mcp.Description("One-line summary for the index.")),
		mcp.WithBoolean("featured", mcp.Description("Mark as a Deep Dive (long read).")),
	), handleNew)

	s.AddTool(mcp.NewTool("devlog_append",
		mcp.WithDescription("Append markdown to a draft. Defaults to the active draft if slug is omitted. Rich components are fenced code blocks: ```embed, ```compare, ```gallery, ```video, ```note, ```warning."),
		mcp.WithString("markdown", mcp.Required(), mcp.Description("Markdown to append.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
	), handleAppend)

	s.AddTool(mcp.NewTool("devlog_capture",
		mcp.WithDescription("Import a media file (image/gif/video/html) into the dev log and append it to a draft. Use this for screenshots, animation gifs, and interactive html artifacts."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to the media file to import.")),
		mcp.WithString("caption", mcp.Description("Caption for the media.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
		mcp.WithString("as", mcp.Description("Force kind: image|video|embed.")),
	), handleCapture)

	s.AddTool(mcp.NewTool("devlog_compare",
		mcp.WithDescription("Import two images and append a before/after comparison slider to a draft."),
		mcp.WithString("before", mcp.Required(), mcp.Description("Path to the 'before' image.")),
		mcp.WithString("after", mcp.Required(), mcp.Description("Path to the 'after' image.")),
		mcp.WithString("caption", mcp.Description("Caption for the comparison.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
	), handleCompare)

	s.AddTool(mcp.NewTool("devlog_stamp",
		mcp.WithDescription("Stamp a draft with its commit hash(es) + computed diffstat, moving it from In Progress into the log. Defaults to HEAD and the active draft."),
		mcp.WithString("hashes", mcp.Description("Comma-separated commit hash(es). Default: HEAD.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
	), handleStamp)

	return server.ServeStdio(s)
}

func store() (*core.Store, error) {
	start := os.Getenv("DEVLOG_DIR")
	if start == "" {
		start, _ = os.Getwd()
	}
	return core.FindStore(start)
}

func argString(req mcp.CallToolRequest, key string) string {
	if v, ok := req.Params.Arguments[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func argBool(req mcp.CallToolRequest, key string) bool {
	if v, ok := req.Params.Arguments[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	entries, err := s.List()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	draftsOnly := argBool(req, "drafts_only")
	type row struct {
		Slug   string   `json:"slug"`
		Title  string   `json:"title"`
		Date   string   `json:"date"`
		Draft  bool     `json:"draft"`
		Hashes []string `json:"hashes,omitempty"`
		Tags   []string `json:"tags,omitempty"`
	}
	var rows []row
	for _, e := range entries {
		if draftsOnly && !e.IsDraft() {
			continue
		}
		rows = append(rows, row{e.Slug, e.FM.Title, e.FM.Date, e.IsDraft(), e.AllHashes(), e.FM.Tags})
	}
	return jsonResult(rows)
}

func handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := s.Get(argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"slug":     e.Slug,
		"title":    e.FM.Title,
		"date":     e.FM.Date,
		"tags":     e.FM.Tags,
		"draft":    e.IsDraft(),
		"hashes":   e.AllHashes(),
		"featured": e.FM.Featured,
		"body":     e.Body,
	})
}

func handleNew(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	body := argString(req, "body")
	if body == "" {
		body = "Start writing here.\n"
	}
	e, err := s.NewEntry(core.Frontmatter{
		Title:    argString(req, "title"),
		Tags:     splitCSV(argString(req, "tags")),
		Summary:  argString(req, "summary"),
		Featured: argBool(req, "featured"),
	}, body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("created draft %q (%s)", e.Slug, e.Path)), nil
}

func handleAppend(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := resolveEntry(s, argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.AppendBody(e, argString(req, "markdown")); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("appended to " + e.Slug), nil
}

func handleCapture(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ref, err := s.ImportMedia(argString(req, "path"), "")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	kind := core.KindFor(ref)
	if as := argString(req, "as"); as != "" {
		kind = core.MediaKind(as)
	}
	e, err := resolveEntry(s, argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("imported %s (not appended: %v)", ref, err)), nil
	}
	if err := s.AppendBody(e, core.MediaBlock(ref, argString(req, "caption"), kind)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("imported %s and added to %s", ref, e.Slug)), nil
}

func handleCompare(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	before, err := s.ImportMedia(argString(req, "before"), "")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	after, err := s.ImportMedia(argString(req, "after"), "")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := resolveEntry(s, argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.AppendBody(e, core.CompareBlock(before, after, argString(req, "caption"))); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("added compare to " + e.Slug), nil
}

func handleStamp(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := resolveEntry(s, argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	hashes := splitCSV(argString(req, "hashes"))
	if len(hashes) == 0 {
		h, err := s.HeadHash()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hashes = []string{h}
	}
	if err := s.Stamp(e, hashes); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("stamped %s → %v", e.Slug, e.AllHashes())), nil
}

func resolveEntry(s *core.Store, slug string) (*core.Entry, error) {
	if slug != "" {
		return s.Get(slug)
	}
	e, err := s.ActiveDraft()
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("no active draft; pass slug or create one with devlog_new")
	}
	return e, nil
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
