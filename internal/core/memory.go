package core

// Memory ingest: the deterministic first transition of the memory pipeline.
//
// Claude Code writes native per-project memory at
//   ~/.claude/projects/<ENCODED>/memory/*.md
// where <ENCODED> is the repo's absolute path with every "/" replaced by "-"
// (e.g. /Users/craig/workspace/devlog -> -Users-craig-workspace-devlog).
//
// Ingest reads those files, admits only the configured metadata.type values
// (default: "project"), and records each observed content change as a
// "generation" in a private, git-ignored ledger at
//   <store>/.state/memory-ledger.json
//
// Ingest is ONE of three separate transitions (ingest -> author -> publish). It
// deliberately does NOT create entries or drafts — that belongs to the author
// phase. Ingest only populates the ledger. Everything here is fail-open: any
// uncertainty or error yields an empty/no-op result rather than a hard failure.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// memoryPrefixLen is the fixed prefix length (bytes) hashed for append
// detection: if the prefix hash is unchanged and the file grew, the change is
// an append; otherwise it's a rewrite.
const memoryPrefixLen = 512

// ledgerVersion is the on-disk schema version of the memory ledger.
const ledgerVersion = 1

// Generation resolution states.
const (
	MemPending     = "pending"
	MemOffered     = "offered"
	MemImported    = "imported"
	MemIgnored     = "ignored"
	MemQuarantined = "quarantined"
)

// Change classifications for a generation.
const (
	ChangeNew       = "new"
	ChangeAppended  = "appended"
	ChangeRewritten = "rewritten"
)

// MemoryGeneration is one observed content-state of a memory file. Its ID is a
// content hash, so re-observing identical content is a no-op (idempotent).
type MemoryGeneration struct {
	ID           string `json:"id"`               // sha256(path\x00content) — stable per content
	Path         string `json:"path"`             // basename within the memory dir
	Size         int64  `json:"size"`             // byte size of the full content
	SHA256       string `json:"sha256"`           // hash of the full content
	PrefixSHA256 string `json:"prefixSha256"`     // hash of the fixed-length prefix
	Change       string `json:"change"`           // new | appended | rewritten
	State        string `json:"state"`            // pending | offered | imported | ignored | quarantined
	Reason       string `json:"reason,omitempty"` // quarantine/ignore reason
	Type         string `json:"type,omitempty"`   // admitted metadata.type
	DetectedAt   string `json:"detectedAt"`       // RFC3339 UTC
	// OfferedAt is the work-generation id at which the gate first offered this
	// generation to the agent (set on the pending -> offered transition).
	OfferedAt string `json:"offeredAt,omitempty"`
}

// memoryLedger is the private, git-ignored delta ledger.
type memoryLedger struct {
	Version     int                `json:"version"`
	Generations []MemoryGeneration `json:"generations"`
}

// ScanResult summarizes a single ingest pass.
type ScanResult struct {
	New         int    // generations classified new this pass
	Appended    int    // generations classified appended this pass
	Rewritten   int    // generations classified rewritten this pass
	Quarantined int    // generations quarantined this pass
	MemoryDir   string // the resolved Claude memory dir (may not exist)
	Skipped     bool   // ingest disabled or memory dir absent — nothing scanned
}

// StateDir is the private per-store state directory (git-ignored).
func (s *Store) StateDir() string { return filepath.Join(s.Dir, ".state") }

// ledgerPath is the on-disk path of the memory ledger.
func (s *Store) ledgerPath() string { return filepath.Join(s.StateDir(), "memory-ledger.json") }

// encodeProjectPath maps an absolute repo path to its Claude Code project dir
// name: every "/" becomes "-". So "/Users/craig/workspace/devlog" becomes
// "-Users-craig-workspace-devlog".
func encodeProjectPath(abs string) string {
	return strings.ReplaceAll(abs, string(filepath.Separator), "-")
}

// claudeMemoryDir returns ~/.claude/projects/<encoded>/memory for a repo path.
func claudeMemoryDir(repoAbs string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects", encodeProjectPath(repoAbs), "memory"), nil
}

