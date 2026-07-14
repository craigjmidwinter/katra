package core

// Agent-hooks-first state reconciliation (Phase 2).
//
// The reconcile layer answers one question at an agent checkpoint (the Stop
// hook, mainly): "the agent just produced a unit of work — has it been declared
// and are its memory generations resolved?" It is the *evaluator*; it never
// mutates repo state on its own and is fail-open everywhere. A katra bug here
// must never block the agent's real work.
//
// Identity: a "work generation" is fingerprinted from the code paths currently
// changed in the working tree (non-store) plus their content hashes. It is
// therefore stable across repeated Stop fires for the same unresolved work, and
// changes when the code changes again — and, crucially, it is recomputable from
// repo state alone, so the out-of-band `katra reconcile` process (a separate
// invocation with no hook context) derives the same id and its receipt matches.
//
// (Judgment call / minor deviation from spec §D: memory generation ids are NOT
// folded into the work-generation id. Baking them in would make the id shift as
// the agent resolves memory, stranding the receipt written moments earlier.
// Memory is instead tracked as a separate obligation dimension checked against
// the ledger at evaluation time — which keeps receipt matching stable and the
// Blocking set cleanly composable.)

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// --- report types -----------------------------------------------------------

// ReconcileReport is the evaluator's verdict for the current checkpoint.
type ReconcileReport struct {
	Applicable       bool               // false → allow immediately (no store / eval error)
	WorkGenerationID string             // fingerprint of the current unit of work
	TouchedPaths     []string           // real (non-store) code paths changed in the working tree
	Draft            DraftState         // active draft, if any
	Memory           []MemoryObligation // unresolved (pending/offered) generations
	Task             TaskDisposition    // declared advance/close/none/skip, or unknown
	Actions          []MechanicalAction // deterministic follow-ups already applied/available
	Warnings         []Issue            // quarantine hits etc. (warn, don't block)
	Blocking         []Issue            // every missing requirement, listed at once
}

// DraftState describes the active draft (if any) at a checkpoint.
type DraftState struct {
	HasDraft bool
	Slug     string
}

// MemoryObligation is one unresolved memory generation the agent must resolve.
type MemoryObligation struct {
	ID     string
	Path   string
	State  string
	Change string
}

// TaskDisposition is the agent's declared relationship to a task for this work.
type TaskDisposition struct {
	Kind   string // advance | close | none | skip | unknown
	Slug   string
	Reason string
}

// MechanicalAction is a deterministic follow-up (informational in the report).
type MechanicalAction struct {
	Kind   string
	Target string
	Detail string
}

// Issue is a single warning or blocking requirement.
type Issue struct {
	Kind    string // task | memory | quarantine
	Message string
}

// --- on-disk ledgers ---------------------------------------------------------

const agentStateVersion = 1

// sessionTouch is one Edit/Write observation within a turn.
type sessionTouch struct {
	Path string `json:"path"`           // repo-relative when resolvable, else absolute
	Hash string `json:"hash,omitempty"` // content hash at observation time (audit only)
}

// agentSession accumulates per-session_id hook state.
type agentSession struct {
	SessionID    string                  `json:"sessionId"`
	Turn         int                     `json:"turn"`
	BaselineHead string                  `json:"baselineHead,omitempty"`
	Touched      map[string]sessionTouch `json:"touched,omitempty"` // key: tool_use_id (deduped)
	// BlockedFingerprint is the fingerprint of the obligation-set the Stop gate
	// last blocked on for this session (work-generation id + the blocking
	// obligations). Stop suppresses a repeat block only when the *whole*
	// fingerprint matches, so genuinely new obligations (e.g. freshly discovered
	// memory) still surface (§ fix #9).
	BlockedFingerprint string `json:"blockedFingerprint,omitempty"`
	UpdatedAt          string `json:"updatedAt"`
}

type sessionLedger struct {
	Version  int                      `json:"version"`
	Sessions map[string]*agentSession `json:"sessions"`
}

