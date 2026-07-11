package cli

import (
	"fmt"
	"path/filepath"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/craigjmidwinter/katra/internal/viewer"
	"github.com/spf13/cobra"
)

// hubCmd groups cross-project ("hub") commands: list every registered katra and
// serve them all from one URL.
func hubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Work across every registered katra",
	}
	cmd.AddCommand(hubListCmd(), hubServeCmd())
	return cmd
}

// hubProjects builds the served project list from the global registry, pruning
// missing stores and assigning each a stable, unique URL id.
func hubProjects() ([]viewer.HubProject, error) {
	r, err := core.LoadRegistry()
	if err != nil {
		return nil, err
	}
	if len(r.Prune()) > 0 {
		_ = r.Save()
	}
	used := map[string]bool{}
	var projects []viewer.HubProject
	for _, dir := range r.Projects {
		s, err := core.FindStore(dir)
		if err != nil {
			continue
		}
		projects = append(projects, viewer.HubProject{ID: projectID(dir, used), Store: s})
	}
	return projects, nil
}

// projectID derives a stable URL slug from a store dir: the repo name (the dir
// holding the katra/), de-duplicated with a numeric suffix on collision.
func projectID(storeDir string, used map[string]bool) string {
	base := filepath.Base(storeDir)
	if base == core.DefaultDirName || base == core.LegacyDirName {
		base = filepath.Base(filepath.Dir(storeDir))
	}
	id := core.Slugify(base)
	if id == "" {
		id = "katra"
	}
	orig := id
	for n := 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", orig, n)
	}
	used[id] = true
	return id
}

func hubServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve every registered katra from one URL (LAN-reachable)",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := hubProjects()
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				return fmt.Errorf("no katras registered — run `katra init` in a repo first")
			}
			return viewer.ServeHub(projects, port)
		},
	}
	cmd.Flags().IntVar(&port, "port", 4200, "port to serve on")
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
