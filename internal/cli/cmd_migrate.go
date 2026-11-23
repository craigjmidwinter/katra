package cli

import (
	"fmt"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Move older conventions into the keys that replaced them",
	}
	cmd.AddCommand(migrateInterimTagsCmd())
	return cmd
}

func migrateInterimTagsCmd() *cobra.Command {
	var write bool
	cmd := &cobra.Command{
		Use:   "interim-tags [--write]",
		Short: "Move owner:/blocked-by:/size: tags into owner, blocked_by and effort",
		Long: "Move the interim tags a katra used before these keys existed into the keys:\n" +
			"owner:<who> → owner, blocked-by:<slug> → blocked_by, size:<XS..XL> → effort.\n" +
			"Migrated tags are dropped. Statuses are not changed: a task that gains\n" +
			"blocked_by stays todo until someone runs `katra task block`.\n\n" +
			"Without --write it prints the plan and changes no file. With --write it\n" +
			"applies it; a second run finds nothing to do.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			nodes, err := s.ListNodes("task", "epic")
			if err != nil {
				return err
			}
			plan := core.PlanInterimTags(nodes)
			changed, skipped := 0, 0
			for _, m := range plan {
				if m.Changed() {
					changed++
				}
				skipped += len(m.Skipped)
				fmt.Printf("%s %s\n", m.Node.Kind(), m.Node.Slug)
				for _, c := range m.Changes {
					fmt.Printf("    %s\n", c)
				}
				for _, sk := range m.Skipped {
					fmt.Printf("    left as a tag: %s\n", sk)
				}
			}
			if !write {
				fmt.Printf("\ndry run: %d node(s) would change, %d tag(s) left in place. Nothing written — rerun with --write.\n", changed, skipped)
				return nil
			}
			for _, m := range plan {
				if !m.Changed() {
					continue
				}
				if err := m.Apply(); err != nil {
					return fmt.Errorf("%s: %w (nodes before it were written; rerun to finish, it is idempotent)", m.Node.Slug, err)
				}
			}
			fmt.Printf("\n✓ migrated %d node(s); %d tag(s) left in place.\n", changed, skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "apply the plan (default: print it and change nothing)")
	return cmd
}
