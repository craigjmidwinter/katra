package core

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Stat is a commit diffstat: files changed, lines added, lines deleted.
type Stat struct {
	F int `yaml:"f"`
	A int `yaml:"a"`
	D int `yaml:"d"`
}

// Add accumulates another stat (used when chaptering several commits).
func (s *Stat) Add(o Stat) {
	s.F += o.F
	s.A += o.A
	s.D += o.D
}

// Frontmatter is the YAML block at the top of every entry file.
type Frontmatter struct {
	Title    string   `yaml:"title"`
	Date     string   `yaml:"date"`
	Time     string   `yaml:"time,omitempty"` // HH:MM:SS, orders same-day entries
	Tags     []string `yaml:"tags,omitempty"`
	Hash     string   `yaml:"hash,omitempty"`
	Hashes   []string `yaml:"hashes,omitempty"`
	Stat     *Stat    `yaml:"stat,omitempty"`
	Summary  string   `yaml:"summary,omitempty"`
	Cover    string   `yaml:"cover,omitempty"`
	Featured bool     `yaml:"featured,omitempty"` // lands in the "Deep Dives" zone
	Pinned   bool     `yaml:"pinned,omitempty"`

	// Node-model fields (Almanac). Empty Type means "entry" for back-compat.
	Type         string   `yaml:"type,omitempty"`          // "" or "entry" = entry; else task|epic|decision|article
	Status       string   `yaml:"status,omitempty"`        // task: todo|doing|done|cut ; epic: planned|active|done|cut ; decision: proposed|accepted|superseded|deprecated
	Effort       string   `yaml:"effort,omitempty"`        // S|M|L
	Horizon      string   `yaml:"horizon,omitempty"`       // now|next|later
	Epic         string   `yaml:"epic,omitempty"`          // task -> parent epic slug
	Entry        string   `yaml:"entry,omitempty"`         // task/decision -> the entry slug that recorded/occasioned it
	Supersedes   []string `yaml:"supersedes,omitempty"`    // decision chain
	SupersededBy []string `yaml:"superseded-by,omitempty"` // mirror of supersedes
}

// Entry is one devlog post: a markdown file with YAML frontmatter.
type Entry struct {
	Path string      // absolute path to the .md file
	Slug string      // filename minus the date prefix and extension
	FM   Frontmatter // parsed frontmatter
	Body string      // markdown body (everything after the frontmatter)
}

// Kind returns the node type: FM.Type, or "entry" when empty (back-compat).
func (e Entry) Kind() string {
	if e.FM.Type == "" {
		return "entry"
	}
	return e.FM.Type
}

// IsDraft reports whether the entry has not yet been stamped with a commit.
// A draft renders in the "In Progress" panel; it just needs a hash.
// Draft-ness is an entry-only concept — other node types (task/epic/decision/
// article) carry their own lifecycle in FM.Status and are never "drafts".
func (e Entry) IsDraft() bool {
	return e.Kind() == "entry" && e.FM.Hash == "" && len(e.FM.Hashes) == 0
}

// AllHashes returns the unified list of commit hashes for the entry.
func (e Entry) AllHashes() []string {
	if len(e.FM.Hashes) > 0 {
		return e.FM.Hashes
	}
	if e.FM.Hash != "" {
		return []string{e.FM.Hash}
	}
	return nil
}

var fmDelim = []byte("---")

// ParseEntry reads and parses an entry file.
func ParseEntry(path string) (Entry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, err
	}
	return parseEntryBytes(path, b)
}

func parseEntryBytes(path string, b []byte) (Entry, error) {
	e := Entry{Path: path, Slug: slugFromFilename(path)}
	rest := b
	// Optional frontmatter: a leading `---` line, YAML, then a closing `---` line.
	if bytes.HasPrefix(b, fmDelim) {
		lines := bytes.SplitN(b, []byte("\n"), 2)
		if len(lines) == 2 {
			body := lines[1]
			idx := bytes.Index(body, []byte("\n---"))
			if idx >= 0 {
				yamlBlock := body[:idx]
				after := body[idx+1:] // starts at "---"
				if nl := bytes.IndexByte(after, '\n'); nl >= 0 {
					rest = after[nl+1:]
				} else {
					rest = nil
				}
				if err := yaml.Unmarshal(yamlBlock, &e.FM); err != nil {
					return e, fmt.Errorf("frontmatter: %w", err)
				}
			}
		}
	}
	e.Body = strings.TrimLeft(string(rest), "\n")
	return e, nil
}

// Serialize renders the entry back to file bytes (frontmatter + body).
func (e Entry) Serialize() ([]byte, error) {
	y, err := yaml.Marshal(e.FM)
	if err != nil {
		return nil, err
	}
	var sb bytes.Buffer
	sb.WriteString("---\n")
	sb.Write(y)
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimRight(e.Body, "\n"))
	sb.WriteString("\n")
	return sb.Bytes(), nil
}

// Save writes the entry back to its Path.
func (e Entry) Save() error {
	b, err := e.Serialize()
	if err != nil {
		return err
	}
	return os.WriteFile(e.Path, b, 0o644)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a title into a filesystem-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

var datePrefixRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)

func slugFromFilename(path string) string {
	name := path
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".md")
	name = datePrefixRe.ReplaceAllString(name, "")
	return name
}

// Today returns today's date in the canonical YYYY-MM-DD form.
func Today() string { return time.Now().Format("2006-01-02") }

// NowTime returns the current wall-clock time in the canonical HH:MM:SS form.
func NowTime() string { return time.Now().Format("15:04:05") }
