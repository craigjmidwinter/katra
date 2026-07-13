package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// hookInput is the subset of Claude Code's hook stdin JSON katra reads. Every
// field is optional across events; absence is handled fail-open.
type hookInput struct {
	SessionID      string `json:"session_id"`
	StopHookActive bool   `json:"stop_hook_active"`
	HookEventName  string `json:"hook_event_name"`
	ToolName       string `json:"tool_name"`
	ToolUseID      string `json:"tool_use_id"`
	ToolInput      struct {
		FilePath string `json:"file_path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
}

// errNoHookInput signals empty or unparseable hook stdin. Every hook event
// treats it as fail-open: allow (never block, never act on a zero-value input
// that would masquerade as the shared "default" session) (§ fix #13).
var errNoHookInput = errors.New("no hook input")

func readHookInput() (hookInput, error) {
	var in hookInput
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return in, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return in, errNoHookInput
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return in, err
	}
	return in, nil
}

// agentHookCmd is katra's single hidden adapter for Claude Code hooks. One
// command surface, event dispatched by the first argument. Every path is
// fail-open: a katra error must never block the agent's real work. Only the
// `stop` and `pre-commit` events can ever block, and only on a real, unresolved
// unit of work.
func agentHookCmd() *cobra.Command {
	var event string
	cmd := &cobra.Command{
		Use:    "agent-hook <event>",
		Short:  "(internal) Claude Code hook adapter (session-start|turn-start|post-tool|stop|snapshot|pre-commit)",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ev := ""
			if len(args) == 1 {
				ev = args[0]
			}
			switch ev {
			case "session-start":
				return hookSessionStartRun()
			case "turn-start":
				return hookTurnStartRun()
			case "post-tool":
				return hookPostToolRun()
			case "stop":
				return hookStopRun()
			case "snapshot":
				return hookSnapshotRun()
			case "pre-commit":
				return hookPreCommitRun()
			default:
				// Unknown event: allow, never error a hook.
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "sub-event (e.g. pre-compact|session-end for snapshot)")
	return cmd
}

// session-start: inject a short summary of unresolved/offered state. Never blocks.
func hookSessionStartRun() error {
	if _, err := readHookInput(); err != nil {
		return nil
	}
	s, err := resolveStore()
	if err != nil {
		return nil
	}
	_, _ = s.ScanMemory() // pick up anything written since last session
	var lines []string
	if obs, err := s.MemoryObligations(); err == nil && len(obs) > 0 {
		lines = append(lines, fmt.Sprintf("📓 katra: %d unresolved memory generation(s) — review with `katra memory status`", len(obs)))
	}
	r := s.EvaluateReconcile()
	if r.Applicable && len(r.TouchedPaths) > 0 && len(r.Blocking) > 0 {
		lines = append(lines, "📓 katra: in-flight code changes need reconciliation — `katra reconcile status`")
	} else if r.Draft.HasDraft {
		lines = append(lines, fmt.Sprintf("📓 katra: active draft %q — keep logging as you build", r.Draft.Slug))
	}
	if len(lines) > 0 {
		fmt.Println(strings.Join(lines, "\n"))
	}
	return nil
}

// turn-start (UserPromptSubmit): note a new turn boundary. Never blocks.
func hookTurnStartRun() error {
	in, err := readHookInput()
	if err != nil {
		return nil
	}
	s, err := resolveStore()
	if err != nil {
		return nil
	}
	_ = s.RecordTurnStart(in.SessionID)
	return nil
}

// post-tool (PostToolUse, matcher Edit|Write): record the touched path and
// incrementally rescan memory. Never blocks. Cheap.
func hookPostToolRun() error {
	in, err := readHookInput()
	if err != nil {
		return nil
	}
	if in.ToolName != "Edit" && in.ToolName != "Write" && in.ToolName != "MultiEdit" {
		return nil
	}
	s, err := resolveStore()
	if err != nil {
		return nil
	}
	if in.ToolInput.FilePath != "" {
		abs := in.ToolInput.FilePath
		if !filepath.IsAbs(abs) {
			if wd, err := os.Getwd(); err == nil {
				abs = filepath.Join(wd, abs)
			}
		}
		_ = s.RecordTouched(in.SessionID, in.ToolUseID, abs)
	}
	_, _ = s.ScanMemory()
	return nil
}

// hookStopRun is the primary gate. It emits {"decision":"block","reason":...} to
// block, or nothing to allow. NEVER blocks a normal conversational turn.
func hookStopRun() error {
	in, err := readHookInput()
	if err != nil {
		return nil // malformed/empty input → allow (§ fix #13)
	}
	s, err := resolveStore()
	if err != nil {
		return nil // not a katra repo → allow
	}
	block, reason := stopDecision(s, in)
	if !block {
		return nil
	}
	out := map[string]string{"decision": "block", "reason": reason}
	_ = json.NewEncoder(os.Stdout).Encode(out)
	return nil
}

// stopDecision implements the exact §G predicate. It returns whether to block
// and the instruction to inject. It is fail-open: any uncertainty allows. On a
// block it records the stop-block watermark and offers pending memory (side
// effects), so a subsequent Stop on the same unchanged work allows.
func stopDecision(s *core.Store, in hookInput) (block bool, reason string) {
	// 1. We already blocked and re-entered — never loop.
	if in.StopHookActive {
		return false, ""
	}
	// 2. Evaluate scoped to THIS session's turn. TouchedPaths is the unit of work
	//    = this turn's authored paths ∩ their net change vs the working tree, so a
	//    conversational turn, an edit-then-revert, or unrelated pre-existing dirt
	//    all yield an empty unit → allow (§ fix #4).
	report := s.EvaluateReconcileForSession(in.SessionID)
	// 3. Fail-open: evaluation didn't apply (no git / error / corrupt receipts).
	if !report.Applicable {
		return false, ""
	}
	// 4. No net-changed authored code this turn → nothing to reconcile.
	if len(report.TouchedPaths) == 0 {
		return false, ""
	}
	// 5. Everything required is present → allow.
	if len(report.Blocking) == 0 {
		return false, ""
	}
	// 6. Compare-and-set the blocked obligation-set fingerprint atomically: block
	//    only if THIS process wins the transition (offers memory in the same
	//    locked op). A repeat Stop on the same obligation set, or a concurrent
	//    Stop, or a persistence failure, all yield won=false → allow (§ fix #8, #9).
	var pending []string
	for _, m := range report.Memory {
		if m.State == core.MemPending {
			pending = append(pending, m.ID)
		}
	}
	if !s.ClaimStopBlock(in.SessionID, report.WorkGenerationID, core.BlockingFingerprint(report), pending) {
		return false, ""
	}
	return true, renderStopReason(report)
}

// renderStopReason builds the exact §G instruction from the report.
func renderStopReason(r core.ReconcileReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You finished a unit of work (%s). Before you stop:\n", strings.Join(r.TouchedPaths, ", "))
	for _, item := range r.Blocking {
		fmt.Fprintf(&b, "  • %s\n", item.Message)
	}
	b.WriteString("  • or bypass just this unit: `katra reconcile --skip --reason \"…\"`")
	return b.String()
}

