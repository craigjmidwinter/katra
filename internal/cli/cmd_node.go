package cli

import (
	"fmt"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

// taskCmd groups the task subcommands (new/list/start/done).
func taskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Work with tasks",
	}
	cmd.AddCommand(taskNewCmd(), taskListCmd(), taskStartCmd(), taskDoneCmd(), taskSpecCmd())
	return cmd
}

func taskNewCmd() *cobra.Command {
	var tags []string
	var effort, epic, summary, body, spec, horizon string
	cmd := &cobra.Command{
		Use:   "new \"Title\"",
		Short: "Create a new task (status: todo, or specced with --spec)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			status := "todo"
			if spec != "" {
				status = "specced"
			}
			e, err := s.NewNode("task", core.Frontmatter{
				Title:   strings.Join(args, " "),
				Status:  status,
				Spec:    spec,
				Effort:  effort,
				Horizon: horizon,
				Epic:    epic,
				Tags:    tags,
				Summary: summary,
			}, body)
			if err != nil {
				return err
			}
			fmt.Printf("✓ task created: %s\n", rel(mustWd(), e.Path))
			fmt.Printf("  slug: %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&effort, "effort", "", "effort estimate (S|M|L)")
	// horizon is documented in docs/format.md as a *task* field, and `epic new`
	// has exposed it since it existed -- but `task new` never did, so the one
	// prioritisation field the format has could not be set on the node type it
	// is documented for. A migration hit this for real: an ordered blocker list
	// had nowhere to go.
	cmd.Flags().StringVar(&horizon, "horizon", "", "planning horizon (now|next|later)")
	cmd.Flags().StringVar(&epic, "epic", "", "parent epic slug")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary")
	cmd.Flags().StringVar(&body, "body", "", "initial body markdown")
	cmd.Flags().StringVar(&spec, "spec", "", "spec artifact ref (node slug, or a path relative to the repo root); creates the task already specced")
	return cmd
}

// taskSpecCmd attaches a spec artifact to a task. Setting spec never moves a
// status backwards: todo/empty advances to specced; doing/done/cut are left
// alone (§ design/task-spec-phase — spec may be recorded retroactively).
func taskSpecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec <slug> <ref>",
		Short: "Attach a spec artifact to a task (advances todo → specced)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			slug, ref := args[0], args[1]
			e, err := s.GetNode(slug)
			if err != nil {
				return err
			}
			e.FM.Spec = ref
			if e.FM.Status == "" || e.FM.Status == "todo" {
				e.FM.Status = "specced"
			}
			if err := e.Save(); err != nil {
				return err
			}
			// A ref that doesn't resolve warns but writes — the spec may be
			// authored in the same change; `katra doctor` is the blocking check.
			nodes, _ := s.ListNodes()
			root, _ := s.RepoRoot()
			if kind, _ := core.ResolveSpec(nodes, root, ref); kind == "" {
				fmt.Printf("⚠ %s: spec %q resolves to neither a node nor a file yet — `katra doctor` will flag it until it does\n", e.Slug, ref)
			}
			fmt.Printf("✓ %s spec → %s (status: %s)\n", e.Slug, ref, e.FM.Status)
			return nil
		},
	}
}

func taskListCmd() *cobra.Command {
	var status []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks (optionally filtered by status)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			nodes, err := s.ListNodes("task")
			if err != nil {
				return err
			}
			want := map[string]bool{}
			for _, st := range status {
				want[st] = true
			}
			n := 0
			for _, e := range nodes {
				if len(want) > 0 && !want[e.FM.Status] {
					continue
				}
				n++
				st := e.FM.Status
				if st == "" {
					st = "todo"
				}
				fmt.Printf("%-6s %-24s %s\n", st, e.Slug, e.FM.Title)
			}
			if n == 0 {
				fmt.Println("no tasks")
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&status, "status", nil, "filter by status (todo|specced|doing|done|cut)")
	return cmd
}

func taskStartCmd() *cobra.Command {
	return statusCmd("start <slug>", "Mark a task as in progress (doing)", "doing")
}

func taskDoneCmd() *cobra.Command {
	return statusCmd("done <slug>", "Mark a task as done", "done")
}

// statusCmd builds a subcommand that loads a node by slug, sets its status, and saves.
func statusCmd(use, short, status string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			e, err := s.GetNode(args[0])
			if err != nil {
				return err
			}
			e.FM.Status = status
			if err := e.Save(); err != nil {
				return err
			}
			fmt.Printf("✓ %s → %s\n", e.Slug, status)
			return nil
		},
	}
}

