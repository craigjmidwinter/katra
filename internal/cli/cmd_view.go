package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/craigjmidwinter/devlog/internal/viewer"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var draftsOnly, asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entries (newest first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			entries, err := s.List()
			if err != nil {
				return err
			}
			if asJSON {
				type row struct {
					Slug   string   `json:"slug"`
					Title  string   `json:"title"`
					Date   string   `json:"date"`
					Time   string   `json:"time,omitempty"`
					Draft  bool     `json:"draft"`
					Hashes []string `json:"hashes,omitempty"`
					Tags   []string `json:"tags,omitempty"`
				}
				var rows []row
				for _, e := range entries {
					if draftsOnly && !e.IsDraft() {
						continue
					}
					rows = append(rows, row{e.Slug, e.FM.Title, e.FM.Date, e.FM.Time, e.IsDraft(), e.AllHashes(), e.FM.Tags})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}
			for _, e := range entries {
				if draftsOnly && !e.IsDraft() {
					continue
				}
				marker := "  "
				if e.IsDraft() {
					marker = "● "
				}
				stat := ""
				if e.FM.Stat != nil {
					stat = fmt.Sprintf("  (+%d −%d)", e.FM.Stat.A, e.FM.Stat.D)
				}
				hash := "draft"
				if h := e.AllHashes(); len(h) > 0 {
					hash = fmt.Sprintf("%v", h)
				}
				when := e.FM.Date
				if e.FM.Time != "" {
					when += " " + e.FM.Time
				}
				fmt.Printf("%s%-19s  %-46s  %s%s\n", marker, when, truncate(e.FM.Title, 46), hash, stat)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&draftsOnly, "drafts", false, "only show unstamped drafts")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func serveCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the live, auto-reloading dev log on the LAN",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			return viewer.Serve(s, port)
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "port to serve on")
	return cmd
}

func buildCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a static site (index.html + data.json + media)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			if err := viewer.Build(s, out); err != nil {
				return err
			}
			fmt.Printf("✓ built static dev log → %s\n", rel(mustWd(), out))
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "dist", "output directory")
	return cmd
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