// snapshot (--event pre-compact|session-end): scan + snapshot only. Never blocks.
func hookSnapshotRun() error {
	if _, err := readHookInput(); err != nil {
		return nil
	}
	s, err := resolveStore()
	if err != nil {
		return nil
	}
	_, _ = s.ScanMemory()
	return nil
}

// pre-commit (PreToolUse, matcher Bash): coverage check. If staged non-store
// code has no covering reconciliation receipt, block via exit 2. Bypassed by
// --no-verify / --amend, and only enforced once the reconcile system is in use.
func hookPreCommitRun() error {
	in, err := readHookInput()
	if err != nil {
		return nil // malformed/empty input → allow (§ fix #13)
	}
	s, err := resolveStore()
	if err != nil {
		return nil
	}
	if !preCommitBlocks(s, in) {
		return nil
	}
	fmt.Fprintln(os.Stderr, "katra: staged code isn't covered by a reconciliation receipt.\n"+
		"  Declare it:   katra reconcile --advance/--close <slug>  (or --no-task/--skip --reason \"…\")\n"+
		"  Or skip gate: git commit --no-verify")
	os.Exit(2) // PreToolUse: exit 2 blocks the tool call
	return nil
}

// preCommitBlocks reports whether a `git commit` tool call should be blocked
// because its staged non-store code has no covering reconciliation receipt. It
// only enforces once the repo has used reconciliation (a receipts ledger
// exists), and never for --no-verify / --amend. Fail-open otherwise.
func preCommitBlocks(s *core.Store, in hookInput) bool {
	c := in.ToolInput.Command
	isCommit := in.ToolName == "Bash" && strings.Contains(c, "git ") && strings.Contains(c, "commit")
	if !isCommit || strings.Contains(c, "--no-verify") || strings.Contains(c, "--amend") {
		return false
	}
	if !s.ReceiptsExist() {
		return false // reconcile system not in use → don't block
	}
	staged, err := s.StagedFiles()
	if err != nil {
		return false
	}
	var code []string
	for _, f := range staged {
		if !s.IsStorePath(f) {
			code = append(code, f)
		}
	}
	if len(code) == 0 {
		return false // only the store staged → bookkeeping, allow
	}
	return !s.CoverageSatisfied(code)
}
