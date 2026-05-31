// Package core is the shared engine behind every devlog surface: the CLI, the
// git hook, the MCP server, and the viewer all drive these types. Nothing here
// imports a command framework — it's plain data + filesystem + git.
package core

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the marker that identifies a devlog directory.
const ConfigFile = "config.yml"

// DefaultDirName is the conventional folder a devlog lives in inside a repo.
const DefaultDirName = "devlog"

// Config is the per-devlog settings file (devlog/config.yml). It is small on
// purpose: presentation + the two git-hook knobs.
type Config struct {
	Title        string `yaml:"title"`
	Description  string `yaml:"description,omitempty"`
	Accent       string `yaml:"accent,omitempty"`       // viewer accent colour, e.g. "#e0533d"
	AutoCommit   bool   `yaml:"autoCommit,omitempty"`   // hook commits the stamp itself
	CommitPrefix string `yaml:"commitPrefix,omitempty"` // prefix for stamp commits
}

// DefaultConfig returns the config written by `devlog init`.
func DefaultConfig(title string) Config {
	return Config{
		Title:        title,
		Description:  "A living, committed chronicle of development.",
		Accent:       "#e0533d",
		AutoCommit:   false,
		CommitPrefix: "devlog:",
	}
}

func (c Config) commitPrefix() string {
	if c.CommitPrefix == "" {
		return "devlog:"
	}
	return c.CommitPrefix
}

// loadConfig reads config.yml from a devlog directory.
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

// ErrNoStore is returned when no devlog directory can be found.
var ErrNoStore = errors.New("no devlog found (run `devlog init`)")
