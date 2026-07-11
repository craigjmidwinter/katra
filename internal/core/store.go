package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store is a single devlog: a directory containing config.yml, entries/, media/.
type Store struct {
	Dir    string // the devlog directory (holds config.yml)
	Config Config
}

// EntriesDir is where the per-entry markdown files live.
func (s *Store) EntriesDir() string { return filepath.Join(s.Dir, "entries") }

// MediaDir is where imported images/gifs/video/embeds live.
func (s *Store) MediaDir() string { return filepath.Join(s.Dir, "media") }

// FindStore walks up from start looking for a devlog directory. It matches
// either `start/.../devlog/config.yml` or a `config.yml` in an ancestor itself.
func FindStore(start string) (*Store, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		if fileExists(filepath.Join(dir, ConfigFile)) && dirExists(filepath.Join(dir, "entries")) {
			return openStore(dir)
		}
		cand := filepath.Join(dir, DefaultDirName, ConfigFile)
		if fileExists(cand) {
			return openStore(filepath.Join(dir, DefaultDirName))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNoStore
		}
		dir = parent
	}
}

func openStore(dir string) (*Store, error) {
	cfg, err := loadConfig(dir)
	if err != nil {
		return nil, err
	}
	return &Store{Dir: dir, Config: cfg}, nil
}

// InitStore creates a new devlog directory at dir with the given title.
func InitStore(dir, title string) (*Store, error) {
	if fileExists(filepath.Join(dir, ConfigFile)) {
		return nil, fmt.Errorf("devlog already exists at %s", dir)
	}
	for _, d := range []string{dir, filepath.Join(dir, "entries"), filepath.Join(dir, "media")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	cfg := DefaultConfig(title)
	if err := saveConfig(dir, cfg); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, Config: cfg}, nil
}

// List returns all entries, newest first (by date, then filename).
func (s *Store) List() ([]Entry, error) {
	ents, err := os.ReadDir(s.EntriesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, de := range ents {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		e, err := ParseEntry(filepath.Join(s.EntriesDir(), de.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", de.Name(), err)
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FM.Date != out[j].FM.Date {
			return out[i].FM.Date > out[j].FM.Date
		}
		// Same day: creation time orders entries; untimed (legacy) entries
		// fall back to filename order among themselves
		if out[i].FM.Time != out[j].FM.Time {
			return out[i].FM.Time > out[j].FM.Time
		}
		return filepath.Base(out[i].Path) > filepath.Base(out[j].Path)
	})
	return out, nil
}

// ActiveDraft returns the newest unstamped entry — the one a stamp targets.
func (s *Store) ActiveDraft() (*Entry, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].IsDraft() {
			return &all[i], nil
		}
	}
	return nil, nil
}

// Get returns the entry with the given slug.
func (s *Store) Get(slug string) (*Entry, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Slug == slug {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("no entry with slug %q", slug)
}

// NewEntry creates and writes a new draft entry, returning it.
func (s *Store) NewEntry(fm Frontmatter, body string) (*Entry, error) {
	if fm.Date == "" {
		fm.Date = Today()
	}
	if fm.Time == "" {
		fm.Time = NowTime()
	}
	if fm.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	slug := Slugify(fm.Title)
	name := fmt.Sprintf("%s-%s.md", fm.Date, slug)
	path := filepath.Join(s.EntriesDir(), name)
	for n := 2; fileExists(path); n++ {
		name = fmt.Sprintf("%s-%s-%d.md", fm.Date, slug, n)
		path = filepath.Join(s.EntriesDir(), name)
	}
	if err := os.MkdirAll(s.EntriesDir(), 0o755); err != nil {
		return nil, err
	}
	e := Entry{Path: path, Slug: slugFromFilename(path), FM: fm, Body: body}
	if err := e.Save(); err != nil {
		return nil, err
	}
	return &e, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
