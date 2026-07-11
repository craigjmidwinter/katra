package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/craigjmidwinter/devlog/internal/core"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var title string
	var here, installHook bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a katra in this repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, _ := os.Getwd()
			base := wd
			if !here {
				if root := gitToplevel(wd); root != "" {
					base = root
				}
			}
			dir := filepath.Join(base, core.DefaultDirName)
			if here {
				dir = wd
			}
			if title == "" {
				title = strings.Title(filepath.Base(base)) + " Dev Log"
			}
			s, err := core.InitStore(dir, title)
			if err != nil {
				return err
			}
			// a welcome draft so the page renders something immediately
			_, _ = s.NewEntry(core.Frontmatter{
				Title:   "Hello, katra",
				Tags:    []string{"meta"},
				Summary: "First entry — delete me.",
			}, sampleBody)

			fmt.Printf("✓ katra created at %s\n", rel(wd, dir))
			fmt.Printf("  entries/   — one markdown file per post\n")
			fmt.Printf("  media/     — images, gifs, video, html embeds\n")
			fmt.Printf("  config.yml — title, accent, hook behaviour\n\n")

			// Register with the global registry so `katra hub` can find it.
			if err := core.Register(dir); err != nil {
				fmt.Printf("  (not registered globally: %v)\n", err)
			} else {
				fmt.Printf("  registered with the global katra registry (%s)\n", core.RegistryPath())
			}

			if installHook {
				if p, err := s.InstallHook(); err != nil {
					fmt.Printf("  (hook not installed: %v)\n", err)
				} else {
					fmt.Printf("✓ post-commit hook installed at %s\n", rel(wd, p))
				}
			}
			fmt.Printf("\nNext:  katra serve     # open the live page\n")
			fmt.Printf("       katra new \"…\"    # start an entry\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "dev log title (default: repo name)")
	cmd.Flags().BoolVar(&here, "here", false, "create the katra in the current directory, not the repo root")
	cmd.Flags().BoolVar(&installHook, "install-hook", false, "also install the auto-stamp post-commit hook")
	return cmd
}

func gitToplevel(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func rel(from, p string) string {
	if r, err := filepath.Rel(from, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

const sampleBody = `Welcome to your dev log. **Write entries as you work** — this file is a *draft*
until you stamp it with a commit, so it shows up in the **In Progress** panel
right away.

## How it works

- ` + "`katra new \"Title\"`" + ` starts a draft (a markdown file in ` + "`entries/`" + `).
- Write the *why*, not a paraphrased diff. Drop in screenshots and components.
- At commit time it gets stamped with the hash + diffstat and drops into the log.

## Rich components

Embed components by fencing them with a registered language:

` + "```note" + `
This is a callout. Markdown **works** inside it.
` + "```" + `

Add media with ` + "`katra capture <file>`" + `, before/after sliders with
` + "`katra compare <before> <after>`" + `, and interactive HTML artifacts with an
` + "`embed`" + ` block. Then run ` + "`katra serve`" + ` and watch it live-reload.
`
