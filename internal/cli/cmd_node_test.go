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
