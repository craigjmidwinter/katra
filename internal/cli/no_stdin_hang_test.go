package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These commands run under hooks, in CI steps and from scripts -- callers whose
// stdin is a pipe with nothing coming. readChunk sniffs stdin for a pipe and
// reads it to EOF, so wiring it in unguarded turns an ordinary command into one
// that hangs forever waiting for input nobody will send. That happened while
// building `new --file`, and it is invisible to a test that runs on a tty.
//
// Each case runs the real binary with stdin held open and empty, and fails on
// timeout rather than blocking the suite.
func buildKatra(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "katra")
	build := exec.Command("go", "build", "-o", bin, "./cmd/katra")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build katra: %v\n%s", err, out)
	}
	return bin
}

func runWithOpenStdin(t *testing.T, bin, dir string, args ...string) (string, error) {
	t.Helper()
	// A pipe we never write to and never close: stdin is not a character
	// device, and there is no EOF. This is the shape a hook provides.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pw.Close(); _ = pr.Close() }()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = pr

	done := make(chan struct{})
	var out []byte
	var runErr error
	go func() {
		out, runErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		return string(out), runErr
	case <-time.After(15 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		t.Fatalf("`katra %s` hung with stdin open and empty — it would hang under a hook", strings.Join(args, " "))
		return "", nil
	}
}

func katraRepo(t *testing.T, bin string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "T"},
		{"commit", "--allow-empty", "-m", "base"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if out, err := runWithOpenStdin(t, bin, dir, "init", "--title", "T"); err != nil {
		t.Fatalf("katra init: %v\n%s", err, out)
	}
	return dir
}

func TestCommandsDoNotHangOnOpenStdin(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the binary; skipped under -short")
	}
	bin := buildKatra(t)
	dir := katraRepo(t, bin)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"new without --file", []string{"new", "A title"}},
		{"checkpoint without a note", []string{"checkpoint", "--dry-run"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runWithOpenStdin(t, bin, dir, tc.args...); err != nil {
				t.Errorf("katra %s: %v", strings.Join(tc.args, " "), err)
			}
		})
	}
}
