package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/craigjmidwinter/devlog/internal/core"
	"github.com/spf13/cobra"
)

var mediaRefRe = regexp.MustCompile(`media/[A-Za-z0-9._\-/]+`)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the katra for problems (dangling media, parse errors)",
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
			for _, e := range entries {
				if e.IsDraft() {
					drafts++
				}
				if _, err := core.RenderMarkdown(e.Body); err != nil {
					fmt.Printf("  ✗ %s: render error: %v\n", e.Slug, err)
					problems++
				}
				for _, ref := range mediaRefRe.FindAllString(e.Body, -1) {
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

			used := map[string]bool{}
			for _, e := range entries {
				for _, ref := range mediaRefRe.FindAllString(e.Body, -1) {
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
				fmt.Printf("  ⚠ not a git repo — stamping/hook unavailable\n")
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
