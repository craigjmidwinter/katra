package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo makes dir a git repo with a katra store inside it, and returns the
// store. The store lives in a subdirectory, matching real layouts.
func initRepoWithStore(t *testing.T) (root string, s *Store) {
	t.Helper()
	root = t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	s, err := InitStore(filepath.Join(root, "katra"), "Test")
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return root, s
}

// TestInstallHookDefaultLocation covers the ordinary repo: the hook lands in
// .git/hooks even though the store lives one level down.
func TestInstallHookDefaultLocation(t *testing.T) {
	root, s := initRepoWithStore(t)
	got, err := s.InstallHook()
	if err != nil {
		t.Fatalf("InstallHook: %v", err)
	}
	want := filepath.Join(root, ".git", "hooks", "post-commit")
	if got != want {
		t.Fatalf("hook path = %q, want %q", got, want)
	}
	b, err := os.ReadFile(got)
	if err != nil || !strings.Contains(string(b), hookMarker) {
		t.Fatalf("hook not written with marker: %v / %s", err, b)
	}
}

// TestInstallHookHuskyUsesTrackedDir is the regression this file exists for.
// husky points core.hooksPath at .husky/_, which `husky install` regenerates on
// every `npm install`. Worse, the generated shim sources `h`, which exec's the
// tracked script and then exits — so a block appended to .husky/_/post-commit
// is not merely fragile, it never runs at all. The hook must go to the tracked
// .husky/post-commit that the shim actually invokes.
func TestInstallHookHuskyUsesTrackedDir(t *testing.T) {
	root, s := initRepoWithStore(t)
	shimDir := filepath.Join(root, ".husky", "_")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// `h` is husky v9's shim runner; its presence is what marks the directory.
	if err := os.WriteFile(filepath.Join(shimDir, "h"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "config", "core.hooksPath", ".husky/_")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}

	got, err := s.InstallHook()
	if err != nil {
		t.Fatalf("InstallHook: %v", err)
	}
	want := filepath.Join(root, ".husky", "post-commit")
	if got != want {
		t.Fatalf("hook path = %q, want %q (must not be the generated _ dir)", got, want)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "post-commit")); err == nil {
		t.Fatal("wrote into husky's generated _ dir, which husky regenerates")
	}

	// And it must be removable again from the same place.
	if _, err := s.UninstallHook(); err != nil {
		t.Fatalf("UninstallHook: %v", err)
	}
	if b, err := os.ReadFile(want); err == nil && strings.Contains(string(b), hookMarker) {
		t.Fatal("UninstallHook left the block behind")
	}
}

// TestHuskyHookDirIgnoresLookalikes keeps the redirect from firing on any
// directory that merely happens to be named `_`.
func TestHuskyHookDirIgnoresLookalikes(t *testing.T) {
	tmp := t.TempDir()
	cases := []struct {
		name string
		dir  string
		set  func(string)
	}{
		{"parent not .husky", filepath.Join(tmp, "hooks", "_"), nil},
		{"no shim runner", filepath.Join(tmp, ".husky", "_"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := os.MkdirAll(c.dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if got := huskyHookDir(c.dir); got != c.dir {
				t.Fatalf("huskyHookDir(%q) = %q, want unchanged", c.dir, got)
			}
		})
	}
	// Positive control: .husky/_ with a shim runner does redirect.
	d := filepath.Join(tmp, "ok", ".husky", "_")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "husky.sh"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := huskyHookDir(d), filepath.Dir(d); got != want {
		t.Fatalf("huskyHookDir(%q) = %q, want %q", d, got, want)
	}
}