// epicCmd groups the epic subcommands.
func epicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Work with epics",
	}
	cmd.AddCommand(epicNewCmd(), epicRollupCmd())
	return cmd
}

// epicRollupCmd reports (or with --write, applies) each epic's status computed
// from its child tasks.
func epicRollupCmd() *cobra.Command {
	var write bool
	cmd := &cobra.Command{
		Use:   "rollup [slug]",
		Short: "Show each epic's status computed from its child tasks (--write to apply)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			nodes, err := s.ListNodes()
			if err != nil {
				return err
			}
			only := ""
			if len(args) == 1 {
				only = args[0]
			}
			var shown int
			for i := range nodes {
				e := nodes[i]
				if e.Kind() != "epic" || (only != "" && e.Slug != only) {
					continue
				}
				shown++
				want := core.EpicRollupStatus(nodes, e.Slug)
				if want == "" {
					fmt.Printf("%-40s %-9s (no child tasks)\n", e.Slug, e.FM.Status)
					continue
				}
				mark := "="
				if want != e.FM.Status {
					mark = "→"
				}
				fmt.Printf("%-40s %-9s %s %s\n", e.Slug, e.FM.Status, mark, want)
				if write && want != e.FM.Status {
					e.FM.Status = want
					if err := e.Save(); err != nil {
						return err
					}
				}
			}
			if only != "" && shown == 0 {
				return fmt.Errorf("no epic with slug %q", only)
			}
			if write {
				fmt.Println("✓ applied computed statuses")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "write the computed status back to each epic")
	return cmd
}

func epicNewCmd() *cobra.Command {
	var horizon, summary, body string
	var tags []string
	cmd := &cobra.Command{
		Use:   "new \"Title\"",
		Short: "Create a new epic (status: planned)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			e, err := s.NewNode("epic", core.Frontmatter{
				Title:   strings.Join(args, " "),
				Status:  "planned",
				Horizon: horizon,
				Tags:    tags,
				Summary: summary,
			}, body)
			if err != nil {
				return err
			}
			fmt.Printf("✓ epic created: %s\n", rel(mustWd(), e.Path))
			fmt.Printf("  slug: %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&horizon, "horizon", "", "planning horizon (now|next|later)")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary")
	cmd.Flags().StringVar(&body, "body", "", "initial body markdown")
	return cmd
}

// decideCmd creates a decision node.
func decideCmd() *cobra.Command {
	var supersedes []string
	var entry, summary, body string
	var tags []string
	cmd := &cobra.Command{
		Use:   "decide \"Title\"",
		Short: "Record a decision (status: accepted)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			e, err := s.NewNode("decision", core.Frontmatter{
				Title:      strings.Join(args, " "),
				Status:     "accepted",
				Date:       core.Today(),
				Supersedes: supersedes,
				Entry:      entry,
				Tags:       tags,
				Summary:    summary,
			}, body)
			if err != nil {
				return err
			}
			fmt.Printf("✓ decision recorded: %s\n", rel(mustWd(), e.Path))
			fmt.Printf("  slug: %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&supersedes, "supersedes", nil, "slug(s) of decision(s) this replaces")
	cmd.Flags().StringVar(&entry, "entry", "", "entry slug that occasioned this decision")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary")
	cmd.Flags().StringVar(&body, "body", "", "initial body markdown")
	return cmd
}

// articleCmd groups the article subcommands.
func articleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "article",
		Short: "Work with articles",
	}
	cmd.AddCommand(articleNewCmd())
	return cmd
}

func articleNewCmd() *cobra.Command {
	var tags []string
	var summary, body string
	cmd := &cobra.Command{
		Use:   "new \"Title\"",
		Short: "Create a new article",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore()
			if err != nil {
				return err
			}
			e, err := s.NewNode("article", core.Frontmatter{
				Title:   strings.Join(args, " "),
				Tags:    tags,
				Summary: summary,
			}, body)
			if err != nil {
				return err
			}
			fmt.Printf("✓ article created: %s\n", rel(mustWd(), e.Path))
			fmt.Printf("  slug: %s\n", e.Slug)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary")
	cmd.Flags().StringVar(&body, "body", "", "initial body markdown")
	return cmd
}
