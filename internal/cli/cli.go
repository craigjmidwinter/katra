// Package cli wires the katra subcommands. Every command is a thin shell over
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

// Execute runs the katra CLI.
func Execute() {
	root := &cobra.Command{
		Use:           "katra",
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
		taskCmd(),
		epicCmd(),
		decideCmd(),
		articleCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// resolveStore finds the katra for the current directory (or $KATRA_DIR,
// falling back to the legacy $DEVLOG_DIR).
func resolveStore() (*core.Store, error) {
	if env := core.EnvDir(); env != "" {
		return core.FindStore(env)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return core.FindStore(wd)
}