// TaskDecl is the agent's task declaration recorded on a receipt.
type TaskDecl struct {
	Kind   string `json:"kind,omitempty"` // advance | close | none
	Slug   string `json:"slug,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ReconcileReceipt records the agent's declaration for one work generation.
type ReconcileReceipt struct {
	WorkGenerationID string   `json:"workGenerationId"`
	TouchedPaths     []string `json:"touchedPaths,omitempty"`
	Task             TaskDecl `json:"task"`
	Skip             bool     `json:"skip,omitempty"`
	SkipReason       string   `json:"skipReason,omitempty"`
	ResolvedMemory   []string `json:"resolvedMemory,omitempty"`
	CreatedAt        string   `json:"createdAt"`
}

type receiptLedger struct {
	Version  int                `json:"version"`
	Receipts []ReconcileReceipt `json:"receipts"`
}

func (s *Store) sessionLedgerPath() string {
	return filepath.Join(s.StateDir(), "agent-sessions.json")
}
func (s *Store) receiptLedgerPath() string {
	return filepath.Join(s.StateDir(), "reconcile-receipts.json")
}

// ReceiptsExist reports whether the reconcile-receipts ledger is present. The
// pre-commit coverage gate only enforces once the reconcile system is actually
// in use in a repo, so a repo that has never reconciled is never blocked by it.
func (s *Store) ReceiptsExist() bool {
	return fileExists(s.receiptLedgerPath())
}

// withStateLock serializes read-modify-write of the private state files across
// overlapping hook processes via an advisory flock. Fail-open: if the lock can't
// be taken, it proceeds anyway rather than blocking the agent.
func (s *Store) withStateLock(fn func() error) error {
	if err := os.MkdirAll(s.StateDir(), 0o755); err != nil {
		return fn()
	}
	_ = s.ensureStateIgnored()
	f, err := os.OpenFile(filepath.Join(s.StateDir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fn()
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fn()
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func (s *Store) loadSessions() (*sessionLedger, error) {
	b, err := os.ReadFile(s.sessionLedgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &sessionLedger{Version: agentStateVersion, Sessions: map[string]*agentSession{}}, nil
		}
		return nil, err
	}
	var l sessionLedger
	if err := json.Unmarshal(b, &l); err != nil {
		// Corrupt ledger: start fresh rather than fail a hook.
		return &sessionLedger{Version: agentStateVersion, Sessions: map[string]*agentSession{}}, nil
	}
	if l.Sessions == nil {
		l.Sessions = map[string]*agentSession{}
	}
	l.Version = agentStateVersion
	return &l, nil
}

func (s *Store) saveSessions(l *sessionLedger) error {
	return s.writeStateJSON(s.sessionLedgerPath(), "agent-sessions-*.tmp", l)
}

// loadReceipts loads the receipt ledger. A missing ledger is an empty ledger
// (nil error). An unreadable or corrupt ledger returns an error so callers can
// fail OPEN (treat coverage as satisfied / evaluation as non-applicable) rather
// than mistaking a corrupt file for "no declaration" and blocking (§ fix #7).
func (s *Store) loadReceipts() (*receiptLedger, error) {
	b, err := os.ReadFile(s.receiptLedgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &receiptLedger{Version: agentStateVersion}, nil
		}
		return nil, err
	}
	var l receiptLedger
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("corrupt reconcile-receipts ledger: %w", err)
	}
	l.Version = agentStateVersion
	return &l, nil
}

func (s *Store) saveReceipts(l *receiptLedger) error {
	return s.writeStateJSON(s.receiptLedgerPath(), "reconcile-receipts-*.tmp", l)
}

// writeStateJSON atomically writes a state file (temp + rename), ensuring the
// state dir exists and is git-ignored.
func (s *Store) writeStateJSON(path, pattern string, v any) error {
	if err := os.MkdirAll(s.StateDir(), 0o755); err != nil {
		return err
	}
	if err := s.ensureStateIgnored(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(s.StateDir(), pattern)
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
	return os.Rename(tmpName, path)
}

// --- session recording (hook side effects) ----------------------------------

func sessionKey(id string) string {
	if id == "" {
		return "default"
	}
	return id
}

// RecordTurnStart marks a new turn boundary for a session and clears the touched
// set so "was there Edit/Write this turn?" resets. Fail-open.
func (s *Store) RecordTurnStart(sessionID string) error {
	return s.withStateLock(func() error {
		l, err := s.loadSessions()
		if err != nil {
			return err
		}
		key := sessionKey(sessionID)
		sess := l.Sessions[key]
		if sess == nil {
			sess = &agentSession{SessionID: key}
			l.Sessions[key] = sess
		}
		sess.Turn++
		sess.Touched = map[string]sessionTouch{}
		if head, err := s.HeadHash(); err == nil {
			sess.BaselineHead = head
		}
		sess.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return s.saveSessions(l)
	})
}

// RecordTouched records an Edit/Write on absPath under toolUseID, deduped so a
// retried tool call does not double-count. Fail-open.
func (s *Store) RecordTouched(sessionID, toolUseID, absPath string) error {
	return s.withStateLock(func() error {
		l, err := s.loadSessions()
		if err != nil {
			return err
		}
		key := sessionKey(sessionID)
		sess := l.Sessions[key]
		if sess == nil {
			sess = &agentSession{SessionID: key}
			l.Sessions[key] = sess
		}
		if sess.Touched == nil {
			sess.Touched = map[string]sessionTouch{}
		}
		dedupe := toolUseID
		if dedupe == "" {
			dedupe = absPath
		}
		t := sessionTouch{Path: s.toRepoRel(absPath)}
		if h, ok := hashFile(absPath); ok {
			t.Hash = h
		}
		sess.Touched[dedupe] = t
		sess.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return s.saveSessions(l)
	})
}

// SessionTouchedThisTurn returns the repo-relative code paths the agent authored
// (Edit/Write) in the current turn, excluding store paths. Empty means "no work
// authored this turn" → the Stop gate must allow.
func (s *Store) SessionTouchedThisTurn(sessionID string) []string {
	l, err := s.loadSessions()
	if err != nil {
		return nil
	}
	sess := l.Sessions[sessionKey(sessionID)]
	if sess == nil {
		return nil
	}
	return touchedCodePaths(s, sess)
}

// touchedCodePaths extracts the deduped, sorted non-store repo-relative paths a
// session authored this turn.
func touchedCodePaths(s *Store, sess *agentSession) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range sess.Touched {
		if t.Path == "" || seen[t.Path] || s.IsStorePath(t.Path) {
			continue
		}
		seen[t.Path] = true
		out = append(out, t.Path)
	}
	sort.Strings(out)
	return out
}

// mostRecentTouchedSession returns the session with the newest UpdatedAt that has
// authored at least one non-store path this turn, or nil. Used to reproduce the
// current turn's unit of work from a context with no session id (the standalone
// `katra reconcile` command).
func (s *Store) mostRecentTouchedSession() *agentSession {
	l, err := s.loadSessions()
	if err != nil {
		return nil
	}
	var best *agentSession
	for _, sess := range l.Sessions {
		if len(touchedCodePaths(s, sess)) == 0 {
			continue
		}
		if best == nil || sess.UpdatedAt > best.UpdatedAt {
			best = sess
		}
	}
	return best
}

// ClaimStopBlock is the atomic compare-and-set the Stop gate uses to decide, in a
// single locked operation, whether THIS process is the one that should emit the
// block for fingerprint (and if so, offer the pending memory in the same critical
// section). It returns true only when it transitioned the session's blocked
// fingerprint to fingerprint — so two concurrent Stops on the same obligation set
// never both block, and a repeat Stop on an unchanged set never re-nags.
//
// Fail-open: if the session state cannot be persisted, it returns false (allow)
// rather than blocking on a bookkeeping failure (§ fix #8).
func (s *Store) ClaimStopBlock(sessionID, workGenID, fingerprint string, pendingMemIDs []string) bool {
	won := false
	err := s.withStateLock(func() error {
		l, err := s.loadSessions()
		if err != nil {
			return err
		}
		key := sessionKey(sessionID)
		sess := l.Sessions[key]
		if sess == nil {
			sess = &agentSession{SessionID: key}
			l.Sessions[key] = sess
		}
		if sess.BlockedFingerprint == fingerprint {
			return nil // already blocked on this exact set → won stays false (allow)
		}
		sess.BlockedFingerprint = fingerprint
		sess.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.saveSessions(l); err != nil {
			return err
		}
		// Offer pending memory inside the same lock (best-effort: a failure here
		// does not un-win the block — we still want to nag once).
		_ = s.offerMemoryLocked(pendingMemIDs, workGenID)
		won = true
		return nil
	})
	if err != nil {
		return false // persistence failed → allow
	}
	return won
}

// --- receipts ----------------------------------------------------------------

// WriteReceipt records (or replaces) the receipt for a work generation.
func (s *Store) WriteReceipt(r ReconcileReceipt) error {
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return s.withStateLock(func() error {
		l, err := s.loadReceipts()
		if err != nil {
			return err
		}
		replaced := false
		for i := range l.Receipts {
			if l.Receipts[i].WorkGenerationID == r.WorkGenerationID {
				l.Receipts[i] = r
				replaced = true
				break
			}
		}
		if !replaced {
			l.Receipts = append(l.Receipts, r)
		}
		return s.saveReceipts(l)
	})
}

// FindReceipt returns the receipt for a work generation, or nil (also nil when
// the ledger is corrupt/unreadable — callers fail open).
func (s *Store) FindReceipt(workGenID string) *ReconcileReceipt {
	l, err := s.loadReceipts()
	if err != nil {
		return nil
	}
	return findReceipt(l, workGenID)
}

// receiptResolves reports whether a receipt satisfies the task-declaration
// requirement (a skip, or any advance/close/none declaration).
func receiptResolves(r *ReconcileReceipt) bool {
	return r != nil && (r.Skip || r.Task.Kind != "")
}

// --- evaluation --------------------------------------------------------------

// EvaluateReconcile computes the checkpoint verdict with no session context: the
// unit of work is reproduced from the most-recently-active session's touched set
// (so the standalone `katra reconcile` command derives the same work generation
// the Stop gate does), falling back to the repo-wide dirty set when no session
// has recorded work. See EvaluateReconcileForSession.
func (s *Store) EvaluateReconcile() ReconcileReport {
	return s.EvaluateReconcileForSession("")
}

// EvaluateReconcileForSession computes the checkpoint verdict for a given hook
// session. It is pure (no mutation) and fail-open: any store/git error, or a
// corrupt receipt ledger, yields a non-applicable report (→ allow).
//
// The unit of work is THIS turn's authored paths intersected with their net
// change vs the working tree (§ fix #4): a path counts only if the session
// touched it this turn AND it still differs from HEAD/index. An edit reverted
// within the turn nets nothing, and a pre-existing unrelated dirty file the
// session never touched is never pulled in — so Stop cannot block a net-no-change
// turn. When sessionID is "" the unit is reproduced from the most-recently-active
// session (or the repo-wide dirty set if none), keeping the standalone reconcile
// command's work-generation id in agreement with Stop's.
func (s *Store) EvaluateReconcileForSession(sessionID string) ReconcileReport {
	var r ReconcileReport
	root, err := s.RepoRoot()
	if err != nil {
		return r // not a git repo → not applicable → allow
	}
	dirty, err := s.DirtyEntries()
	if err != nil {
		return r
	}
	// Corrupt/unreadable receipt ledger → cannot evaluate → allow (§ fix #7).
	receipts, err := s.loadReceipts()
	if err != nil {
		return r // Applicable stays false
	}
	r.Applicable = true

	// Two views of "dirty". dirtyAll is everything the agent plausibly authored —
	// used when we can attribute the unit to observed Edit/Write calls, where a
	// newly written (still untracked) source file is genuinely work. dirtyTracked
	// excludes untracked files and backs the repo-wide fallback, so a large
	// untracked scratch tree (build artifacts, caches, media) is never mistaken
	// for the agent's work.
	dirtyAll := map[string]bool{}
	dirtyTracked := map[string]bool{}
	for _, e := range dirty {
		if !s.IsWorkProduct(e.Path) {
			continue
		}
		dirtyAll[e.Path] = true
		if !e.Untracked {
			dirtyTracked[e.Path] = true
		}
	}
	r.TouchedPaths = s.unitForSession(sessionID, dirtyAll, dirtyTracked)
	sort.Strings(r.TouchedPaths)
	r.WorkGenerationID = s.workGenIDWorkingTree(root, r.TouchedPaths)

	if d, _ := s.ActiveDraft(); d != nil {
		r.Draft = DraftState{HasDraft: true, Slug: d.Slug}
	}

	// Memory obligations + quarantine warnings.
	if gens, err := s.MemoryGenerations(); err == nil {
		for _, g := range gens {
			switch g.State {
			case MemPending, MemOffered:
				r.Memory = append(r.Memory, MemoryObligation{ID: g.ID, Path: g.Path, State: g.State, Change: g.Change})
			case MemQuarantined:
				r.Warnings = append(r.Warnings, Issue{Kind: "quarantine", Message: "quarantined memory " + g.Path + " (" + g.Reason + ")"})
			}
		}
	}

	receipt := findReceipt(receipts, r.WorkGenerationID)
	r.Task = dispositionFromReceipt(receipt)

	// A skip receipt resolves the whole checkpoint (task + memory) for this work.
	if receipt != nil && receipt.Skip {
		return r
	}
	if !receiptResolves(receipt) {
		r.Blocking = append(r.Blocking, Issue{Kind: "task",
			Message: "declare the task: `katra reconcile --advance <slug>` or `--close <slug>` (or `--no-task --reason \"…\"`)"})
	}
	for _, m := range r.Memory {
		r.Blocking = append(r.Blocking, Issue{Kind: "memory",
			Message: "resolve memory " + m.ID[:min(12, len(m.ID))] + "… (" + m.Path + "): add it to your draft then `katra memory resolve " + m.ID + " --imported`, or `katra memory ignore " + m.ID + " --reason \"…\"`"})
	}
	return r
}

