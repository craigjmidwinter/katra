package cli

import (
	"fmt"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// reconcileCmd is the agent's declaration surface. After finishing a unit of
// work the agent declares how it relates to a task (advance/close/none) or skips
// this unit, which writes a receipt keyed by the current work generation. The
// Stop gate reads those receipts to decide whether to let the agent stop.
func reconcileCmd() *cobra.Command {
	var advance, closeSlug, reason string
	var noTask, skip bool
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Declare how the just-finished work relates to a task (agent checkpoint)",
		Long: "Records the agent's declaration for the current unit of work:\n" +
			"  --advance <slug>   mark a task doing (if todo) and record it\n" +
			"  --close   <slug>   declare the task closed (done+link applied at publish)\n" +
			"  --no-task          this work advances no task (needs --reason)\n" +
			"  --skip             resolve just this unit of work (needs --reason)\n" +
			"With no flags it prints what the gate currently wants (same as `reconcile status`).",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			// Count chosen actions.
			n := 0
			for _, on := range []bool{advance != "", closeSlug != "", noTask, skip} {
				if on {
					n++
				}
			}
			if n == 0 {
				return printReconcileStatus(s)
			}
			if n > 1 {
				return fmt.Errorf("choose exactly one of --advance / --close / --no-task / --skip")
			}

			report := s.EvaluateReconcile()
			receipt := core.ReconcileReceipt{
				WorkGenerationID: report.WorkGenerationID,
				TouchedPaths:     report.TouchedPaths,
			}

			switch {
			case advance != "":
				if err := s.ReconcileAdvance(advance); err != nil {
					return err
				}
				receipt.Task = core.TaskDecl{Kind: "advance", Slug: advance, Reason: reason}
				fmt.Printf("✓ advancing %s\n", advance)
			case closeSlug != "":
				if err := s.ReconcileClose(closeSlug); err != nil {
					return err
				}
				receipt.Task = core.TaskDecl{Kind: "close", Slug: closeSlug, Reason: reason}
				fmt.Printf("✓ declared %s closed (applied at publish)\n", closeSlug)
			case noTask:
				if reason == "" {
					return fmt.Errorf("--no-task requires --reason \"…\"")
				}
				receipt.Task = core.TaskDecl{Kind: "none", Reason: reason}
				fmt.Printf("✓ recorded: this work advances no task\n")
			case skip:
				if reason == "" {
					return fmt.Errorf("--skip requires --reason \"…\"")
				}
				receipt.Skip = true
				receipt.SkipReason = reason
				fmt.Printf("✓ skipped this unit of work\n")
			}

			if err := s.WriteReceipt(receipt); err != nil {
				return fmt.Errorf("record receipt: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&advance, "advance", "", "task slug this work advances (todo → doing)")
	cmd.Flags().StringVar(&closeSlug, "close", "", "task slug this work closes (applied at publish)")
	cmd.Flags().BoolVar(&noTask, "no-task", false, "this work advances no task (with --reason)")
	cmd.Flags().BoolVar(&skip, "skip", false, "resolve just this unit of work (with --reason)")
	cmd.Flags().StringVar(&reason, "reason", "", "explanation for --no-task / --skip")
	cmd.AddCommand(reconcileStatusCmd())
	return cmd
}

func reconcileStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print what the gate currently wants for the in-flight work",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			return printReconcileStatus(s)
		},
	}
}

func printReconcileStatus(s *core.Store) error {
	r := s.EvaluateReconcile()
	if !r.Applicable {
		fmt.Println("reconcile: not applicable (no git repo)")
		return nil
	}
	if len(r.TouchedPaths) == 0 {
		fmt.Println("reconcile: no changed code in the working tree — nothing to reconcile")
		return nil
	}
	fmt.Printf("work-generation: %s\n", r.WorkGenerationID[:min(12, len(r.WorkGenerationID))])
	fmt.Printf("changed code (%d): %s\n", len(r.TouchedPaths), strings.Join(r.TouchedPaths, ", "))
	if r.Draft.HasDraft {
		fmt.Printf("active draft: %s\n", r.Draft.Slug)
	} else {
		fmt.Println("active draft: none (start one with `katra new \"…\"`)")
	}
	if r.Task.Kind != "" && r.Task.Kind != "unknown" {
		fmt.Printf("task: declared %s %s\n", r.Task.Kind, r.Task.Slug)
	}
	if len(r.Blocking) == 0 {
		fmt.Println("status: ✓ reconciled — the gate will allow a stop")
		return nil
	}
	fmt.Println("status: needs reconciliation —")
	for _, b := range r.Blocking {
		fmt.Printf("  • %s\n", b.Message)
	}
	return nil
}
