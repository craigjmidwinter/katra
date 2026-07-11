package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/craigjmidwinter/devlog/internal/core"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	var tags []string
	var summary, date, body string
	var featured bool
	cmd := &cobra.Command{
		Use:   "new \"Title\"",
		Short: "Start a new draft entry",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			title := strings.Join(args, " ")
			if body == "" {
				body = "Start writing here.\n"
			}
			e, err := s.NewEntry(core.Frontmatter{
				Title:    title,
				Tags:     tags,
				Summary:  summary,
				Date:     date,
				Featured: featured,
			}, body)
			if err != nil {
				return err
			}
			fmt.Printf("✓ draft created: %s\n", rel(mustWd(), e.Path))
			fmt.Printf("  slug: %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary for the index")
	cmd.Flags().StringVar(&date, "date", "", "entry date (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&body, "body", "", "initial body markdown")
	cmd.Flags().BoolVar(&featured, "featured", false, "mark as a Deep Dive")
	return cmd
}

func appendCmd() *cobra.Command {
	var slug string
	var fromFile string
	cmd := &cobra.Command{
		Use:   "append [text...]",
		Short: "Append markdown to a draft (text args, --file, or stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			chunk, err := readChunk(args, fromFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(chunk) == "" {
				return fmt.Errorf("nothing to append")
			}
			e, err := targetEntry(s, slug)
			if err != nil {
				return err
			}
			if err := s.AppendBody(e, chunk); err != nil {
				return err
			}
			fmt.Printf("✓ appended to %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&slug, "entry", "", "target entry slug (default: active draft)")
	cmd.Flags().StringVar(&fromFile, "file", "", "read markdown from a file ('-' for stdin)")
	return cmd
}

func readChunk(args []string, fromFile string) (string, error) {
	if fromFile == "-" {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	if fromFile != "" {
		b, err := os.ReadFile(fromFile)
		return string(b), err
	}
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	if fi, _ := os.Stdin.Stat(); fi != nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	return "", nil
}

// targetEntry resolves the entry to operate on: an explicit slug, or the active draft.
func targetEntry(s *core.Store, slug string) (*core.Entry, error) {
	if slug != "" {
		return s.Get(slug)
	}
	e, err := s.ActiveDraft()
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("no active draft — pass --entry <slug> or `katra new`")
	}
	return e, nil
}

func mustWd() string {
	wd, _ := os.Getwd()
	return wd
}
