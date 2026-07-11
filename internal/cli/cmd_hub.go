package cli

import (
	"fmt"

	"github.com/craigjmidwinter/devlog/internal/core"
	"github.com/spf13/cobra"
)

// hubCmd groups cross-project ("hub") commands. Today it can list every
// registered katra; the multi-tenant serving daemon lands under here later.
func hubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Work across every registered katra",
	}
	cmd.AddCommand(hubListCmd())
	return cmd
}

func hubListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered katra projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := core.LoadRegistry()
			if err != nil {
				return err
			}
			if stale := r.Prune(); len(stale) > 0 {
				for _, p := range stale {
					fmt.Printf("  (dropped missing: %s)\n", p)
				}
				_ = r.Save()
			}
			if len(r.Projects) == 0 {
				fmt.Println("No katras registered yet — `katra init` registers one.")
				return nil
			}
			for _, dir := range r.Projects {
				s, err := core.FindStore(dir)
				if err != nil {
					fmt.Printf("%-28s %s  (unreadable: %v)\n", "?", dir, err)
					continue
				}
				entries, _ := s.ListNodes("entry")
				tasks, _ := s.ListNodes("task")
				var drafts, doing int
				for _, e := range entries {
					if e.IsDraft() {
						drafts++
					}
				}
				for _, t := range tasks {
					if t.FM.Status == "doing" {
						doing++
					}
				}
				fmt.Printf("%-28s %s  (%d entries, %d drafts, %d doing)\n",
					truncate(s.Config.Title, 28), dir, len(entries), drafts, doing)
			}
			return nil
		},
	}
}
