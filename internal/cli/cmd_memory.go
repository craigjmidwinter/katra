package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// memoryCmd is the `katra memory` group: the deterministic ingest surface over
// Claude Code's native per-project memory. It only reads memory and records
// generations to a private ledger — it never creates entries or drafts (that's
// the later author phase).
func memoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Ingest Claude Code's native memory into a private katra ledger",
	}
	cmd.AddCommand(
		memoryScanCmd(),
		memoryStatusCmd(),
		memoryIgnoreCmd(),
		memoryResolveCmd(),
	)
	return cmd
}

func memoryScanCmd() *cobra.Command {
	var snapshot, quiet bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan Claude memory, update the ledger, and report changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				// Fail-open: no store is not an error for a hook.
				if snapshot || quiet {
					return nil
				}
				return err
			}
			res, err := s.ScanMemory()
			if err != nil {
				if snapshot || quiet {
					return nil // never let ingest noise fail a hook
				}
				return err
			}
			if quiet || snapshot {
				return nil
			}
			if res.Skipped {
				fmt.Println("memory: nothing to ingest (disabled or no Claude memory for this repo)")
				return nil
			}
			fmt.Printf("memory: %d new, %d appended, %d rewritten, %d quarantined\n",
				res.New, res.Appended, res.Rewritten, res.Quarantined)
			return nil
		},
	}
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "record generations to the ledger silently (for hook use)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress output")
	return cmd
}

func memoryStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List ledger generations (pending / imported / ignored / quarantined)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			gens, err := s.MemoryGenerations()
			if err != nil {
				return err
			}
			if len(gens) == 0 {
				fmt.Println("memory: no generations recorded (run `katra memory scan`)")
				return nil
			}
			// Group by state for a scannable, stable listing.
			order := []string{core.MemPending, core.MemOffered, core.MemQuarantined, core.MemImported, core.MemIgnored}
			byState := map[string][]core.MemoryGeneration{}
			for _, g := range gens {
				byState[g.State] = append(byState[g.State], g)
			}
			for _, st := range order {
				list := byState[st]
				if len(list) == 0 {
					continue
				}
				sort.SliceStable(list, func(i, j int) bool { return list[i].Path < list[j].Path })
				for _, g := range list {
					// Print a short generation id so it can be resolved precisely
					// (`katra memory resolve <id-prefix>`) rather than only by
					// basename (§ fix #11).
					id := g.ID
					if len(id) > 12 {
						id = id[:12]
					}
					line := fmt.Sprintf("%-12s %-10s %-12s %s", g.State, g.Change, id, g.Path)
					if g.Reason != "" {
						line += "  (" + g.Reason + ")"
					}
					fmt.Println(line)
				}
			}
			return nil
		},
	}
	return cmd
}

func memoryIgnoreCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "ignore <file>",
		Short: "Mark a memory file's current generation as ignored",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			g, err := s.ResolveMemory(args[0], core.MemIgnored, reason)
			if err != nil {
				if err == os.ErrNotExist {
					return fmt.Errorf("no ledger generation for %q (run `katra memory scan`)", args[0])
				}
				return err
			}
			fmt.Printf("ignored %s\n", g.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why this generation is ignored")
	return cmd
}

func memoryResolveCmd() *cobra.Command {
	var imported bool
	var reason string
	cmd := &cobra.Command{
		Use:   "resolve <file>",
		Short: "Resolve a memory file's current generation (author-phase primitive)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			if !imported {
				return fmt.Errorf("specify a resolution, e.g. --imported")
			}
			g, err := s.ResolveMemory(args[0], core.MemImported, reason)
			if err != nil {
				if err == os.ErrNotExist {
					return fmt.Errorf("no ledger generation for %q (run `katra memory scan`)", args[0])
				}
				return err
			}
			fmt.Printf("imported %s\n", g.Path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&imported, "imported", false, "mark the generation as imported")
	cmd.Flags().StringVar(&reason, "reason", "", "optional note")
	return cmd
}