func dispositionFromReceipt(r *ReconcileReceipt) TaskDisposition {
	if r == nil {
		return TaskDisposition{Kind: "unknown"}
	}
	if r.Skip {
		return TaskDisposition{Kind: "skip", Reason: r.SkipReason}
	}
	if r.Task.Kind == "" {
		return TaskDisposition{Kind: "unknown"}
	}
	return TaskDisposition{Kind: r.Task.Kind, Slug: r.Task.Slug, Reason: r.Task.Reason}
}

// CoverageSatisfied reports whether a receipt covers the work generation of the
// given repo-relative code paths, fingerprinted from their STAGED index blobs.
// Used by the pre-commit coverage gate. Fail-open: a corrupt/unreadable receipt
// ledger is treated as satisfied so a bookkeeping failure never blocks a commit
// (§ fix #7).
func (s *Store) CoverageSatisfied(paths []string) bool {
	l, err := s.loadReceipts()
	if err != nil {
		return true // corrupt receipts → don't block the commit
	}
	return receiptResolves(findReceipt(l, s.CoverageReceiptID(paths)))
}

// CoverageReceiptID computes the work-generation id for a set of repo-relative
// code paths from their STAGED index blobs (not the working tree) — the exact
// bytes a commit will record. This is the pre-commit counterpart to the Stop
// gate's working-tree id; the two agree when the staged content equals the
// working tree (the normal reconcile-then-commit flow), because both hash the
// same canonical change record keyed on the git blob id (§ fix #2, #10).
func (s *Store) CoverageReceiptID(paths []string) string {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	recs := s.indexChangeRecords(sorted)
	return fingerprintChangeRecords(recs)
}

