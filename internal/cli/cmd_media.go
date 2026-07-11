package cli

import (
	"fmt"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

func captureCmd() *cobra.Command {
	var caption, slug, name, as string
	var noAppend bool
	cmd := &cobra.Command{
		Use:   "capture <file>",
		Short: "Import media into the katra and add it to a draft",
		Args:  cobra.ExactArgs(1),
		Long: "Copies an image / gif / video / html artifact into media/ and appends\n" +
			"the right block to the active draft (or --entry). Use --no-append to\n" +
			"only import it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			ref, err := s.ImportMedia(args[0], name)
			if err != nil {
				return err
			}
			fmt.Printf("✓ imported %s\n", ref)
			if noAppend {
				return nil
			}
			kind := core.KindFor(ref)
			if as != "" {
				kind = core.MediaKind(as)
			}
			e, err := targetEntry(s, slug)
			if err != nil {
				fmt.Printf("  (not appended: %v)\n", err)
				return nil
			}
			if err := s.AppendBody(e, core.MediaBlock(ref, caption, kind)); err != nil {
				return err
			}
			fmt.Printf("✓ added to %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&caption, "caption", "", "caption for the media")
	cmd.Flags().StringVar(&slug, "entry", "", "target entry slug (default: active draft)")
	cmd.Flags().StringVar(&name, "name", "", "store under this filename")
	cmd.Flags().StringVar(&as, "as", "", "force kind: image|video|embed")
	cmd.Flags().BoolVar(&noAppend, "no-append", false, "import only; don't add to an entry")
	return cmd
}

func compareCmd() *cobra.Command {
	var caption, slug string
	var noImport bool
	cmd := &cobra.Command{
		Use:   "compare <before> <after>",
		Short: "Add a before/after slider to a draft",
		Args:  cobra.ExactArgs(2),
		Long: "Imports two images and appends a before/after compare slider. Pass\n" +
			"--no-import if the paths already point inside media/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			before, after := args[0], args[1]
			if !noImport {
				if before, err = s.ImportMedia(args[0], ""); err != nil {
					return err
				}
				if after, err = s.ImportMedia(args[1], ""); err != nil {
					return err
				}
			}
			e, err := targetEntry(s, slug)
			if err != nil {
				return err
			}
			if err := s.AppendBody(e, core.CompareBlock(before, after, caption)); err != nil {
				return err
			}
			fmt.Printf("✓ added compare (%s ⇆ %s) to %s\n", before, after, e.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&caption, "caption", "", "caption for the comparison")
	cmd.Flags().StringVar(&slug, "entry", "", "target entry slug (default: active draft)")
	cmd.Flags().BoolVar(&noImport, "no-import", false, "paths already reference media/, don't copy")
	return cmd
}
