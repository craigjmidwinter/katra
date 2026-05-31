// Command devlog-mcp serves the devlog operations over the Model Context
// Protocol (stdio), so agents can drive the dev log structurally.
package main

import (
	"fmt"
	"os"

	"github.com/craigjmidwinter/devlog/internal/mcpserver"
)

func main() {
	if err := mcpserver.Run("0.1.0"); err != nil {
		fmt.Fprintln(os.Stderr, "devlog-mcp:", err)
		os.Exit(1)
	}
}
