package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestServerToolInventory locks the wire-level tool list and the human
// reference together. Registry clients discover this surface through
// tools/list, so a handler that exists in code but is absent from the docs is
// a distribution defect rather than a cosmetic documentation miss.
func TestServerToolInventory(t *testing.T) {
	response := newServer("test").HandleMessage(context.Background(), []byte(`{
		"jsonrpc":"2.0","id":1,"method":"tools/list"
	}`))
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(envelope.Result.Tools))
	for _, tool := range envelope.Result.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	want := []string{
		"katra_append",
		"katra_article_new",
		"katra_capture",
		"katra_compare",
		"katra_decide",
		"katra_epic_new",
		"katra_get",
		"katra_list",
		"katra_new",
		"katra_nodes",
		"katra_stamp",
		"katra_task_list",
		"katra_task_new",
		"katra_task_set_status",
		"katra_task_spec",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tool inventory:\n got %v\nwant %v", got, want)
	}

	docs, err := os.ReadFile("../../docs/agents.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range want {
		if !strings.Contains(string(docs), "| `"+name+"` |") {
			t.Errorf("docs/agents.md does not list %s", name)
		}
	}
}

// newTestStore initializes a katra and points KATRA_DIR at it, mirroring how
// store() resolves in a real MCP session.
func newTestStore(t *testing.T) *core.Store {
	t.Helper()
	s, err := core.InitStore(t.TempDir(), "MCP Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	t.Setenv("KATRA_DIR", s.Dir)
	return s
}

func callReq(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("result has no content: %+v", res)
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("result content is not text: %+v", res.Content[0])
	}
	return tc.Text
}

// TestHandleTaskNewWithSpec verifies the spec arg creates the task already
// specced and carries the ref, mirroring `katra task new --spec`.
func TestHandleTaskNewWithSpec(t *testing.T) {
	s := newTestStore(t)
	res, err := handleTaskNew(context.Background(), callReq(map[string]any{
		"title": "Pre-Specced",
		"spec":  "docs/design/foo.md",
	}))
	if err != nil {
		t.Fatalf("handleTaskNew: %v", err)
	}
	if res.IsError {
		t.Fatalf("handleTaskNew errored: %s", resultText(t, res))
	}
	node, err := s.GetNode("pre-specced")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Status != "specced" {
		t.Errorf("Status = %q, want specced", node.FM.Status)
	}
	if node.FM.Spec != "docs/design/foo.md" {
		t.Errorf("Spec = %q, want docs/design/foo.md", node.FM.Spec)
	}
}

// TestHandleTaskSpec mirrors the CLI's task spec behavior over MCP: a todo
// task advances to specced, a doing task is left alone, and a ref that
// resolves to neither a node nor a file warns in the result text but still
// writes.
func TestHandleTaskSpec(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.NewNode("decision", core.Frontmatter{Title: "Some Decision"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Needs A Spec", Status: "todo"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Already Moving", Status: "doing"}, ""); err != nil {
		t.Fatal(err)
	}

	// todo -> specced.
	res, err := handleTaskSpec(context.Background(), callReq(map[string]any{
		"slug": "needs-a-spec", "ref": "some-decision",
	}))
	if err != nil {
		t.Fatalf("handleTaskSpec: %v", err)
	}
	if strings.Contains(resultText(t, res), "warning") {
		t.Errorf("a resolving ref should not warn: %s", resultText(t, res))
	}
	node, err := s.GetNode("needs-a-spec")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Status != "specced" || node.FM.Spec != "some-decision" {
		t.Errorf("node = %+v, want status specced, spec some-decision", node.FM)
	}

	// doing stays doing.
	if _, err := handleTaskSpec(context.Background(), callReq(map[string]any{
		"slug": "already-moving", "ref": "some-decision",
	})); err != nil {
		t.Fatalf("handleTaskSpec: %v", err)
	}
	node, err = s.GetNode("already-moving")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Status != "doing" {
		t.Errorf("Status = %q, want doing (unchanged)", node.FM.Status)
	}

	// An unresolved ref warns but still writes.
	res, err = handleTaskSpec(context.Background(), callReq(map[string]any{
		"slug": "needs-a-spec", "ref": "docs/design/does-not-exist.md",
	}))
	if err != nil {
		t.Fatalf("handleTaskSpec: %v", err)
	}
	if !strings.Contains(resultText(t, res), "warning") {
		t.Errorf("expected a warning for an unresolved ref: %s", resultText(t, res))
	}
	node, err = s.GetNode("needs-a-spec")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Spec != "docs/design/does-not-exist.md" {
		t.Errorf("Spec = %q, want the unresolved ref written anyway", node.FM.Spec)
	}

	// ref is required.
	res, err = handleTaskSpec(context.Background(), callReq(map[string]any{"slug": "needs-a-spec"}))
	if err != nil {
		t.Fatalf("handleTaskSpec: %v", err)
	}
	if !res.IsError {
		t.Error("missing ref: want an error result")
	}
}

// TestToNodeRowIncludesSpec locks the spec field on the JSON shape
// katra_nodes/katra_task_list return.
func TestToNodeRowIncludesSpec(t *testing.T) {
	e := core.Entry{Slug: "x", FM: core.Frontmatter{Title: "X", Type: "task", Spec: "docs/design/foo.md"}}
	row := toNodeRow(e)
	if row.Spec != "docs/design/foo.md" {
		t.Errorf("Spec = %q, want docs/design/foo.md", row.Spec)
	}
}