// --- work-generation identity ------------------------------------------------

// workGenIDWorkingTree fingerprints a unit of work from its changed code paths,
// keyed on a canonical change record per path (operation, mode, HEAD blob id, and
// working-tree blob id). Deterministic and content-sensitive: changing content,
// mode, or the operation (add/modify/delete) changes the id; a no-op
// re-observation keeps it stable. Distinguishing deletions, chmods, and
// return-to-prior-bytes avoids the collisions of a bare content hash (§ fix #10).
func (s *Store) workGenIDWorkingTree(root string, paths []string) string {
	recs := s.workingTreeChangeRecords(root, paths)
	return fingerprintChangeRecords(recs)
}

// unitForSession derives the unit of work for a checkpoint: the paths the session
// authored this turn intersected with the currently net-dirty code set (§ fix
// #4). A path the session never touched (pre-existing dirt) or one it edited then
// reverted (no longer dirty) is excluded. With sessionID "" it reproduces the
// most-recently-active session's unit so the standalone reconcile command agrees
// with Stop; with no touched session at all it falls back to the repo-wide dirty
// set (legacy/direct callers).
// maxUntrackedFallback bounds how many untracked files the repo-wide fallback
// will treat as a unit of work. Above this, the untracked set is assumed to be a
// scratch/artifact tree rather than something the agent just authored.
const maxUntrackedFallback = 50

