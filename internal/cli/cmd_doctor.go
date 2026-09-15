package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// visualOffenders caps how many zero-visual entries doctor names; the count is
// the finding, the list is just enough to start on.
const visualOffenders = 5

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the katra for problems (dangling media, parse errors, stale epics)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			fmt.Printf("katra: %s\n", s.Dir)
			problems := 0

			entries, err := s.List()
			if err != nil {
				return err
			}
			drafts := 0
			var blind []core.Entry // published entries carrying no visual
			for _, e := range entries {
				if e.IsDraft() {
					drafts++
				} else if !core.HasVisual(e) {
					blind = append(blind, e)
				}
				if _, err := core.RenderMarkdown(e.Body); err != nil {
					fmt.Printf("  ✗ %s: render error: %v\n", e.Slug, err)
					problems++
				}
				for _, ref := range core.MediaRefs(e.Body) {
					p := filepath.Join(s.Dir, ref)
					if _, err := os.Stat(p); err != nil {
						fmt.Printf("  ✗ %s: missing media %s\n", e.Slug, ref)
						problems++
					}
				}
			}
			fmt.Printf("  %d entries (%d drafts)\n", len(entries), drafts)
			if drafts > 1 {
				fmt.Printf("  ⚠ %d drafts open — `katra stamp` targets the newest\n", drafts)
			}

			// Zero-visual entries are a habit problem, not a broken file: a
			// warning with names, never an error.
			if published := len(entries) - drafts; published > 0 && len(blind) > 0 {
				fmt.Printf("  ⚠ no visual in %d of %d published entries (%d%% coverage)\n",
					len(blind), published, (published-len(blind))*100/published)
				for i, e := range blind {
					if i >= visualOffenders {
						fmt.Printf("      … and %d more\n", len(blind)-visualOffenders)
						break
					}
					fmt.Printf("      %s  %s\n", e.FM.Date, e.Slug)
				}
			}

			// Epic status is a cache the viewer no longer trusts; report the
			// drift so `katra epic rollup --write` has something to act on.
			if nodes, err := s.ListNodes(); err == nil {
				for _, d := range core.EpicStatusDrift(nodes) {
					fmt.Printf("  ⚠ epic %s: stored %s, its tasks say %s — `katra epic rollup --write`\n",
						d.Slug, orUnset(d.Stored), d.Computed)
				}
			}

			// Authorship coverage. Reported rather than assumed: a count of
			// authored nodes without the count of unauthored ones measures
			// whoever remembered to set $KATRA_AUTHOR, not who did the work.
			// Never an error -- an unattributed node is not broken, and nodes
			// created before the field existed can never be attributed at all.
			if tasks, err := s.ListNodes("task"); err == nil && len(tasks) > 0 {
				if missing, err := s.UnattributedNodes("task"); err == nil && len(missing) > 0 {
					fmt.Printf("  ⚠ no author on %d of %d tasks (%d%% attributable)\n",
						len(missing), len(tasks), (len(tasks)-len(missing))*100/len(tasks))
					if core.Author() == "" {
						fmt.Printf("      $%s is unset here, so new nodes will be unattributed too\n", core.AuthorEnv)
					}
				}
			}

			// `katra task spec` writes a dangling ref regardless (the spec may land
			// in the same change), so doctor is where it actually gets reported.
			if nodes, err := s.ListNodes(); err == nil {
				root, _ := s.RepoRoot()
				for _, n := range nodes {
					if n.Kind() != "task" || n.FM.Spec == "" {
						continue
					}
					if kind, _ := core.ResolveSpec(nodes, root, n.FM.Spec); kind == "" {
						fmt.Printf("  ⚠ %s: spec %q resolves to neither a node nor a file\n", n.Slug, n.FM.Spec)
					}
				}
			}

			used := map[string]bool{}
			for _, e := range entries {
				for _, ref := range core.MediaRefs(e.Body) {
					used[strings.TrimPrefix(ref, "media/")] = true
				}
				if e.FM.Cover != "" {
					used[strings.TrimPrefix(e.FM.Cover, "media/")] = true
				}
			}
			if mfiles, _ := os.ReadDir(s.MediaDir()); mfiles != nil {
				orphans := 0
				for _, mf := range mfiles {
					if !mf.IsDir() && !used[mf.Name()] {
						orphans++
					}
				}
				if orphans > 0 {
					fmt.Printf("  ⚠ %d unreferenced file(s) in media/\n", orphans)
				}
			}

			if _, err := s.RepoRoot(); err != nil {
				if core.IsGitNotFound(err) {
					fmt.Printf("  ⚠ git not found on PATH — install git to enable stamping/hook\n")
				} else {
					fmt.Printf("  ⚠ not a git repo — stamping/hook unavailable\n")
				}
			}

			if problems == 0 {
				fmt.Printf("✓ no problems\n")
			} else {
				fmt.Printf("✗ %d problem(s)\n", problems)
				os.Exit(1)
			}
			return nil
		},
	}
}

// orUnset labels an empty status readably in doctor output.
func orUnset(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
