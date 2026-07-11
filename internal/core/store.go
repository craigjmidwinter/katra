package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store is a single katra: a directory containing config.yml, entries/, media/.
type Store struct {
	Dir    string // the katra directory (holds config.yml)
	Config Config
}

// EntriesDir is where the per-entry markdown files live.
func (s *Store) EntriesDir() string { return filepath.Join(s.Dir, "entries") }

// TasksDir is where task nodes live.
func (s *Store) TasksDir() string { return filepath.Join(s.Dir, "tasks") }

// EpicsDir is where epic nodes live.
func (s *Store) EpicsDir() string { return filepath.Join(s.Dir, "epics") }

// DecisionsDir is where decision (ADR) nodes live.
func (s *Store) DecisionsDir() string { return filepath.Join(s.Dir, "decisions") }

// ArticlesDir is where article (evergreen reference) nodes live.
func (s *Store) ArticlesDir() string { return filepath.Join(s.Dir, "articles") }

// NodeTypes is the canonical list of node types the store knows about.
var NodeTypes = []string{"entry", "task", "epic", "decision", "article"}

// DirForType returns the store subdirectory for a given node type. An empty
// type (or "entry") maps to the entries dir for back-compat.
func (s *Store) DirForType(t string) string {
	switch t {
	case "task":
		return s.TasksDir()
	case "epic":
		return s.EpicsDir()
	case "decision":
		return s.DecisionsDir()
	case "article":
		return s.ArticlesDir()
	default: // "" or "entry"
		return s.EntriesDir()
	}
}

// MediaDir is where imported images/gifs/video/embeds live.
func (s *Store) MediaDir() string { return filepath.Join(s.Dir, "media") }

// FindStore walks up from start looking for a katra directory. It matches
// either `start/.../katra/config.yml` or a `config.yml` in an ancestor itself.
func FindStore(start string) (*Store, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		if fileExists(filepath.Join(dir, ConfigFile)) && dirExists(filepath.Join(dir, "entries")) {
			return openStore(dir)
		}
		// Prefer the current dir name; fall back to the legacy one so repos
		// created before the rename resolve without migration.
		for _, name := range []string{DefaultDirName, LegacyDirName} {
			if fileExists(filepath.Join(dir, name, ConfigFile)) {
				return openStore(filepath.Join(dir, name))
			}
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

// InitStore creates a new katra directory at dir with the given title.
func InitStore(dir, title string) (*Store, error) {
	if fileExists(filepath.Join(dir, ConfigFile)) {
		return nil, fmt.Errorf("katra already exists at %s", dir)
	}
	for _, d := range []string{
		dir,
		filepath.Join(dir, "entries"),
		filepath.Join(dir, "tasks"),
		filepath.Join(dir, "epics"),
		filepath.Join(dir, "decisions"),
		filepath.Join(dir, "articles"),
		filepath.Join(dir, "media"),
	} {
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

// sortNodes orders nodes newest-first: by date, then time, then filename.
func sortNodes(out []Entry) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FM.Date != out[j].FM.Date {
			return out[i].FM.Date > out[j].FM.Date
		}
		// Same day: creation time orders nodes; untimed (legacy) nodes
		// fall back to filename order among themselves
		if out[i].FM.Time != out[j].FM.Time {
			return out[i].FM.Time > out[j].FM.Time
		}
		return filepath.Base(out[i].Path) > filepath.Base(out[j].Path)
	})
}

// List returns all entries, newest first (by date, then filename).
func (s *Store) List() ([]Entry, error) {
	return s.ListNodes("entry")
}

// ListNodes returns all nodes of the given types, newest first. Each returned
// node has its FM.Type normalized so Kind() reports the requested type (an
// entry with empty Type stays "entry"). With no types, it returns every node
// type the store knows about.
func (s *Store) ListNodes(types ...string) ([]Entry, error) {
	if len(types) == 0 {
		types = NodeTypes
	}
	var out []Entry
	for _, t := range types {
		dir := s.DirForType(t)
		ents, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, de := range ents {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
				continue
			}
			e, err := ParseEntry(filepath.Join(dir, de.Name()))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", de.Name(), err)
			}
			// Normalize the type from the containing dir so callers can rely
			// on Kind(). Entries keep an empty Type (Kind() -> "entry").
			if t != "entry" && e.FM.Type == "" {
				e.FM.Type = t
			}
			out = append(out, e)
		}
	}
	sortNodes(out)
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
	return s.NewNode("entry", fm, body)
}