func (s *Store) unitForSession(sessionID string, dirtyAll, dirtyTracked map[string]bool) []string {
	// Attributed to observed Edit/Write calls: keep untracked files — a newly
	// written source file is untracked but is real work.
	intersect := func(touched []string) []string {
		out := make([]string, 0, len(touched))
		for _, p := range touched {
			if dirtyAll[p] {
				out = append(out, p)
			}
		}
		return out
	}
	if sessionID != "" {
		// An explicit session with no authored work → empty unit (allow).
		return intersect(s.SessionTouchedThisTurn(sessionID))
	}
	if sess := s.mostRecentTouchedSession(); sess != nil {
		return intersect(touchedCodePaths(s, sess))
	}
	// No session context at all: we cannot attribute anything, so guess from the
	// repo. Tracked changes always count. Untracked files are ambiguous — a newly
	// written source file is real work, but a repo may also carry a large
	// untracked scratch tree (build artifacts, caches, media). Include them only
	// when there are few enough to plausibly BE a unit of work; a repo sitting on
	// thousands of untracked artifacts is not "work the agent just did", and
	// hashing them would be both meaningless and slow.
	var untracked []string
	for p := range dirtyAll {
		if !dirtyTracked[p] {
			untracked = append(untracked, p)
		}
	}
	out := make([]string, 0, len(dirtyTracked)+len(untracked))
	for p := range dirtyTracked {
		out = append(out, p)
	}
	if len(untracked) <= maxUntrackedFallback {
		out = append(out, untracked...)
	}
	return out
}

