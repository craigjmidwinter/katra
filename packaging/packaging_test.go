// Package packaging holds no code. Its tests guard the release metadata that
// lives outside Go -- the MCP Bundle manifest, server.json and the goreleaser
// matrix -- against the one failure mode prose cannot catch: the three drifting
// apart. See docs/design/mcpb-bundle.md.
package packaging

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot is the directory above this package; `go test` runs with the
// package directory as its working directory.
const repoRoot = ".."

// renderManifest substitutes the Unix values the way scripts/build-mcpb.sh
// does, then parses the result.
func renderManifest(t *testing.T, version string) map[string]any {
	return renderManifestForBinary(t, version, "katra-mcp")
}

func renderManifestForBinary(t *testing.T, version, binary string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot, "packaging", "mcpb", "manifest.json.tmpl"))
	if err != nil {
		t.Fatalf("read manifest template: %v", err)
	}

	var manifest map[string]any
	rendered := strings.NewReplacer("__VERSION__", version, "__BINARY__", binary).Replace(string(raw))
	if err := json.Unmarshal([]byte(rendered), &manifest); err != nil {
		t.Fatalf("rendered manifest is not valid JSON: %v", err)
	}
	return manifest
}

func serverJSON(t *testing.T) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot, "server.json"))
	if err != nil {
		t.Fatalf("read server.json: %v", err)
	}

	var server map[string]any
	if err := json.Unmarshal(raw, &server); err != nil {
		t.Fatalf("server.json is not valid JSON: %v", err)
	}
	return server
}

// TestManifestHasRequiredFields pins the six fields modelcontextprotocol/mcpb
// lists as required. A bundle missing one is rejected at install time, which is
// the most expensive place to find out.
func TestManifestHasRequiredFields(t *testing.T) {
	manifest := renderManifest(t, "1.2.3")

	for _, field := range []string{"manifest_version", "name", "version", "description", "author", "server"} {
		if _, ok := manifest[field]; !ok {
			t.Errorf("manifest is missing required field %q", field)
		}
	}

	if got := manifest["manifest_version"]; got != "0.3" {
		t.Errorf("manifest_version = %v, want 0.3", got)
	}
	if got := manifest["version"]; got != "1.2.3" {
		t.Errorf("version placeholder did not render: got %v", got)
	}
}

// TestManifestServerMatchesBundleLayout keeps the manifest's entry point and
// the directory scripts/build-mcpb.sh actually writes in agreement. They are
// declared in two files and there is nothing but this test between them.
func TestManifestServerMatchesBundleLayout(t *testing.T) {
	server, ok := renderManifest(t, "1.2.3")["server"].(map[string]any)
	if !ok {
		t.Fatal("manifest server block is not an object")
	}

	if got := server["type"]; got != "binary" {
		t.Errorf("server.type = %v, want binary; katra ships a pre-compiled executable", got)
	}
	if got := server["entry_point"]; got != "server/katra-mcp" {
		t.Errorf("server.entry_point = %v, want server/katra-mcp (the path build-mcpb.sh stages)", got)
	}

	config, ok := server["mcp_config"].(map[string]any)
	if !ok {
		t.Fatal("manifest server.mcp_config is not an object")
	}
	if got := config["command"]; got != "${__dirname}/server/katra-mcp" {
		t.Errorf("server.mcp_config.command = %v, want ${__dirname}/server/katra-mcp", got)
	}
}

// TestManifestDescriptionMatchesServerJSON is the anti-drift test. katra is
// listed in two places by two different mechanisms; if the sentence describing
// it diverges, one of the two listings is lying about the project and nobody
// notices, because nobody reads both.
func TestManifestDescriptionMatchesServerJSON(t *testing.T) {
	manifest := renderManifest(t, "1.2.3")
	server := serverJSON(t)

	if manifest["description"] != server["description"] {
		t.Errorf("description drift:\n  manifest.json.tmpl: %v\n  server.json:        %v",
			manifest["description"], server["description"])
	}
}

// TestManifestHomepageMatchesServerJSON keeps the two listings pointing at the
// same page, and — because the check is parity rather than a hardcoded string —
// makes moving that page a one-line edit in one file rather than a hunt.
//
// Both previously named https://craigjmidwinter.github.io/katra/, which 301s to
// *http://* midwinter.io/katra/ — a redirect through plaintext on the way to the
// page. A published listing is a link other people follow; it should be the
// address, not a forwarding note.
func TestManifestHomepageMatchesServerJSON(t *testing.T) {
	manifest := renderManifest(t, "1.2.3")
	server := serverJSON(t)

	if manifest["homepage"] != server["websiteUrl"] {
		t.Errorf("homepage drift:\n  manifest.json.tmpl homepage: %v\n  server.json websiteUrl:      %v",
			manifest["homepage"], server["websiteUrl"])
	}
}

