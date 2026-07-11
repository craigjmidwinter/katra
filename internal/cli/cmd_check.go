package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// checkCmd is the enforcement primitive behind a commit gate. It exits non-zero
// ONLY when there is real code work staged for commit with no active katra
// draft to record it. Every other case — no store, git error, nothing staged,
// only store/bookkeeping files staged, or a draft already open — exits 0. This
// fail-open bias means a katra hiccup can never block a commit; only the
// specific "you staged code but didn't open a log" case does.
func checkCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Exit non-zero if code is staged with no active draft (for commit-gate hooks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return nil // no katra here → nothing to enforce
			}
			staged, err := s.StagedFiles()
			if err != nil || len(staged) == 0 {
				return nil // git error or nothing staged → allow
			}
			root, err := s.RepoRoot()
			if err != nil {
				return nil // can't reason about paths → allow
			}
			// Canonicalize both (git returns the real path; the store dir may
			// carry a symlink, e.g. /tmp → /private/tmp on macOS) so Rel is sane.
			if real, e := filepath.EvalSymlinks(root); e == nil {
				root = real
			}
			dir := s.Dir
			if real, e := filepath.EvalSymlinks(dir); e == nil {
				dir = real
			}
			storeRel, err := filepath.Rel(root, dir)
			if err != nil {
				return nil
			}
			storePrefix := filepath.ToSlash(storeRel) + "/"
			onlyStore := true
			for _, f := range staged {
				if !strings.HasPrefix(filepath.ToSlash(f), storePrefix) {
					onlyStore = false
					break
				}
			}
			if onlyStore {
				return nil // only the katra store staged → bookkeeping, allow
			}
			draft, _ := s.ActiveDraft()
			if draft != nil {
				return nil // work is being logged → allow
			}
			if !quiet {
				fmt.Fprintln(os.Stderr, "katra: code is staged but no active draft is open.")
				fmt.Fprintln(os.Stderr, "  Log this work first:  katra new \"what you're doing\"")
				fmt.Fprintln(os.Stderr, "  Or skip the gate:     git commit --no-verify")
			}
			os.Exit(1)
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress the explanatory message")
	return cmd
}