// MemoryDir resolves this store's Claude Code memory directory. It tries the
// repo root (git), then symlink-resolved variants, returning the first that
// exists. If none exist it returns the primary candidate and false — callers
// treat a missing dir as "nothing to ingest", not an error.
func (s *Store) MemoryDir() (dir string, exists bool) {
	var candidates []string
	add := func(repo string) {
		if repo == "" {
			return
		}
		if abs, err := filepath.Abs(repo); err == nil {
			if md, err := claudeMemoryDir(abs); err == nil {
				candidates = append(candidates, md)
			}
		}
		if real, err := filepath.EvalSymlinks(repo); err == nil && real != repo {
			if md, err := claudeMemoryDir(real); err == nil {
				candidates = append(candidates, md)
			}
		}
	}
	if root, err := s.RepoRoot(); err == nil {
		add(root)
	}
	add(filepath.Dir(s.Dir)) // fallback: the dir containing the store

	if len(candidates) == 0 {
		return "", false
	}
	for _, c := range candidates {
		if dirExists(c) {
			return c, true
		}
	}
	return candidates[0], false
}

// ScanMemory runs an ingest pass: it reads admitted memory files, classifies
// each observed content change as a generation, quarantines likely-secret
// content, and persists the ledger atomically. It is idempotent — a re-scan
// with no memory changes yields no new generations. Fail-open throughout.
//
// The load→modify→save of the ledger runs under the shared state lock so a
// concurrent Stop/PostToolUse cannot lose updates (§ fix #1).
func (s *Store) ScanMemory() (ScanResult, error) {
	var res ScanResult
	if !s.Config.memoryEnabled() {
		res.Skipped = true
		return res, nil
	}
	dir, exists := s.MemoryDir()
	res.MemoryDir = dir
	if !exists {
		res.Skipped = true
		return res, nil
	}
	err := s.withStateLock(func() error {
		return s.scanMemoryLocked(dir, &res)
	})
	return res, err
}

