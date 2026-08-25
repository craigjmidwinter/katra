package buildinfo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigjmidwinter/katra/internal/buildinfo"
)

// TestVersionInGoInstallBuild is the regression test for the actual bug: a
// binary produced the way the README's first install path produces one has to
// be able to name its own build.
//
// A unit test cannot cover this. The fallback reads runtime/debug build info,
// which the `go` tool only populates for a module-aware install — under
// `go test` the main module's version is always "(devel)". So this drives the
// real command, and asserts on what a user would actually type.
func TestVersionInGoInstallBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and links two binaries; skipped under -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}

	// Install from the module in the working tree rather than from the proxy,
	// so this tests the code under test and works offline.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()

	install := exec.Command("go", "install", "./cmd/katra")
	install.Dir = root
	install.Env = append(os.Environ(), "GOBIN="+bin)
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("go install: %v\n%s", err, out)
	}

	out, err := exec.Command(filepath.Join(bin, "katra"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("katra --version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))

	// The module is a git repository, so the go tool synthesises a pseudo-version
	// (v0.0.0-<time>-<commit>) rather than "(devel)" -- which is why this can
	// assert the real thing rather than a proxy for it. Installed at a tag, the
	// same path reports that tag.
	if got == "" {
		t.Fatal("katra --version printed nothing")
	}
	if strings.HasSuffix(got, " "+buildinfo.Stamped) {
		t.Errorf("katra --version = %q; a go install build still cannot name its own build", got)
	}
	if strings.Contains(got, "(devel)") {
		t.Errorf("katra --version = %q; the module system's placeholder reached the user", got)
	}
	t.Logf("go install build reports: %s", got)
}
