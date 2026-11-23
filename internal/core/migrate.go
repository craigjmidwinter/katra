package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Interim tags are the conventions a katra used before owner, blocked_by and
// the full effort scale had keys: `owner:<who>`, `blocked-by:<slug>` and
// `size:<effort>` in a node's tags. A migration moves each into its key and
// drops the tag, so the two can never disagree afterwards.
const (
	tagOwner     = "owner:"
	tagBlockedBy = "blocked-by:"
	tagSize      = "size:"
)

// TagMigration is the plan for one node. Changes describes each edit for a
// dry run; Skipped describes tags left in place and why. Node is the node as
// it would be written.
type TagMigration struct {
	Node    Entry
	Changes []string
	Skipped []string
}

// PlanInterimTags plans the migration for every task and epic carrying an
// interim tag, touching nothing. It is pure, so a dry run and a write use the
// same plan, and a second plan after a write is empty: that is idempotence.
//
// Conflicts are skipped rather than guessed: two owner tags, or an owner tag
// on a node whose owner key already says someone else, stay as tags and are
// reported. A size tag is the more specific record (the scale had no XL, so
// those tasks were written `effort: L` with `size:XL` beside it), so it wins
// over effort when it is on the scale.
func PlanInterimTags(nodes []Entry) []TagMigration {
	var out []TagMigration
	for _, n := range nodes {
		if n.Kind() != "task" && n.Kind() != "epic" {
			continue
		}
		m := TagMigration{Node: n}
		var owners, sizes []string
		for _, t := range n.FM.Tags {
			if v, ok := cutTag(t, tagOwner); ok {
				owners = append(owners, v)
			}
			if v, ok := cutTag(t, tagSize); ok {
				sizes = append(sizes, v)
			}
		}
		var kept []string
		for _, t := range n.FM.Tags {
			switch {
			case hasTagPrefix(t, tagOwner):
				v, _ := cutTag(t, tagOwner)
				cur := strings.TrimSpace(m.Node.FM.Owner)
				switch {
				case v == "":
					m.Skipped = append(m.Skipped, fmt.Sprintf("%q: empty owner", t))
					kept = append(kept, t)
				case len(owners) > 1:
					m.Skipped = append(m.Skipped, fmt.Sprintf("%q: %d owner tags, set the owner by hand", t, len(owners)))
					kept = append(kept, t)
				case cur != "" && cur != v:
					m.Skipped = append(m.Skipped, fmt.Sprintf("%q: owner is already %q", t, cur))
					kept = append(kept, t)
				default:
					if cur == "" {
						m.Changes = append(m.Changes, "owner: "+v)
					}
					m.Node.FM.Owner = v
				}
			case hasTagPrefix(t, tagBlockedBy):
				v, _ := cutTag(t, tagBlockedBy)
				if v == "" {
					m.Skipped = append(m.Skipped, fmt.Sprintf("%q: empty blocker", t))
					kept = append(kept, t)
					continue
				}
				if !contains(m.Node.FM.BlockedBy, v) {
					m.Node.FM.BlockedBy = append(append([]string(nil), m.Node.FM.BlockedBy...), v)
					m.Changes = append(m.Changes, "blocked_by: + "+v)
				}
			case hasTagPrefix(t, tagSize):
				v, _ := cutTag(t, tagSize)
				if len(sizes) > 1 {
					m.Skipped = append(m.Skipped, fmt.Sprintf("%q: %d size tags, set the effort by hand", t, len(sizes)))
					kept = append(kept, t)
					continue
				}
				eff, err := NormalizeEffort(v)
				if err != nil || eff == "" {
					m.Skipped = append(m.Skipped, fmt.Sprintf("%q: not on the effort scale (%s)", t, strings.Join(Efforts, "|")))
					kept = append(kept, t)
					continue
				}
				if m.Node.FM.Effort != eff {
					m.Changes = append(m.Changes, fmt.Sprintf("effort: %s → %s", orNone(m.Node.FM.Effort), eff))
					m.Node.FM.Effort = eff
				}
			default:
				kept = append(kept, t)
			}
		}
		if len(kept) != len(n.FM.Tags) {
			m.Changes = append(m.Changes, fmt.Sprintf("tags: drop %d migrated", len(n.FM.Tags)-len(kept)))
			m.Node.FM.Tags = kept
		}
		if len(m.Changes) > 0 || len(m.Skipped) > 0 {
			out = append(out, m)
		}
	}
	return out
}

// Changed reports whether the plan writes anything for this node.
func (m TagMigration) Changed() bool { return len(m.Changes) > 0 }

func hasTagPrefix(t, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(t)), prefix)
}

func cutTag(t, prefix string) (string, bool) {
	t = strings.TrimSpace(t)
	if !hasTagPrefix(t, prefix) {
		return "", false
	}
	return strings.TrimSpace(t[len(prefix):]), true
}

