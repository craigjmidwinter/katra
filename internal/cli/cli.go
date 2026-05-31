// Package cli wires the devlog subcommands. Every command is a thin shell over
// internal/core (and internal/viewer); the same operations are also exposed
// over MCP, so the logic stays in core, not here.
package cli

import (
	"fmt"
	"os"

	"github.com/craigjmidwinter/devlog/internal/core"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

// Execute runs the devlog CLI.
func Execute() {
	root := &cobra.Command{
		Use:           "devlog",
		Short:         "A committed, rich-component dev log you write as you build",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		initCmd(),
		newCmd(),
		appendCmd(),
		stampCmd(),
		captureCmd(),
		compareCmd(),
		listCmd(),
		serveCmd(),
		buildCmd(),
		hookCmd(),
		doctorCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// resolveStore finds the devlog for the current directory (or $DEVLOG_DIR).
func resolveStore() (*core.Store, error) {
	if env := os.Getenv("DEVLOG_DIR"); env != "" {
		return core.FindStore(env)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return core.FindStore(wd)
}