// NewNode creates and writes a new node of the given type into its per-type
// dir with the <date>-<slug>.md naming scheme (collision-suffixed), returning
// it. An empty nodeType is treated as "entry". The node's frontmatter Type is
// set to nodeType for non-entry types.
func (s *Store) NewNode(nodeType string, fm Frontmatter, body string) (*Entry, error) {
	if nodeType == "" {
		nodeType = "entry"
	}
	if nodeType != "entry" {
		fm.Type = nodeType
	}
	if fm.Date == "" {
		fm.Date = Today()
	}
	if fm.Time == "" {
		fm.Time = NowTime()
	}
	if fm.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	dir := s.DirForType(nodeType)
	slug := Slugify(fm.Title)
	name := fmt.Sprintf("%s-%s.md", fm.Date, slug)
	path := filepath.Join(dir, name)
	for n := 2; fileExists(path); n++ {
		name = fmt.Sprintf("%s-%s-%d.md", fm.Date, slug, n)
		path = filepath.Join(dir, name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	e := Entry{Path: path, Slug: slugFromFilename(path), FM: fm, Body: body}
	if err := e.Save(); err != nil {
		return nil, err
	}
	return &e, nil
}

// GetNode returns the node with the given slug, searching across all type
// dirs. Slugs are expected to be unique across the store; the first match
// (newest-first across all types) wins.
func (s *Store) GetNode(slug string) (*Entry, error) {
	all, err := s.ListNodes()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Slug == slug {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("no node with slug %q", slug)
}

// CloseTasks marks each given task slug done and links it to the entry that
// completed it (sets FM.Entry = entrySlug). Every slug is validated to refer to
// a task before any file is mutated; it returns the slugs it closed.
func (s *Store) CloseTasks(entrySlug string, taskSlugs []string) ([]string, error) {
	nodes := make([]*Entry, 0, len(taskSlugs))
	for _, slug := range taskSlugs {
		n, err := s.GetNode(slug)
		if err != nil {
			return nil, fmt.Errorf("closes %q: %w", slug, err)
		}
		if n.Kind() != "task" {
			return nil, fmt.Errorf("closes %q: not a task (it is a %s)", slug, n.Kind())
		}
		nodes = append(nodes, n)
	}
	closed := make([]string, 0, len(nodes))
	for _, n := range nodes {
		n.FM.Status = "done"
		n.FM.Entry = entrySlug
		if err := n.Save(); err != nil {
			return closed, err
		}
		closed = append(closed, n.Slug)
	}
	return closed, nil
}

// EpicRollupStatus computes an epic's suggested status from its child tasks
// (tasks whose FM.Epic == epicSlug), given the full set of nodes. Cut tasks are
// ignored. It returns "" when the epic has no non-cut child tasks — meaning
// "no basis; keep the stored status". Otherwise: all done -> "done"; any work
// started (a done or doing child) -> "active"; all still todo -> "planned".
func EpicRollupStatus(nodes []Entry, epicSlug string) string {
	var total, done, started int
	for _, n := range nodes {
		if n.Kind() != "task" || n.FM.Epic != epicSlug {
			continue
		}
		switch n.FM.Status {
		case "cut":
			continue
		case "done":
			done++
		case "doing":
			started++
		}
		total++
	}
	switch {
	case total == 0:
		return ""
	case done == total:
		return "done"
	case done > 0 || started > 0:
		return "active"
	default:
		return "planned"
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
