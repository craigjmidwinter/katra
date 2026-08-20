package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func hookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the git post-commit auto-stamp hook",
	}
	cmd.AddCommand(hookInstallCmd(), hookUninstallCmd(), hookRunCmd())
	return cmd
}

func hookInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the post-commit hook that auto-stamps the active draft",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			p, err := s.InstallHook()
			if err != nil {
				return err
			}
			fmt.Printf("✓ post-commit hook installed at %s\n", rel(mustWd(), p))
			fmt.Printf("  After each commit, the active draft is stamped with that commit.\n")
			if !s.Config.AutoCommit {
				fmt.Printf("  The stamp is left as a working-tree change for you to commit.\n")
				fmt.Printf("  (set autoCommit: true in config.yml to commit it automatically)\n")
			}
			return nil
		},
	}
}

func hookUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the katra post-commit hook",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			p, err := s.UninstallHook()
			if err != nil {
				return err
			}
			fmt.Printf("✓ removed katra hook from %s\n", rel(mustWd(), p))
			return nil
		},
	}
}

// hookRunCmd is what the installed post-commit hook calls. It stamps the active
// draft with the just-made commit, unless this commit was its own bookkeeping.
func hookRunCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:    "run",
		Short:  "(internal) stamp the active draft for the current HEAD",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return nil // not in a katra repo — silently no-op
			}
			skip, err := s.HookShouldSkip()
			if err != nil || skip {
				return nil
			}
			e, err := s.ActiveDraft()
			if err != nil || e == nil {
				return nil
			}
			head, err := s.HeadHash()
			if err != nil {
				return nil
			}
			res, err := s.PublishEntry(e, []string{head})
			if err != nil {
				return nil
			}
			if !quiet {
				fmt.Printf("katra: stamped %q with %s\n", e.Slug, head)
				if len(res.Closed) > 0 {
					fmt.Printf("katra: closed task(s): %v\n", res.Closed)
				}
				if len(res.Epics) > 0 {
					fmt.Printf("katra: rolled up epic(s): %v\n", res.Epics)
				}
				warnNoVisual(*e)
			}
			if s.Config.AutoCommit {
				_ = s.CommitStamp(e, res.Mutated)
			} else if !quiet {
				fmt.Printf("katra: review + commit %s\n", rel(mustWd(), e.Path))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress output")
	return cmd
}
