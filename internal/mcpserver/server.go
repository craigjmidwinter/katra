// Package mcpserver exposes the katra core operations as MCP tools so any
// agent or tooling can drive the dev log structurally (not just by shelling out
// to the CLI). It resolves the katra from $KATRA_DIR (or legacy $DEVLOG_DIR),
// else the process cwd.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Run starts the MCP server over stdio.
func Run(version string) error {
	return server.ServeStdio(newServer(version))
}

// newServer keeps construction separate from transport so the protocol
// surface can be tested without spawning a subprocess. The registry package
// and the native binary both call Run, so they still use this exact server.
func newServer(version string) *server.MCPServer {
	s := server.NewMCPServer("katra", version)

	s.AddTool(mcp.NewTool("katra_list",
		mcp.WithDescription("List dev log entries (newest first). Returns slug, title, date, draft status, hashes and tags."),
		mcp.WithBoolean("drafts_only", mcp.Description("Only return unstamped drafts.")),
	), handleList)

	s.AddTool(mcp.NewTool("katra_get",
		mcp.WithDescription("Get one entry's frontmatter and raw markdown body by slug."),
		mcp.WithString("slug", mcp.Required(), mcp.Description("The entry slug.")),
	), handleGet)

	s.AddTool(mcp.NewTool("katra_new",
		mcp.WithDescription("Create a new draft entry. Returns its slug. Write the *why*, not a paraphrased diff."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Entry title.")),
		mcp.WithString("body", mcp.Description("Initial markdown body.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags.")),
		mcp.WithString("summary", mcp.Description("One-line summary for the index.")),
		mcp.WithBoolean("featured", mcp.Description("Mark as a Deep Dive (long read).")),
	), handleNew)

	s.AddTool(mcp.NewTool("katra_append",
		mcp.WithDescription("Append markdown to a draft. Defaults to the active draft if slug is omitted. Rich components are fenced code blocks: ```embed, ```compare, ```gallery, ```video, ```note, ```warning."),
		mcp.WithString("markdown", mcp.Required(), mcp.Description("Markdown to append.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
	), handleAppend)

	s.AddTool(mcp.NewTool("katra_capture",
		mcp.WithDescription("Import a media file (image/gif/video/html) into the dev log and append it to a draft. Use this for screenshots, animation gifs, and interactive html artifacts."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to the media file to import.")),
		mcp.WithString("caption", mcp.Description("Caption for the media.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
		mcp.WithString("as", mcp.Description("Force kind: image|video|embed.")),
	), handleCapture)

	s.AddTool(mcp.NewTool("katra_compare",
		mcp.WithDescription("Import two images and append a before/after comparison slider to a draft."),
		mcp.WithString("before", mcp.Required(), mcp.Description("Path to the 'before' image.")),
		mcp.WithString("after", mcp.Required(), mcp.Description("Path to the 'after' image.")),
		mcp.WithString("caption", mcp.Description("Caption for the comparison.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
	), handleCompare)

	s.AddTool(mcp.NewTool("katra_stamp",
		mcp.WithDescription("Stamp a draft with its commit hash(es) + computed diffstat, moving it from In Progress into the log. Defaults to HEAD and the active draft."),
		mcp.WithString("hashes", mcp.Description("Comma-separated commit hash(es). Default: HEAD.")),
		mcp.WithString("slug", mcp.Description("Target entry slug (default: active draft).")),
	), handleStamp)

	// --- Katra node-model tools -----------------------------------------

	s.AddTool(mcp.NewTool("katra_nodes",
		mcp.WithDescription("List nodes of a given type (newest first). Returns slug, title, type, date, status and other node-model fields. Omit type to list every node type."),
		mcp.WithString("type", mcp.Description("Node type to list: entry|task|epic|decision|article. Empty = all types.")),
	), handleNodes)

	s.AddTool(mcp.NewTool("katra_task_new",
		mcp.WithDescription("Create a new task node (status defaults to todo, or specced when spec is given). Returns its slug."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Task title.")),
		mcp.WithString("body", mcp.Description("Initial markdown body.")),
		mcp.WithString("effort", mcp.Description("Effort estimate: S|M|L.")),
		mcp.WithString("epic", mcp.Description("Parent epic slug.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags.")),
		mcp.WithString("spec", mcp.Description("Spec artifact ref (a node slug in the katra, or a path relative to the repo root); creates the task already specced.")),
	), handleTaskNew)

	s.AddTool(mcp.NewTool("katra_task_list",
		mcp.WithDescription("List task nodes (newest first). Optionally filter by status."),
		mcp.WithString("status", mcp.Description("Filter to this status: todo|specced|doing|done|cut.")),
	), handleTaskList)

	s.AddTool(mcp.NewTool("katra_task_set_status",
		mcp.WithDescription("Set a task's status (todo|specced|doing|done|cut)."),
		mcp.WithString("slug", mcp.Required(), mcp.Description("Task slug.")),
		mcp.WithString("status", mcp.Required(), mcp.Description("New status: todo|specced|doing|done|cut.")),
	), handleTaskSetStatus)

	s.AddTool(mcp.NewTool("katra_task_spec",
		mcp.WithDescription("Attach a spec artifact to a task. Advances todo/empty status to specced; leaves doing/done/cut alone (setting spec never moves status backwards). A ref that resolves to neither a node nor a file warns but still writes — `katra doctor` is the blocking check, not this."),
		mcp.WithString("slug", mcp.Required(), mcp.Description("Task slug.")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("Spec ref: a node slug in the katra, or a path relative to the repo root.")),
	), handleTaskSpec)

	s.AddTool(mcp.NewTool("katra_epic_new",
		mcp.WithDescription("Create a new epic node. Returns its slug."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Epic title.")),
		mcp.WithString("body", mcp.Description("Initial markdown body.")),
		mcp.WithString("horizon", mcp.Description("Planning horizon: now|next|later.")),
	), handleEpicNew)

	s.AddTool(mcp.NewTool("katra_decide",
		mcp.WithDescription("Create a new decision (ADR) node. Returns its slug."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Decision title.")),
		mcp.WithString("body", mcp.Description("Initial markdown body.")),
		mcp.WithString("supersedes", mcp.Description("Comma-separated decision slug(s) this one supersedes.")),
		mcp.WithString("entry", mcp.Description("The entry slug that recorded/occasioned this decision.")),
	), handleDecide)

	s.AddTool(mcp.NewTool("katra_article_new",
		mcp.WithDescription("Create a new article (evergreen reference) node. Returns its slug."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Article title.")),
		mcp.WithString("body", mcp.Description("Initial markdown body.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags.")),
	), handleArticleNew)

	return s
}

func store() (*core.Store, error) {
	start := core.EnvDir()
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
		Time   string   `json:"time,omitempty"`
		Draft  bool     `json:"draft"`
		Hashes []string `json:"hashes,omitempty"`
		Tags   []string `json:"tags,omitempty"`
	}
	var rows []row
	for _, e := range entries {
		if draftsOnly && !e.IsDraft() {
			continue
		}
		rows = append(rows, row{e.Slug, e.FM.Title, e.FM.Date, e.FM.Time, e.IsDraft(), e.AllHashes(), e.FM.Tags})
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
		"time":     e.FM.Time,
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
		body = core.DraftPlaceholderBody
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
	// Route through PublishEntry so a stamp over MCP applies the same task-close
	// + epic-rollup follow-ups as the CLI and post-commit hook (§ fix #12).
	res, err := s.PublishEntry(e, hashes)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("stamped %s → %v", e.Slug, e.AllHashes())
	if len(res.Closed) > 0 {
		msg += fmt.Sprintf("; closed %v", res.Closed)
	}
	if len(res.Epics) > 0 {
		msg += fmt.Sprintf("; rolled up %v", res.Epics)
	}
	return mcp.NewToolResultText(msg), nil
}

// nodeRow is the JSON shape returned by node listing tools.
type nodeRow struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
	Date    string   `json:"date"`
	Time    string   `json:"time,omitempty"`
	Status  string   `json:"status,omitempty"`
	Spec    string   `json:"spec,omitempty"`
	Effort  string   `json:"effort,omitempty"`
	Horizon string   `json:"horizon,omitempty"`
	Epic    string   `json:"epic,omitempty"`
	Entry   string   `json:"entry,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

func toNodeRow(e core.Entry) nodeRow {
	return nodeRow{
		Slug:    e.Slug,
		Title:   e.FM.Title,
		Type:    e.Kind(),
		Date:    e.FM.Date,
		Time:    e.FM.Time,
		Status:  e.FM.Status,
		Spec:    e.FM.Spec,
		Effort:  e.FM.Effort,
		Horizon: e.FM.Horizon,
		Epic:    e.FM.Epic,
		Entry:   e.FM.Entry,
		Tags:    e.FM.Tags,
	}
}

func handleNodes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var nodes []core.Entry
	if t := argString(req, "type"); t != "" {
		nodes, err = s.ListNodes(t)
	} else {
		nodes, err = s.ListNodes()
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rows := make([]nodeRow, 0, len(nodes))
	for _, e := range nodes {
		rows = append(rows, toNodeRow(e))
	}
	return jsonResult(rows)
}

func handleTaskNew(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	spec := argString(req, "spec")
	status := "todo"
	if spec != "" {
		status = "specced"
	}
	e, err := s.NewNode("task", core.Frontmatter{
		Title:  argString(req, "title"),
		Status: status,
		Spec:   spec,
		Effort: argString(req, "effort"),
		Epic:   argString(req, "epic"),
		Tags:   splitCSV(argString(req, "tags")),
	}, argString(req, "body"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("created task %q (%s)", e.Slug, e.Path)), nil
}

// handleTaskSpec mirrors `katra task spec`: it attaches a spec ref to a task,
// advancing todo/empty status to specced (setting spec never moves status
// backwards), and warns — but still writes — when the ref resolves to neither
// a node nor a file. `katra doctor` is the blocking check, not this.
func handleTaskSpec(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ref := argString(req, "ref")
	if ref == "" {
		return mcp.NewToolResultError("ref is required"), nil
	}
	e, err := s.GetNode(argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e.FM.Spec = ref
	if e.FM.Status == "" || e.FM.Status == "todo" {
		e.FM.Status = "specced"
	}
	if err := e.Save(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("set %s spec → %s (status: %s)", e.Slug, ref, e.FM.Status)
	nodes, _ := s.ListNodes()
	root, _ := s.RepoRoot()
	if kind, _ := core.ResolveSpec(nodes, root, ref); kind == "" {
		msg += "; warning: ref resolves to neither a node nor a file yet — `katra doctor` will flag it until it does"
	}
	return mcp.NewToolResultText(msg), nil
}

func handleTaskList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	nodes, err := s.ListNodes("task")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	status := argString(req, "status")
	rows := make([]nodeRow, 0, len(nodes))
	for _, e := range nodes {
		if status != "" && e.FM.Status != status {
			continue
		}
		rows = append(rows, toNodeRow(e))
	}
	return jsonResult(rows)
}

func handleTaskSetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	status := argString(req, "status")
	if status == "" {
		return mcp.NewToolResultError("status is required"), nil
	}
	e, err := s.GetNode(argString(req, "slug"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e.FM.Status = status
	if err := e.Save(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("set %s status → %s", e.Slug, status)), nil
}

func handleEpicNew(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := s.NewNode("epic", core.Frontmatter{
		Title:   argString(req, "title"),
		Horizon: argString(req, "horizon"),
	}, argString(req, "body"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("created epic %q (%s)", e.Slug, e.Path)), nil
}

func handleDecide(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := s.NewNode("decision", core.Frontmatter{
		Title:      argString(req, "title"),
		Supersedes: splitCSV(argString(req, "supersedes")),
		Entry:      argString(req, "entry"),
	}, argString(req, "body"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("created decision %q (%s)", e.Slug, e.Path)), nil
}

func handleArticleNew(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s, err := store()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	e, err := s.NewNode("article", core.Frontmatter{
		Title: argString(req, "title"),
		Tags:  splitCSV(argString(req, "tags")),
	}, argString(req, "body"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("created article %q (%s)", e.Slug, e.Path)), nil
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
		return nil, fmt.Errorf("no active draft; pass slug or create one with katra_new")
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
