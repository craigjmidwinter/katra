package cli

import (
	"fmt"
	"os"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// warnNoVisual nudges when an entry lands with nothing to look at. The skill
// asks every entry for one visual, and an entry is easiest to fix in the moment
// it is stamped — so this is a note on stderr, never a failure: the entry is
// already written and the commit already made.
func warnNoVisual(e core.Entry) {
	if core.HasVisual(e) {
		return
	}
	fmt.Fprintln(os.Stderr, "⚠ no visual in this entry — `katra capture <file>` adds a screenshot, chart or diagram")
}

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
			// persists, and applied by PublishEntry after a successful stamp.
			e.FM.Closes = mergeSlugs(e.FM.Closes, closes)
			res, err := s.PublishEntry(e, hashes)
			if err != nil {
				return err
			}
			fmt.Printf("✓ stamped %s → %v", e.Slug, e.AllHashes())
			if e.FM.Stat != nil {
				fmt.Printf("  (%d files, +%d −%d)", e.FM.Stat.F, e.FM.Stat.A, e.FM.Stat.D)
			}
			fmt.Println()
			if len(res.Closed) > 0 {
				fmt.Printf("✓ closed %d task(s): %v\n", len(res.Closed), res.Closed)
			}
			if len(res.Epics) > 0 {
				fmt.Printf("✓ rolled up %d epic(s): %v\n", len(res.Epics), res.Epics)
			}
			warnNoVisual(*e)
			if commit {
				if err := s.CommitStamp(e, res.Mutated); err != nil {
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
