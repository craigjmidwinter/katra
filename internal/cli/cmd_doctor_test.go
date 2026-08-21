package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// runDoctor executes `katra doctor` against storeDir and returns its stdout.
// doctor exits the process when it finds a *problem*, so every fixture here is
// deliberately problem-free: the checks under test are warnings.
func runDoctor(t *testing.T, storeDir string) string {
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
	root.AddCommand(doctorCmd())
	root.SetOut(w)
	root.SetErr(w)
	root.SetArgs([]string{"doctor"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v", err)
	}

	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

// stamped writes a published (non-draft) entry — doctor's visual check only
// looks at entries that have already landed.
func stamped(t *testing.T, s *core.Store, title, body string, cover string) {
	t.Helper()
	if _, err := s.NewEntry(core.Frontmatter{Title: title, Hash: "abc1234", Cover: cover}, body); err != nil {
		t.Fatalf("NewEntry(%q): %v", title, err)
	}
}

// TestDoctorReportsZeroVisualEntries covers the nudge: published entries with
// neither a cover nor a media reference are counted, coverage is reported, and
// the offenders are named.
func TestDoctorReportsZeroVisualEntries(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Doctor Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.MediaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// The referenced file must exist, or the reference is a *problem* and
	// doctor exits before reaching the warnings.
	write(t, filepath.Join(s.MediaDir(), "shot.png"), "not really a png")

	stamped(t, s, "Has An Image", "Look:\n\n![a shot](media/shot.png)\n", "")
	stamped(t, s, "Has A Cover", "Words only in the body.", "media/shot.png")
	stamped(t, s, "All Prose One", "Just prose.", "")
	stamped(t, s, "All Prose Two", "Also just prose.", "")
	// A draft is exempt: it hasn't shipped yet.
	if _, err := s.NewEntry(core.Frontmatter{Title: "Still Drafting"}, core.DraftPlaceholderBody); err != nil {
		t.Fatal(err)
	}

	out := runDoctor(t, s.Dir)
	if !strings.Contains(out, "no visual in 2 of 4 published entries (50% coverage)") {
		t.Errorf("missing zero-visual warning:\n%s", out)
	}
	for _, slug := range []string{"all-prose-one", "all-prose-two"} {
		if !strings.Contains(out, slug) {
			t.Errorf("offender %q not listed:\n%s", slug, out)
		}
	}
	if strings.Contains(out, "has-an-image") || strings.Contains(out, "has-a-cover") {
		t.Errorf("entry with a visual was listed as an offender:\n%s", out)
	}
	if strings.Contains(out, "still-drafting") {
		t.Errorf("draft counted as a published entry with no visual:\n%s", out)
	}
	// A warning, never an error.
	if !strings.Contains(out, "✓ no problems") {
		t.Errorf("zero-visual entries must not be a problem:\n%s", out)
	}
}

// TestDoctorSilentWhenEveryEntryHasAVisual: no finding, no line.
func TestDoctorSilentWhenEveryEntryHasAVisual(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Doctor Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.MediaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(s.MediaDir(), "shot.png"), "not really a png")
	stamped(t, s, "Illustrated", "![a shot](media/shot.png)", "")

	if out := runDoctor(t, s.Dir); strings.Contains(out, "no visual") {
		t.Errorf("unexpected zero-visual warning:\n%s", out)
	}
}

// TestDoctorReportsEpicStatusDrift: the stored epic status is a stamp-time
// cache, so doctor is what notices it disagreeing with the child tasks.
func TestDoctorReportsEpicStatusDrift(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Doctor Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("epic", core.Frontmatter{Title: "Drifting Epic", Status: "planned"}, "body"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Started Task", Status: "doing", Epic: "drifting-epic"}, "body"); err != nil {
		t.Fatal(err)
	}
	// An epic whose cache is correct must stay quiet.
	if _, err := s.NewNode("epic", core.Frontmatter{Title: "Honest Epic", Status: "active"}, "body"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Other Task", Status: "doing", Epic: "honest-epic"}, "body"); err != nil {
		t.Fatal(err)
	}

	out := runDoctor(t, s.Dir)
	if !strings.Contains(out, "epic drifting-epic: stored planned, its tasks say active") {
		t.Errorf("missing epic drift warning:\n%s", out)
	}
	if strings.Contains(out, "honest-epic") {
		t.Errorf("epic in sync was reported as drifting:\n%s", out)
	}
	if !strings.Contains(out, "✓ no problems") {
		t.Errorf("drift must be a warning, not a problem:\n%s", out)
	}
}

// TestDoctorReportsNotAGitRepo: a store with no .git anywhere above it (git
// itself present) gets the plain "not a git repo" diagnosis.
func TestDoctorReportsNotAGitRepo(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Doctor Test")
	if err != nil {
		t.Fatal(err)
	}
	out := runDoctor(t, s.Dir)
	if !strings.Contains(out, "not a git repo — stamping/hook unavailable") {
		t.Errorf("missing not-a-git-repo warning:\n%s", out)
	}
}

// TestDoctorReportsGitNotFound covers Finding 5 from the ergonomics pass: a
// real git repo, but the `git` binary is missing from PATH, must not be
// misdiagnosed as "not a git repo" — that sends someone toward `git init`,
// which is wrong (and destructive-ish) advice for a repo that already exists.
func TestDoctorReportsGitNotFound(t *testing.T) {
	s, _ := cliGitStore(t)
	// PATH with no `git` on it — this repo is real, only the binary is gone.
	t.Setenv("PATH", t.TempDir())

	out := runDoctor(t, s.Dir)
	if !strings.Contains(out, "git not found on PATH — install git") {
		t.Errorf("missing git-not-found warning:\n%s", out)
	}
	if strings.Contains(out, "not a git repo") {
		t.Errorf("misdiagnosed missing git as \"not a git repo\":\n%s", out)
	}
}

// TestDoctorReportsDanglingSpec: a task's spec: ref that resolves to neither a
// node nor a file is what doctor is for — `katra task spec` itself only warns
// and writes, so this is where the dangling reference actually gets reported.
// A warning, like the other doctor nudges, never a problem (exit code).
func TestDoctorReportsDanglingSpec(t *testing.T) {
	s, err := core.InitStore(t.TempDir(), "Doctor Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("decision", core.Frontmatter{Title: "Real Decision"}, "body"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Specced Right", Status: "specced", Spec: "real-decision"}, "body"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewNode("task", core.Frontmatter{Title: "Specced Wrong", Status: "specced", Spec: "docs/design/does-not-exist.md"}, "body"); err != nil {
		t.Fatal(err)
	}
	// A task with no spec at all must stay silent.
	if _, err := s.NewNode("task", core.Frontmatter{Title: "No Spec", Status: "todo"}, "body"); err != nil {
		t.Fatal(err)
	}

	out := runDoctor(t, s.Dir)
	if !strings.Contains(out, `specced-wrong: spec "docs/design/does-not-exist.md" resolves to neither a node nor a file`) {
		t.Errorf("missing dangling-spec warning:\n%s", out)
	}
	if strings.Contains(out, "specced-right") {
		t.Errorf("a spec that resolves must not be reported:\n%s", out)
	}
	if strings.Contains(out, "no-spec") {
		t.Errorf("a task with no spec must not be reported:\n%s", out)
	}
	if !strings.Contains(out, "✓ no problems") {
		t.Errorf("a dangling spec must be a warning, not a problem:\n%s", out)
	}
}