// TestManifestPlatformsMatchBuildMatrix stops the manifest claiming a platform
// the release does not build. The explicit Windows mapping also prevents Go's
// GOOS spelling from leaking into MCPB's platform vocabulary.
func TestManifestPlatformsMatchBuildMatrix(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}

	var config struct {
		Builds []struct {
			ID   string   `yaml:"id"`
			GOOS []string `yaml:"goos"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}

	var built []string
	for _, build := range config.Builds {
		if build.ID == "katra-mcp" {
			built = build.GOOS
		}
	}
	if built == nil {
		t.Fatal("no katra-mcp build in .goreleaser.yml; the bundle has nothing to package")
	}

	compatibility, ok := renderManifest(t, "1.2.3")["compatibility"].(map[string]any)
	if !ok {
		t.Fatal("manifest compatibility block is not an object")
	}
	declared, ok := compatibility["platforms"].([]any)
	if !ok {
		t.Fatal("manifest compatibility.platforms is not an array")
	}

	if len(declared) != len(built) {
		t.Fatalf("manifest declares %d platforms, goreleaser builds %d (%v)", len(declared), len(built), built)
	}
	for i, want := range built {
		if want == "windows" {
			want = "win32"
		}
		if declared[i] != want {
			t.Errorf("platform %d: manifest says %v, goreleaser builds %q", i, declared[i], want)
		}
	}
}

// TestBuildScriptRejectsUnshippedPlatform proves the guard fires rather than
// trusting that it would. A gate nobody has faulted is a gate nobody knows the
// state of.
func runBundleScript(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	script := filepath.Join(repoRoot, "scripts", "build-mcpb.sh")
	if runtime.GOOS == "windows" {
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Skipf("bash is required to exercise build-mcpb.sh: %v", err)
		}
		return exec.Command(bash, append([]string{script}, args...)...).CombinedOutput()
	}
	return exec.Command(script, args...).CombinedOutput()
}

func TestBuildScriptRejectsUnshippedPlatform(t *testing.T) {
	script := filepath.Join(repoRoot, "scripts", "build-mcpb.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("stat build-mcpb.sh: %v", err)
	}

	// Any executable stands in for the binary; the platform check runs first.
	binary, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("no /usr/bin/true to stand in for the binary: %v", err)
	}

	out, err := runBundleScript(t, binary, "plan9", "amd64", "1.2.3", t.TempDir())
	if err == nil {
		t.Fatal("build-mcpb.sh bundled goos=plan9, which the release does not build")
	}
	if !strings.Contains(string(out), "refusing to bundle unshipped platform") {
		t.Errorf("guard fired for the wrong reason: %s", out)
	}
}

func TestBuildScriptPackagesWindowsExecutable(t *testing.T) {
	binary, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("no executable to stand in for the binary: %v", err)
	}
	outDir := t.TempDir()
	if out, err := runBundleScript(t, binary, "windows", "amd64", "1.2.3", outDir); err != nil {
		t.Fatalf("build Windows bundle: %v\n%s", err, out)
	}

	bundle := filepath.Join(outDir, "katra_1.2.3_windows_amd64.mcpb")
	zr, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatalf("open Windows bundle: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var manifest map[string]any
	foundBinary := false
	for _, f := range zr.File {
		switch f.Name {
		case "server/katra-mcp.exe":
			foundBinary = true
		case "manifest.json":
			r, err := f.Open()
			if err != nil {
				t.Fatalf("open bundled manifest: %v", err)
			}
			data, readErr := io.ReadAll(r)
			_ = r.Close()
			if readErr != nil {
				t.Fatalf("read bundled manifest: %v", readErr)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("parse bundled manifest: %v", err)
			}
		}
	}
	if !foundBinary {
		t.Error("Windows bundle does not contain server/katra-mcp.exe")
	}
	server, ok := manifest["server"].(map[string]any)
	if !ok {
		t.Fatal("Windows bundle manifest has no server object")
	}
	if got := server["entry_point"]; got != "server/katra-mcp.exe" {
		t.Errorf("Windows entry_point = %v, want server/katra-mcp.exe", got)
	}
	config, ok := server["mcp_config"].(map[string]any)
	if !ok {
		t.Fatal("Windows bundle manifest has no mcp_config object")
	}
	if got := config["command"]; got != "${__dirname}/server/katra-mcp.exe" {
		t.Errorf("Windows command = %v, want ${__dirname}/server/katra-mcp.exe", got)
	}
}
