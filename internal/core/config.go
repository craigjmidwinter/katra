// Package core is the shared engine behind every katra surface: the CLI, the
// git hook, the MCP server, and the viewer all drive these types. Nothing here
// imports a command framework — it's plain data + filesystem + git.
package core

import (
	"errors"
	"os"
	"path/filepath"

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
	Title        string `yaml:"title"`
	Description  string `yaml:"description,omitempty"`
	Accent       string `yaml:"accent,omitempty"`       // viewer accent colour, e.g. "#e0533d"
	AutoCommit   bool   `yaml:"autoCommit,omitempty"`   // hook commits the stamp itself
	CommitPrefix string `yaml:"commitPrefix,omitempty"` // prefix for stamp commits
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
