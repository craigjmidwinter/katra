package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func stampCmd() *cobra.Command {
	var hashes []string
	var slug string
	var commit bool
	var closes []string
	cmd := &cobra.Command{
		Use:   "stamp",
		Short: "Stamp a draft with its commit hash + diffstat (logs it)",
		Long: "Stamps the active draft (or --entry) with one or more commit hashes\n" +
			"and the computed diffstat, moving it from In Progress into the log.\n" +
			"Defaults to HEAD when no --hash is given.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			e, err := targetEntry(s, slug)
			if err != nil {
				return err
			}
			if len(hashes) == 0 {
				h, err := s.HeadHash()
				if err != nil {
					return err
				}
				hashes = []string{h}
			}
			// Tasks this entry closes: the draft's `closes:` frontmatter plus any
			// --closes flags, de-duped. Recorded on the entry so the linkage
			// persists, and applied after a successful stamp.
			toClose := mergeSlugs(e.FM.Closes, closes)
			e.FM.Closes = toClose
			if err := s.Stamp(e, hashes); err != nil {
				return err
			}
			fmt.Printf("✓ stamped %s → %v", e.Slug, e.AllHashes())
			if e.FM.Stat != nil {
				fmt.Printf("  (%d files, +%d −%d)", e.FM.Stat.F, e.FM.Stat.A, e.FM.Stat.D)
			}
			fmt.Println()
			if len(toClose) > 0 {
				closed, err := s.CloseTasks(e.Slug, toClose)
				if err != nil {
					return fmt.Errorf("stamped, but closing tasks failed: %w", err)
				}
				fmt.Printf("✓ closed %d task(s): %v\n", len(closed), closed)
			}
			if commit {
				if err := s.CommitStamp(e); err != nil {
					return fmt.Errorf("stamped but commit failed: %w", err)
				}
				fmt.Printf("✓ committed the stamp\n")
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&hashes, "hash", nil, "commit hash(es); repeat or comma-separate for a chapter (default HEAD)")
	cmd.Flags().StringVar(&slug, "entry", "", "target entry slug (default: active draft)")
	cmd.Flags().BoolVar(&commit, "commit", false, "git add + commit the stamped entry")
	cmd.Flags().StringSliceVar(&closes, "closes", nil, "task slug(s) this entry completes → mark done + link (adds to the entry's `closes:`)")
	return cmd
}

// mergeSlugs returns the de-duplicated union of two slug lists, preserving order
// (declared frontmatter first, then flag additions).
func mergeSlugs(a, b []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
