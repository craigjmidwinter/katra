package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// runNodeCmd executes the node subcommands against a store rooted at storeDir
// (via $DEVLOG_DIR) and returns captured stdout plus any error. The commands
// print with fmt.Printf (os.Stdout), so we redirect the real os.Stdout.
func runNodeCmd(t *testing.T, storeDir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("DEVLOG_DIR", storeDir)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	root := &cobra.Command{Use: "katra", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(taskCmd(), epicCmd(), decideCmd(), articleCmd())
	root.SetOut(w)
	root.SetErr(w)
	root.SetArgs(args)
	execErr := root.Execute()

	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), execErr
}

// TestNodeCommandsCreateTypedNodes drives `task/epic/decide/article new` and
// checks each writes to the right per-type dir with the right frontmatter.
func TestNodeCommandsCreateTypedNodes(t *testing.T) {
	store, err := core.InitStore(t.TempDir(), "CLI Test")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		args       []string
		wantSlug   string
		wantType   string
		wantStatus string
		dir        func(*core.Store) string
	}{
		{"task", []string{"task", "new", "Build the Thing", "--effort", "M", "--epic", "big-epic"},
			"build-the-thing", "task", "todo", (*core.Store).TasksDir},
		{"epic", []string{"epic", "new", "Big Epic", "--horizon", "now"},
			"big-epic", "epic", "planned", (*core.Store).EpicsDir},
		{"decision", []string{"decide", "Use Go", "--supersedes", "old-choice"},
			"use-go", "decision", "accepted", (*core.Store).DecisionsDir},
		{"article", []string{"article", "new", "Reference Doc"},
			"reference-doc", "article", "", (*core.Store).ArticlesDir},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := runNodeCmd(t, store.Dir, c.args...); err != nil {
				t.Fatalf("run %v: %v", c.args, err)
			}
			node, err := store.GetNode(c.wantSlug)
			if err != nil {
				t.Fatalf("GetNode(%q): %v", c.wantSlug, err)
			}
			if node.Kind() != c.wantType {
				t.Errorf("Kind() = %q, want %q", node.Kind(), c.wantType)
			}
			if node.FM.Status != c.wantStatus {
				t.Errorf("Status = %q, want %q", node.FM.Status, c.wantStatus)
			}
			if filepath.Dir(node.Path) != c.dir(store) {
				t.Errorf("dir = %s, want %s", filepath.Dir(node.Path), c.dir(store))
			}
		})
	}

	// Spot-check that structured edges landed from flags.
	task, _ := store.GetNode("build-the-thing")
	if task.FM.Effort != "M" || task.FM.Epic != "big-epic" {
		t.Errorf("task edges: effort=%q epic=%q, want M/big-epic", task.FM.Effort, task.FM.Epic)
	}
	dec, _ := store.GetNode("use-go")
	if len(dec.FM.Supersedes) != 1 || dec.FM.Supersedes[0] != "old-choice" {
		t.Errorf("decision supersedes = %v, want [old-choice]", dec.FM.Supersedes)
	}
}

// TestTaskStatusTransitions drives `task start` / `task done`, asserting the
// status field round-trips to disk.
func TestTaskStatusTransitions(t *testing.T) {
	store, err := core.InitStore(t.TempDir(), "CLI Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Movable Task"); err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		args []string
		want string
	}{
		{[]string{"task", "start", "movable-task"}, "doing"},
		{[]string{"task", "done", "movable-task"}, "done"},
	}
	for _, st := range steps {
		if _, err := runNodeCmd(t, store.Dir, st.args...); err != nil {
			t.Fatalf("run %v: %v", st.args, err)
		}
		node, err := store.GetNode("movable-task")
		if err != nil {
			t.Fatal(err)
		}
		if node.FM.Status != st.want {
			t.Errorf("after %v, status = %q, want %q", st.args, node.FM.Status, st.want)
		}
	}

	// Unknown slug should error.
	if _, err := runNodeCmd(t, store.Dir, "task", "done", "no-such-task"); err == nil {
		t.Error("task done on missing slug: want error, got nil")
	}
}