// Apply writes the plan to the node's file as a minimal edit: the frontmatter
// is edited as a YAML node tree, so every key and value the migration does
// not touch keeps its exact spelling and quoting, and the body is copied byte
// for byte. Save would re-serialize the whole frontmatter in katra's own
// style, which on a hand-written file re-quotes titles and drops blank lines:
// noise in a diff that should show only the migration.
func (m TagMigration) Apply() error {
	if !m.Changed() {
		return nil
	}
	b, err := os.ReadFile(m.Node.Path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(b, fmDelim) {
		return fmt.Errorf("%s: no frontmatter to edit", m.Node.Path)
	}
	first := bytes.IndexByte(b, '\n')
	end := bytes.Index(b[first:], []byte("\n---"))
	if first < 0 || end < 0 {
		return fmt.Errorf("%s: unterminated frontmatter", m.Node.Path)
	}
	block, rest := b[first+1:first+end+1], b[first+end+1:]

	var doc yaml.Node
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return fmt.Errorf("%s: frontmatter: %w", m.Node.Path, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s: frontmatter is not a mapping", m.Node.Path)
	}
	mp := doc.Content[0]

	// Tags: drop exactly the items the plan dropped, keep the rest in place.
	if i := mapIndex(mp, "tags"); i >= 0 && mp.Content[i+1].Kind == yaml.SequenceNode {
		seq := mp.Content[i+1]
		var kept []*yaml.Node
		for _, it := range seq.Content {
			migrated := hasTagPrefix(it.Value, tagOwner) || hasTagPrefix(it.Value, tagBlockedBy) || hasTagPrefix(it.Value, tagSize)
			if !migrated || contains(m.Node.FM.Tags, it.Value) {
				kept = append(kept, it)
			}
		}
		if len(kept) == 0 {
			mp.Content = append(mp.Content[:i], mp.Content[i+2:]...)
		} else {
			seq.Content = kept
		}
	}
	setScalar(mp, "effort", m.Node.FM.Effort)
	setScalar(mp, "owner", m.Node.FM.Owner)
	if len(m.Node.FM.BlockedBy) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, v := range m.Node.FM.BlockedBy {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
		}
		setNode(mp, "blocked_by", seq)
	}

	var out bytes.Buffer
	out.Write(b[:first+1])
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(listIndent(block))
	if err := enc.Encode(mp); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	out.Write(rest)

	// Never write a file katra cannot read back with the values planned.
	got, err := parseEntryBytes(m.Node.Path, out.Bytes())
	if err != nil || len(got.Unreadable) > 0 || got.FM.Owner != m.Node.FM.Owner ||
		got.FM.Effort != m.Node.FM.Effort || strings.Join(got.FM.BlockedBy, "\x00") != strings.Join(m.Node.FM.BlockedBy, "\x00") {
		return fmt.Errorf("%s: the migrated frontmatter does not read back as planned (%v %v) — file left unchanged", m.Node.Path, err, got.Unreadable)
	}
	// Write beside the file and rename over it, so an interrupted run leaves
	// either the old file or the new one, never half of one.
	tmp, err := os.CreateTemp(filepath.Dir(m.Node.Path), ".katra-migrate-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(out.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if fi, err := os.Stat(m.Node.Path); err == nil {
		_ = os.Chmod(tmp.Name(), fi.Mode().Perm())
	}
	return os.Rename(tmp.Name(), m.Node.Path)
}

// listIndent returns the indent a frontmatter block uses for list items (the
// spaces before its first "- "), so a migrated file keeps its own layout.
// katra writes 4; a hand-written file may use 2.
func listIndent(block []byte) int {
	for _, line := range strings.Split(string(block), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if n := len(line) - len(trimmed); n >= 2 && strings.HasPrefix(trimmed, "- ") {
			return n
		}
	}
	return 4
}

// mapIndex returns the index of key's key node in a mapping, or -1.
func mapIndex(mp *yaml.Node, key string) int {
	for i := 0; i+1 < len(mp.Content); i += 2 {
		if mp.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// setScalar sets a string key, keeping the existing node's style when the key
// is already there; an empty value leaves the mapping as it is.
func setScalar(mp *yaml.Node, key, value string) {
	if value == "" {
		return
	}
	if i := mapIndex(mp, key); i >= 0 {
		v := mp.Content[i+1]
		if v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Value == value {
			return // untouched, so its quoting stays exactly as written
		}
		// Replace the node outright. Editing Value alone keeps the old tag,
		// so an empty (null) key became `owner: !!null craig`, which no
		// longer parses.
		mp.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
		return
	}
	setNode(mp, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

// setNode replaces key's value, or inserts the key where katra itself would
// write it: after the node-model keys that precede it in Frontmatter, so a
// migrated file reads the same as one katra created.
func setNode(mp *yaml.Node, key string, val *yaml.Node) {
	if i := mapIndex(mp, key); i >= 0 {
		mp.Content[i+1] = val
		return
	}
	order := []string{"type", "status", "spec", "effort", "horizon", "owner", "blocked_by"}
	at := len(mp.Content)
	for j := indexOf(order, key) - 1; j >= 0; j-- {
		if i := mapIndex(mp, order[j]); i >= 0 {
			at = i + 2
			break
		}
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	mp.Content = append(mp.Content[:at], append([]*yaml.Node{k, val}, mp.Content[at:]...)...)
}

func indexOf(list []string, s string) int {
	for i, x := range list {
		if x == s {
			return i
		}
	}
	return len(list)
}
