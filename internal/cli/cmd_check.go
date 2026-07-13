package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// gateBlocks reports whether a commit should be blocked: true only when real
// code is staged with no active draft to record it. It is fail-open — any
// uncertainty (no store, git error, nothing staged, only the store staged, or a
// draft already open) returns false, so a katra hiccup can never block a commit.
func gateBlocks() bool {
	s, err := resolveStore()
	if err != nil {
		return false
	}
	staged, err := s.StagedFiles()
	if err != nil || len(staged) == 0 {
		return false
	}
	if s.StoreRelPrefix() == "" {
		return false
	}
	for _, f := range staged {
		if !s.IsStorePath(f) {
			// found a non-store (code) file staged
			draft, _ := s.ActiveDraft()
			return draft == nil
		}
	}
	return false // only the store is staged → bookkeeping, allow
}

const gateMsg = "katra: code is staged but no active draft is open.\n" +
	"  Log this work first:  katra new \"what you're doing\"\n" +
	"  Or skip the gate:     git commit --no-verify"

// checkCmd is the enforcement primitive: exit 1 when code is staged without a
// draft, else exit 0. Usable directly in a git pre-commit hook or CI.
func checkCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Exit non-zero if code is staged with no active draft (for commit-gate hooks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if gateBlocks() {
				if !quiet {
					fmt.Fprintln(os.Stderr, gateMsg)
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
			if gateBlocks() {
				fmt.Fprintln(os.Stderr, gateMsg)
				os.Exit(2) // PreToolUse: exit 2 blocks the tool call
			}
			return nil
		},
	}
}