// TestTaskListFiltersByStatus verifies `task list --status` filters output and
// only lists tasks (not other node types).
func TestTaskListFiltersByStatus(t *testing.T) {
	store, err := core.InitStore(t.TempDir(), "CLI Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Todo One"); err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Done One"); err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "done", "done-one"); err != nil {
		t.Fatal(err)
	}
	// An epic must not surface in `task list`.
	if _, err := runNodeCmd(t, store.Dir, "epic", "new", "Some Epic"); err != nil {
		t.Fatal(err)
	}

	out, err := runNodeCmd(t, store.Dir, "task", "list", "--status", "done")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte("done-one")) {
		t.Errorf("filtered list missing done-one:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte("todo-one")) {
		t.Errorf("filtered list should not contain todo-one:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte("some-epic")) {
		t.Errorf("task list leaked an epic:\n%s", out)
	}
}

// TestTaskSpecCommand drives `task spec`: a todo task advances to specced, a
// doing task is left alone (setting spec never moves status backwards), and a
// ref that resolves to neither a node nor a file warns but still writes.
func TestTaskSpecCommand(t *testing.T) {
	store, err := core.InitStore(t.TempDir(), "CLI Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "decide", "Some Decision"); err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Needs A Spec"); err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Already Moving"); err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "start", "already-moving"); err != nil {
		t.Fatal(err)
	}

	// todo -> specced, with the ref resolving to the decision node.
	out, err := runNodeCmd(t, store.Dir, "task", "spec", "needs-a-spec", "some-decision")
	if err != nil {
		t.Fatalf("task spec: %v", err)
	}
	if bytes.Contains([]byte(out), []byte("⚠")) {
		t.Errorf("a ref that resolves should not warn:\n%s", out)
	}
	node, err := store.GetNode("needs-a-spec")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Spec != "some-decision" {
		t.Errorf("Spec = %q, want some-decision", node.FM.Spec)
	}
	if node.FM.Status != "specced" {
		t.Errorf("Status = %q, want specced", node.FM.Status)
	}

	// doing stays doing — setting spec never moves status backwards.
	if _, err := runNodeCmd(t, store.Dir, "task", "spec", "already-moving", "some-decision"); err != nil {
		t.Fatalf("task spec: %v", err)
	}
	node, err = store.GetNode("already-moving")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Status != "doing" {
		t.Errorf("Status = %q, want doing (unchanged)", node.FM.Status)
	}
	if node.FM.Spec != "some-decision" {
		t.Errorf("Spec = %q, want some-decision", node.FM.Spec)
	}

	// A ref that resolves to neither a node nor a file warns but still writes.
	out, err = runNodeCmd(t, store.Dir, "task", "spec", "needs-a-spec", "docs/design/does-not-exist.md")
	if err != nil {
		t.Fatalf("task spec with unresolved ref: want no error (warn but write), got %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("⚠")) {
		t.Errorf("expected a warning for an unresolved ref:\n%s", out)
	}
	node, err = store.GetNode("needs-a-spec")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Spec != "docs/design/does-not-exist.md" {
		t.Errorf("Spec = %q, want the unresolved ref written anyway", node.FM.Spec)
	}
}

// TestTaskNewWithSpecFlag verifies `task new --spec` creates the task already
// specced with the ref recorded.
func TestTaskNewWithSpecFlag(t *testing.T) {
	store, err := core.InitStore(t.TempDir(), "CLI Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Pre-Specced", "--spec", "docs/design/foo.md"); err != nil {
		t.Fatal(err)
	}
	node, err := store.GetNode("pre-specced")
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

// TestTaskNewAcceptsHorizon closes an asymmetry that had a real cost: horizon
// is documented in docs/format.md as a *task* field, `epic new` exposed it, and
// `task new` did not — so the one prioritisation field the format has could not
// be set on the node type it is documented for.
//
// Found during a migration assessment: a project with an ordered blocker list
// had nowhere to put the ordering, and was told to keep its list rather than
// flatten it to fit the tool.
func TestTaskNewAcceptsHorizon(t *testing.T) {
	store, err := core.InitStore(t.TempDir(), "CLI Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runNodeCmd(t, store.Dir, "task", "new", "Ranked work", "--horizon", "next"); err != nil {
		t.Fatal(err)
	}

	node, err := store.GetNode("ranked-work")
	if err != nil {
		t.Fatal(err)
	}
	if node.FM.Horizon != "next" {
		t.Errorf("Horizon = %q, want next", node.FM.Horizon)
	}
}

// TestTaskAndEpicAgreeOnHorizon: the asymmetry was invisible because each
// command was correct on its own. Asserting they expose the same field is what
// makes a future divergence fail rather than sit unnoticed.
func TestTaskAndEpicAgreeOnHorizon(t *testing.T) {
	for _, kind := range []string{"task", "epic"} {
		cmd := taskCmd()
		if kind == "epic" {
			cmd = epicCmd()
		}
		var found bool
		for _, sub := range cmd.Commands() {
			if sub.Name() != "new" {
				continue
			}
			if sub.Flags().Lookup("horizon") != nil {
				found = true
			}
		}
		if !found {
			t.Errorf("`%s new` does not expose --horizon; the format documents horizon for tasks", kind)
		}
	}
}
