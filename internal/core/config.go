// Package core is the shared engine behind every katra surface: the CLI, the
// git hook, the MCP server, and the viewer all drive these types. Nothing here
// imports a command framework — it's plain data + filesystem + git.
package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the marker that identifies a katra directory.
const ConfigFile = "config.yml"

// EnvDir returns an explicit store dir from the environment: $KATRA_DIR, or the
// legacy $DEVLOG_DIR, or "" when neither is set.
func EnvDir() string {
	if d := os.Getenv("KATRA_DIR"); d != "" {
		return d
	}
	return os.Getenv("DEVLOG_DIR")
}

// DefaultDirName is the conventional folder a katra lives in inside a repo.
const DefaultDirName = "katra"

// LegacyDirName is the previous folder name ("devlog"). FindStore falls back to
// it so repos created before the rename keep working without migration.
const LegacyDirName = "devlog"

// Config is the per-katra settings file (katra/config.yml). It is small on
// purpose: presentation + the two git-hook knobs.
type Config struct {
	Title        string       `yaml:"title"`
	Description  string       `yaml:"description,omitempty"`
	Accent       string       `yaml:"accent,omitempty"`       // viewer accent colour, e.g. "#e0533d"
	AutoCommit   bool         `yaml:"autoCommit,omitempty"`   // hook commits the stamp itself
	CommitPrefix string       `yaml:"commitPrefix,omitempty"` // prefix for stamp commits
	Memory       MemoryConfig `yaml:"memory,omitempty"`       // Claude Code memory ingest settings
}

// MemoryConfig controls the memory-ingest layer (see memory.go). All fields are
// optional; an absent block behaves as enabled with defaults, so older configs
// keep working.
type MemoryConfig struct {
	// Enabled toggles ingest. nil means the default (true) — a pointer so that
	// an explicit `enabled: false` is distinguishable from an absent field.
	Enabled *bool `yaml:"enabled,omitempty"`
	// Types is the set of memory metadata.type values admitted for ingest.
	// Empty means the default: ["project"].
	Types []string `yaml:"types,omitempty"`
	// SensitiveTerms are extra strings that trigger quarantine on a match
	// (case-insensitive), layered on top of the built-in secret detectors.
	SensitiveTerms []string `yaml:"sensitiveTerms,omitempty"`
}

// memoryEnabled reports whether ingest is on (default true when unset).
func (c Config) memoryEnabled() bool {
	if c.Memory.Enabled == nil {
		return true
	}
	return *c.Memory.Enabled
}

// memoryTypes returns the admitted metadata.type values (default ["project"]).
func (c Config) memoryTypes() []string {
	if len(c.Memory.Types) == 0 {
		return []string{"project"}
	}
	return c.Memory.Types
}

// memoryAdmits reports whether a memory file's metadata.type is ingested.
func (c Config) memoryAdmits(t string) bool {
	if t == "" {
		return false
	}
	for _, want := range c.memoryTypes() {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

// DefaultConfig returns the config written by `katra init`.
func DefaultConfig(title string) Config {
	return Config{
		Title:        title,
		Description:  "A living, committed chronicle of development.",
		Accent:       "#e0533d",
		AutoCommit:   false,
		CommitPrefix: "katra:",
	}
}

func (c Config) commitPrefix() string {
	if c.CommitPrefix == "" {
		return "katra:"
	}
	return c.CommitPrefix
}

// loadConfig reads config.yml from a katra directory.
func loadConfig(dir string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Title == "" {
		c.Title = "Dev Log"
	}
	return c, nil
}

// saveConfig writes config.yml.
func saveConfig(dir string, c Config) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ConfigFile), b, 0o644)
}

// ErrNoStore is returned when no katra directory can be found.
var ErrNoStore = errors.New("no katra found (run `katra init`)")
