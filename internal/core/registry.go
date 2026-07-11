package core

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// HubRegistry is the machine-global list of katra store directories, so a future
// hub daemon (and `katra hub list` today) can find every project without a
// per-invocation scan. It lives at RegistryPath().
type HubRegistry struct {
	Projects []string `yaml:"projects"`

	path string // where this registry was loaded from / saves to
}

// RegistryPath returns the global registry file location: $KATRA_REGISTRY, else
// $XDG_CONFIG_HOME/katra/registry.yml, else ~/.config/katra/registry.yml.
func RegistryPath() string {
	if p := os.Getenv("KATRA_REGISTRY"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "katra", "registry.yml")
}

// LoadRegistry reads the global registry, returning an empty one if the file
// does not exist yet.
func LoadRegistry() (*HubRegistry, error) {
	path := RegistryPath()
	r := &HubRegistry{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(b, r); err != nil {
		return nil, err
	}
	r.path = path
	return r, nil
}

// Add records a store directory (as an absolute path), keeping the list sorted
// and de-duplicated. It reports whether the entry was newly added.
func (r *HubRegistry) Add(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	for _, p := range r.Projects {
		if p == abs {
			return false
		}
	}
	r.Projects = append(r.Projects, abs)
	sort.Strings(r.Projects)
	return true
}

// Prune drops entries whose store no longer exists, returning the removed paths.
func (r *HubRegistry) Prune() []string {
	kept := r.Projects[:0:0]
	var removed []string
	for _, p := range r.Projects {
		if fileExists(filepath.Join(p, ConfigFile)) {
			kept = append(kept, p)
		} else {
			removed = append(removed, p)
		}
	}
	r.Projects = kept
	return removed
}

// Save writes the registry back to its file, creating the parent dir.
func (r *HubRegistry) Save() error {
	if r.path == "" {
		r.path = RegistryPath()
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, b, 0o644)
}

// Register adds dir to the global registry (if not already present) and saves.
// It is a no-op when the entry already exists.
func Register(dir string) error {
	r, err := LoadRegistry()
	if err != nil {
		return err
	}
	if r.Add(dir) {
		return r.Save()
	}
	return nil
}
