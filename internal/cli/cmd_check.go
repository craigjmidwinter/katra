package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// gateState is the outcome of evaluating the commit gate.
type gateState int

const (
	gateOpen       gateState = iota // nothing to block
	gateNoDraft                     // code staged, no draft at all
	gateEmptyDraft                  // code staged, draft open but nothing written in it
)

// blocks reports whether this state should stop the commit.
func (g gateState) blocks() bool { return g != gateOpen }

// message is what to tell the human (or agent) about the block.
func (g gateState) message() string {
	switch g {
	case gateNoDraft:
		return "katra: code is staged but no active draft is open.\n" +
			"  Log this work first:  katra new \"what you're doing\"\n" +
			"  Or skip the gate:     git commit --no-verify"
	case gateEmptyDraft:
		return "katra: your draft is still the placeholder — write a sentence.\n" +
			"  Say what you did:     katra append \"why this change, not what it does\"\n" +
			"  Or skip the gate:     git commit --no-verify"
	default:
		return ""
	}
}

// gateCheck reports whether a commit should be blocked: it blocks only when
// real code is staged with no *written* draft to record it. It is fail-open —
// any uncertainty (no store, git error, nothing staged, only the store staged,
// or a draft already written) returns gateOpen, so a katra hiccup can never
// block a commit.
func gateCheck() gateState {
	s, err := resolveStore()
	if err != nil {
		return gateOpen
	}
	return gateFor(s)
}

// gateFor is the gate itself, given a store. Every error path returns gateOpen:
// the gate exists to nudge, never to strand a commit behind a katra bug.
func gateFor(s *core.Store) gateState {
	staged, err := s.StagedFiles()
	if err != nil || len(staged) == 0 {
		return gateOpen
	}
	if s.StoreRelPrefix() == "" {
		return gateOpen
	}
	for _, f := range staged {
		if s.IsStorePath(f) {
			continue
		}
		// found a non-store (code) file staged
		draft, err := s.ActiveDraft()
		if err != nil {
			return gateOpen
		}
		if draft == nil {
			return gateNoDraft
		}
		// A draft whose body is still the `katra new` prompt (or empty) has
		// recorded nothing; treating it as a draft would let the gate be
		// satisfied by typing one command and never writing.
		if core.IsUnwrittenBody(draft.Body) {
			return gateEmptyDraft
		}
		return gateOpen
	}
	return gateOpen // only the store is staged → bookkeeping, allow
}

// checkCmd is the enforcement primitive: exit 1 when code is staged without a
// draft, else exit 0. Usable directly in a git pre-commit hook or CI.
func checkCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Exit non-zero if code is staged with no active draft (for commit-gate hooks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if g := gateCheck(); g.blocks() {
				if !quiet {
					fmt.Fprintln(os.Stderr, g.message())
				}
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress the explanatory message")
	return cmd
}

// guardCmd is the Claude Code PreToolUse hook. It reads the tool call from
// stdin; if it's a `git commit` (without --no-verify), it applies the gate and
// exits 2 to block. Everything else exits 0.
func guardCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "guard",
		Short:  "PreToolUse hook: block `git commit` when work isn't logged (reads the tool call on stdin)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var in struct {
				ToolName  string `json:"tool_name"`
				ToolInput struct {
					Command string `json:"command"`
				} `json:"tool_input"`
			}
			b, _ := io.ReadAll(os.Stdin)
			_ = json.Unmarshal(b, &in)
			c := in.ToolInput.Command
			isCommit := in.ToolName == "Bash" && strings.Contains(c, "git ") && strings.Contains(c, "commit")
			if !isCommit || strings.Contains(c, "--no-verify") || strings.Contains(c, "--amend") {
				return nil // not a gated commit → allow
			}
			if g := gateCheck(); g.blocks() {
				fmt.Fprintln(os.Stderr, g.message())
				os.Exit(2) // PreToolUse: exit 2 blocks the tool call
			}
			return nil
		},
	}
}
