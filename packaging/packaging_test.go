// Package packaging holds no code. Its tests guard the release metadata that
// lives outside Go -- the MCP Bundle manifest, server.json and the goreleaser
// matrix -- against the one failure mode prose cannot catch: the three drifting
// apart. See docs/design/mcpb-bundle.md.
package packaging

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot is the directory above this package; `go test` runs with the
// package directory as its working directory.
const repoRoot = ".."

// renderManifest substitutes the version placeholder the way
// scripts/build-mcpb.sh does, then parses the result.
func renderManifest(t *testing.T, version string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot, "packaging", "mcpb", "manifest.json.tmpl"))
	if err != nil {
		t.Fatalf("read manifest template: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal([]byte(strings.ReplaceAll(string(raw), "__VERSION__", version)), &manifest); err != nil {
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
// the release does not build. Widening compatibility.platforms to the spec's
// full enum is the tempting, wrong edit: it advertises a Windows bundle that
// would never exist.
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
		if declared[i] != want {
			t.Errorf("platform %d: manifest says %v, goreleaser builds %q", i, declared[i], want)
		}
	}
}

// TestBuildScriptRejectsUnshippedPlatform proves the guard fires rather than
// trusting that it would. A gate nobody has faulted is a gate nobody knows the
// state of.
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

	out, err := exec.Command(script, binary, "windows", "amd64", "1.2.3", t.TempDir()).CombinedOutput()
	if err == nil {
		t.Fatal("build-mcpb.sh bundled goos=windows, which the release does not build")
	}
	if !strings.Contains(string(out), "refusing to bundle unshipped platform") {
		t.Errorf("guard fired for the wrong reason: %s", out)
	}
}