// findReceipt returns the receipt for workGenID from an already-loaded ledger.
func findReceipt(l *receiptLedger, workGenID string) *ReconcileReceipt {
	if l == nil {
		return nil
	}
	for i := range l.Receipts {
		if l.Receipts[i].WorkGenerationID == workGenID {
			r := l.Receipts[i]
			return &r
		}
	}
	return nil
}

// BlockingFingerprint fingerprints the obligation-set a Stop block is about: the
// work-generation id plus the identities of every blocking obligation (the task
// requirement and each pending/offered memory id). Stop suppresses a repeat block
// only when this whole fingerprint is unchanged, so a newly discovered obligation
// re-surfaces even at the same work generation (§ fix #9).
func BlockingFingerprint(r ReconcileReport) string {
	h := sha256.New()
	h.Write([]byte(r.WorkGenerationID))
	h.Write([]byte{0})
	ids := make([]string, 0, len(r.Blocking))
	for _, b := range r.Blocking {
		ids = append(ids, b.Kind+":"+b.Message)
	}
	sort.Strings(ids)
	for _, id := range ids {
		h.Write([]byte(id))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// changeRecord is the canonical identity of one path's change, independent of
// whether it is sourced from the working tree or the staged index. Two changes
// with the same record are the same unit of work; any difference in operation,
// mode, baseline, or new content yields a different record (§ fix #10).
type changeRecord struct {
	Path     string // repo-relative, slash-separated
	Op       string // A (add) | M (modify) | D (delete)
	Mode     string // git file mode, e.g. 100644 / 100755 ("" for a deletion)
	BaseBlob string // git blob id at HEAD ("" when absent at HEAD)
	Blob     string // git blob id of the new content ("" for a deletion)
}

// fingerprintChangeRecords hashes a set of change records into a stable
// work-generation id. Records are sorted by path so ordering never affects the
// id.
func fingerprintChangeRecords(recs []changeRecord) string {
	sort.Slice(recs, func(i, j int) bool { return recs[i].Path < recs[j].Path })
	h := sha256.New()
	for _, rc := range recs {
		for _, field := range []string{rc.Path, rc.Op, rc.Mode, rc.BaseBlob, rc.Blob} {
			h.Write([]byte(field))
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// hashFile returns the sha256 of a file's content. ok is false when unreadable.
func hashFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return sha256Hex(b), true
}

// toRepoRel converts an absolute path to a repo-root-relative slash path when it
// lies inside the repo; otherwise it returns the input unchanged.
func (s *Store) toRepoRel(p string) string {
	if p == "" {
		return ""
	}
	root, err := s.RepoRoot()
	if err != nil {
		return p
	}
	abs := p
	if !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		}
	}
	if real, e := filepath.EvalSymlinks(abs); e == nil {
		abs = real
	}
	if real, e := filepath.EvalSymlinks(root); e == nil {
		root = real
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return filepath.ToSlash(rel)
}
