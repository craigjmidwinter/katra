package cli

import (
	"fmt"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

func checkpointCmd() *cobra.Command {
	var slug, fromFile string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "checkpoint [text...]",
		Short: "Capture open loops before losing context (status, not narrative)",
		Long: `Write down what is in flight, what is owed, and where it sits.

An entry says what happened; a checkpoint says what is unfinished. Use it when a
session is about to be compacted or cleared -- the moment knowledge is destroyed
between commits, which the commit gate cannot help with.

Everything katra can derive is derived: tasks in flight and owed, changed code
and whether it has been declared, unresolved memory, the branch and commit. Add
the part only you know as text, --file, or stdin.

With no active draft, one is created. A session running out of room should not
have to make a second decision.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}

			// The note is optional, so stdin is read only when --file asks for
			// it. readChunk's pipe sniffing would otherwise block forever under
			// a hook or any other non-tty caller -- which is precisely where
			// this command is meant to run.
			var note string
			if fromFile != "" || len(args) > 0 {
				note, err = readChunk(args, fromFile)
				if err != nil {
					return err
				}
			}

			c := s.BuildCheckpoint()
			block := c.Render()
			if strings.TrimSpace(note) != "" {
				block += "\n" + strings.TrimSpace(note) + "\n"
			}

			if dryRun {
				fmt.Print(block)
				if !c.HasOpenLoops() {
					fmt.Println("\n(nothing in flight — a hook would stay silent here)")
				}
				return nil
			}

			e, created, err := checkpointTarget(s, c, slug, block)
			if err != nil {
				return err
			}
			if created {
				fmt.Printf("✓ checkpoint written to new entry %s\n", e.Slug)
			} else {
				if err := s.AppendBody(e, block); err != nil {
					return err
				}
				fmt.Printf("✓ checkpoint written to %s\n", e.Slug)
			}
			printCheckpointNext(c, strings.TrimSpace(note) != "")
			return nil
		},
	}

	cmd.Flags().StringVar(&slug, "entry", "", "target entry slug (default: active draft)")
	cmd.Flags().StringVar(&fromFile, "file", "", "read the note from a file ('-' for stdin)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the checkpoint instead of writing it")
	return cmd
}

// checkpointTarget picks where a checkpoint lands: an explicit --entry, else
// today's checkpoint entry, else a new one. Never someone's unrelated draft.
//
// It deliberately does not default to the active draft. A checkpoint is
// session-scoped and a draft is subject-scoped, and the active draft of a long
// session is usually about something else entirely — which is how the first
// live checkpoint landed inside an entry opened hours earlier on another topic.
//
// created reports whether the entry was made with the block already in it, so
// the caller does not append twice.
func checkpointTarget(s *core.Store, c core.Checkpoint, slug, block string) (e *core.Entry, created bool, err error) {
	if slug != "" {
		e, err = s.Get(slug)
		return e, false, err
	}
	if e = s.CheckpointEntry(c.At); e != nil {
		return e, false, nil
	}
	e, err = s.NewEntry(core.Frontmatter{
		Title:   core.CheckpointTitle(c.At),
		Summary: "Open loops captured before clearing context",
		Tags:    []string{"checkpoint"},
	}, block)
	return e, true, err
}

// printCheckpointNext names the one thing the tool could not derive. The status
// block is complete on its own; what it cannot know is why the work was being
// done, which is the part a fresh session most needs and the part only the
// clearing session has.
func printCheckpointNext(c core.Checkpoint, hadNote bool) {
	if !hadNote {
		fmt.Println("  add what only you know:  katra append \"…\"  (or pipe it)")
	}
	if c.Undeclared && len(c.InFlight) > 0 {
		fmt.Println("  code is undeclared:      katra reconcile --advance/--close <slug>")
	}
}
