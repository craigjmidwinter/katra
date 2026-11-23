package core

import (
	"os"
	"strings"
	"testing"
)

func TestBlockUnblockWait(t *testing.T) {
	s := newTestStore(t)
	n, err := s.NewNode("task", Frontmatter{Title: "Ship It", Status: "doing"}, "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.BlockTask(n.Slug, nil); err == nil || !strings.Contains(err.Error(), "--by") {
		t.Fatalf("block with no blocker: %v, want a refusal naming --by", err)
	}
	if got, _ := s.GetNode(n.Slug); got.FM.Status != "doing" {
		t.Fatalf("refused block still wrote: status %q", got.FM.Status)
	}

	b, err := s.BlockTask(n.Slug, []string{"other-task", " craig picks the licence ", "other-task"})
	if err != nil {
		t.Fatal(err)
	}
	if b.FM.Status != "blocked" || strings.Join(b.FM.BlockedBy, "|") != "other-task|craig picks the licence" {
		t.Fatalf("after block: status %q blocked_by %q", b.FM.Status, b.FM.BlockedBy)
	}
	b, _ = s.BlockTask(n.Slug, []string{"third"})
	if len(b.FM.BlockedBy) != 3 {
		t.Errorf("a second block replaced the blockers instead of adding: %q", b.FM.BlockedBy)
	}

	u, err := s.UnblockTask(n.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if u.FM.Status != "todo" || u.FM.BlockedBy != nil {
		t.Errorf("after unblock: status %q blocked_by %q, want todo and none", u.FM.Status, u.FM.BlockedBy)
	}
	if _, err := s.UnblockTask(n.Slug); err == nil {
		t.Error("unblocking a todo task should be refused")
	}

	if _, err := s.WaitOnOwner(n.Slug, ""); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("wait with no owner: %v, want a refusal", err)
	}
	w, err := s.WaitOnOwner(n.Slug, "role:steward")
	if err != nil || w.FM.Status != "waiting_on_owner" || w.FM.Owner != "role:steward" {
		t.Fatalf("wait: %+v %v", w.FM, err)
	}

	sp, _ := s.NewNode("task", Frontmatter{Title: "Specced One", Status: "specced", Spec: "docs/x.md"}, "")
	if _, err := s.BlockTask(sp.Slug, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.UnblockTask(sp.Slug); u.FM.Status != "specced" {
		t.Errorf("unblocking a task with a spec gave %q, want specced", u.FM.Status)
	}
}

// TestSetStatusRefusesOnlyWhatTheChangeIntroduces: an off-type status and a
// bare blocked are refused, but a legacy node with an unrelated odd value can
// still change status.
func TestSetStatusRefusesOnlyWhatTheChangeIntroduces(t *testing.T) {
	s := newTestStore(t)
	n, _ := s.NewNode("task", Frontmatter{Title: "Odd", Status: "todo"}, "")
	for _, bad := range []string{"wip", "blocked", "active"} {
		if _, err := s.SetStatus(n.Slug, bad); err == nil {
			t.Errorf("SetStatus(%q) accepted", bad)
		}
	}
	// Hand-edit an off-scale effort onto it, as an older tool might have.
	b, _ := os.ReadFile(n.Path)
	if err := os.WriteFile(n.Path, []byte(strings.Replace(string(b), "status: todo", "status: todo\neffort: medium", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus(n.Slug, "doing"); err != nil {
		t.Errorf("legacy odd effort blocked an unrelated status change: %v", err)
	}
	if _, err := s.BlockTask("no-such-task", []string{"x"}); err == nil {
		t.Error("blocking a missing task should fail")
	}
}

func TestValidateNodeStatuses(t *testing.T) {
	for _, c := range []struct {
		fm   Frontmatter
		want string
	}{
		{Frontmatter{Type: "task", Status: "open"}, `not a task status`},
		{Frontmatter{Type: "epic", Status: "doing"}, `not a epic status`},
		{Frontmatter{Type: "task", Status: "blocked"}, "needs blocked_by"},
		{Frontmatter{Type: "task", Status: "blocked", BlockedBy: []string{" "}}, "needs blocked_by"},
		{Frontmatter{Type: "task", Status: "waiting_on_owner"}, "needs an owner"},
		{Frontmatter{Type: "task", Status: "blocked", BlockedBy: []string{"x"}}, ""},
		{Frontmatter{Type: "task", Status: "waiting_on_owner", Owner: "me"}, ""},
		{Frontmatter{Type: "article", Status: "whatever"}, ""},
	} {
		errs := ValidateNode(Entry{Slug: "n", FM: c.fm})
		joined := ""
		for _, e := range errs {
			joined += e.Error() + ";"
		}
		if (c.want == "") != (joined == "") || !strings.Contains(joined, c.want) {
			t.Errorf("%+v: got %q, want %q", c.fm, joined, c.want)
		}
	}
}