// scanMemoryLocked performs the ledger read-modify-write of a scan pass. The
// caller MUST already hold the state lock (it uses the unlocked load/save
// helpers) — never call it directly from a locked context that would re-acquire.
func (s *Store) scanMemoryLocked(dir string, res *ScanResult) error {
	ledger, err := s.loadLedger()
	if err != nil {
		// Corrupt/unreadable ledger: start fresh rather than fail the scan.
		ledger = &memoryLedger{Version: ledgerVersion}
	}
	latest := latestByPath(ledger.Generations)
	known := idSet(ledger.Generations)

	ents, err := os.ReadDir(dir)
	if err != nil {
		res.Skipped = true
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var added []MemoryGeneration
	for _, de := range ents {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if name == "MEMORY.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		full := filepath.Join(dir, name)
		content, err := os.ReadFile(full)
		if err != nil {
			continue // unreadable file: skip, don't fail
		}
		mtype := memoryType(content)
		if !s.Config.memoryAdmits(mtype) {
			continue
		}

		fullHash := sha256Hex(content)
		id := generationID(name, content)
		if known[id] {
			continue // identical content already recorded — idempotent no-op
		}

		gen := MemoryGeneration{
			ID:           id,
			Path:         name,
			Size:         int64(len(content)),
			SHA256:       fullHash,
			PrefixSHA256: sha256Hex(prefixBytes(content, memoryPrefixLen)),
			Type:         mtype,
			DetectedAt:   now,
		}
		gen.Change = classifyChange(latest[name], content)

		if reason := s.quarantineReason(content); reason != "" {
			gen.State = MemQuarantined
			gen.Reason = reason
			res.Quarantined++
		} else {
			gen.State = MemPending
		}

		switch gen.Change {
		case ChangeNew:
			res.New++
		case ChangeAppended:
			res.Appended++
		case ChangeRewritten:
			res.Rewritten++
		}

		added = append(added, gen)
		known[id] = true
		g := gen
		latest[name] = &g
	}

	if len(added) == 0 {
		return nil // nothing changed: don't rewrite the ledger
	}
	ledger.Version = ledgerVersion
	ledger.Generations = append(ledger.Generations, added...)
	return s.saveLedger(ledger)
}

// MemoryGenerations returns every generation recorded in the ledger (oldest
// first). A missing ledger is an empty slice, not an error.
func (s *Store) MemoryGenerations() ([]MemoryGeneration, error) {
	ledger, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	return ledger.Generations, nil
}

// ResolveMemory sets the resolution state of a specific generation. Matching is
// tried in order: exact generation id, then a unique hex id prefix, then a file
// basename. The basename fallback selects the newest UNRESOLVED generation (so a
// rewritten file's older pending generation is never stranded by a newer,
// already-resolved one; § fix #11), and only if none are unresolved does it fall
// back to the newest generation overall. state must be one of the Mem* constants.
// The read-modify-write runs under the shared state lock (§ fix #1).
func (s *Store) ResolveMemory(ref, state, reason string) (*MemoryGeneration, error) {
	var out *MemoryGeneration
	err := s.withStateLock(func() error {
		g, e := s.resolveMemoryLocked(ref, state, reason)
		out = g
		return e
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// resolveMemoryLocked performs the ledger RMW for ResolveMemory. The caller MUST
// already hold the state lock.
func (s *Store) resolveMemoryLocked(ref, state, reason string) (*MemoryGeneration, error) {
	ledger, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	idx := s.matchGeneration(ledger.Generations, ref)
	if idx < 0 {
		return nil, os.ErrNotExist
	}
	ledger.Generations[idx].State = state
	ledger.Generations[idx].Reason = reason
	if err := s.saveLedger(ledger); err != nil {
		return nil, err
	}
	g := ledger.Generations[idx]
	return &g, nil
}

// matchGeneration resolves ref to a generation index, or -1. See ResolveMemory
// for the matching order. A ref that matches more than one id prefix is
// ambiguous → -1 (no match), so the caller reports "not found" rather than
// silently resolving the wrong one.
func (s *Store) matchGeneration(gens []MemoryGeneration, ref string) int {
	// 1. Exact generation-id match.
	for i := range gens {
		if gens[i].ID == ref {
			return i
		}
	}
	// 2. Unique hex id prefix (only when ref looks like an id fragment).
	if len(ref) >= 6 && isHex(ref) {
		match := -1
		for i := range gens {
			if strings.HasPrefix(gens[i].ID, ref) {
				if match >= 0 {
					match = -1 // ambiguous
					break
				}
				match = i
			}
		}
		if match >= 0 {
			return match
		}
	}
	// 3. Basename fallback: newest UNRESOLVED, else newest overall.
	base := filepath.Base(ref)
	newestUnresolved, newestAny := -1, -1
	for i := range gens {
		if gens[i].Path != base {
			continue
		}
		newestAny = i
		if gens[i].State == MemPending || gens[i].State == MemOffered {
			newestUnresolved = i
		}
	}
	if newestUnresolved >= 0 {
		return newestUnresolved
	}
	return newestAny
}

// isHex reports whether s is non-empty and all lowercase hex digits.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// MemoryObligations returns the generations that still require the agent's
// attention before a checkpoint is clean: those pending or already offered.
// Quarantined generations are surfaced as warnings elsewhere, not obligations.
func (s *Store) MemoryObligations() ([]MemoryGeneration, error) {
	gens, err := s.MemoryGenerations()
	if err != nil {
		return nil, err
	}
	var out []MemoryGeneration
	for _, g := range gens {
		if g.State == MemPending || g.State == MemOffered {
			out = append(out, g)
		}
	}
	return out, nil
}

// OfferMemory records that the gate has offered the given pending generations to
// the agent at workGenID, moving them pending -> offered. Already-offered or
// resolved generations are left untouched. Fail-open: a missing ledger is a
// no-op.
func (s *Store) OfferMemory(ids []string, workGenID string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.withStateLock(func() error {
		return s.offerMemoryLocked(ids, workGenID)
	})
}

// offerMemoryLocked performs the ledger RMW for OfferMemory. The caller MUST
// already hold the state lock — the Stop compare-and-set drives it from within
// its own locked section to keep block + offer a single atomic op (§ fix #8).
func (s *Store) offerMemoryLocked(ids []string, workGenID string) error {
	if len(ids) == 0 {
		return nil
	}
	ledger, err := s.loadLedger()
	if err != nil {
		return err
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	changed := false
	for i := range ledger.Generations {
		g := &ledger.Generations[i]
		if want[g.ID] && g.State == MemPending {
			g.State = MemOffered
			g.OfferedAt = workGenID
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveLedger(ledger)
}

// --- ledger persistence -----------------------------------------------------

func (s *Store) loadLedger() (*memoryLedger, error) {
	b, err := os.ReadFile(s.ledgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &memoryLedger{Version: ledgerVersion}, nil
		}
		return nil, err
	}
	var l memoryLedger
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	if l.Version == 0 {
		l.Version = ledgerVersion
	}
	return &l, nil
}

// saveLedger writes the ledger atomically (temp + rename) and ensures the
// state dir is git-ignored so the ledger stays local.
func (s *Store) saveLedger(l *memoryLedger) error {
	if err := os.MkdirAll(s.StateDir(), 0o755); err != nil {
		return err
	}
	if err := s.ensureStateIgnored(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(s.StateDir(), "memory-ledger-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, s.ledgerPath())
}

// ensureStateIgnored makes sure `.state/` is excluded from git via the store's
// own .gitignore so the private ledger never gets committed.
func (s *Store) ensureStateIgnored() error {
	giPath := filepath.Join(s.Dir, ".gitignore")
	existing, _ := os.ReadFile(giPath)
	for _, line := range strings.Split(string(existing), "\n") {
		switch strings.TrimSpace(line) {
		case ".state/", ".state", "/.state/", "/.state":
			return nil // already ignored
		}
	}
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += ".state/\n"
	return os.WriteFile(giPath, []byte(out), 0o644)
}

// --- classification & detection helpers ------------------------------------

// classifyChange compares new content against the last-known generation for the
// same path. It is "appended" only when the previous full content is an exact
// byte-prefix of the new content and the file grew — an exact test that holds
// for files of any size (the fixed-length prefix hash stored on the generation
// is the coarse audit fingerprint; this is the precise classifier). Any other
// difference is a rewrite.
func classifyChange(prev *MemoryGeneration, content []byte) string {
	if prev == nil {
		return ChangeNew
	}
	if int64(len(content)) > prev.Size && sha256Hex(content[:prev.Size]) == prev.SHA256 {
		return ChangeAppended
	}
	return ChangeRewritten
}

// memoryType extracts metadata.type from a memory file's YAML frontmatter.
// Missing frontmatter or type yields "".
func memoryType(content []byte) string {
	block := extractFrontmatter(content)
	if block == nil {
		return ""
	}
	var fm struct {
		Metadata struct {
			Type string `yaml:"type"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(block, &fm); err != nil {
		return ""
	}
	return fm.Metadata.Type
}

// extractFrontmatter returns the raw YAML between the leading `---` and its
// closing `---`, or nil when there is no frontmatter block.
func extractFrontmatter(b []byte) []byte {
	if !strings.HasPrefix(string(b), "---") {
		return nil
	}
	// Skip the opening delimiter line.
	nl := strings.IndexByte(string(b), '\n')
	if nl < 0 {
		return nil
	}
	rest := b[nl+1:]
	idx := strings.Index(string(rest), "\n---")
	if idx < 0 {
		return nil
	}
	return rest[:idx]
}

// --- quarantine (safety net, not a scrub) ----------------------------------

var secretPatterns = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PRIVATE KEY-----`), "private key block"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "AWS access key id"},
	{regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-._~+/]{20,}=*`), "bearer token"},
	{regexp.MustCompile(`\bghp_[A-Za-z0-9]{20,}\b`), "GitHub token"},
	{regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), "Slack token"},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`), "API secret key"},
	// KEY=VALUE where KEY looks secret.
	{regexp.MustCompile(`(?i)\b(?:[A-Z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSWD|API[_-]?KEY|PRIVATE[_-]?KEY|ACCESS[_-]?KEY|CLIENT[_-]?SECRET))[A-Z0-9_]*\s*[:=]\s*['"]?[^\s'"]{8,}`), "secret-looking KEY=VALUE"},
}

// quarantineReason returns a non-empty reason when content should be
// quarantined: a built-in secret detector fires, or a configured sensitive term
// appears (case-insensitive). Quarantine blocks import; it never redacts.
func (s *Store) quarantineReason(content []byte) string {
	text := string(content)
	for _, p := range secretPatterns {
		if p.re.MatchString(text) {
			return "possible secret: " + p.reason
		}
	}
	lower := strings.ToLower(text)
	for _, term := range s.Config.Memory.SensitiveTerms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(term)) {
			return "sensitive term: " + term
		}
	}
	return ""
}

// --- small utilities --------------------------------------------------------

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func generationID(path string, content []byte) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func prefixBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}

// latestByPath returns the newest (last-appended) generation per path.
func latestByPath(gens []MemoryGeneration) map[string]*MemoryGeneration {
	m := make(map[string]*MemoryGeneration, len(gens))
	for i := range gens {
		g := gens[i]
		m[g.Path] = &g
	}
	return m
}

func idSet(gens []MemoryGeneration) map[string]bool {
	m := make(map[string]bool, len(gens))
	for _, g := range gens {
		m[g.ID] = true
	}
	return m
}
