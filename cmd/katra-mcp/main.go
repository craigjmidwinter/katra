// Command katra-mcp serves the katra operations over the Model Context
// Protocol (stdio), so agents can drive the dev log structurally.
package main

import (
	"fmt"
	"os"

	"github.com/craigjmidwinter/katra/internal/buildinfo"
	"github.com/craigjmidwinter/katra/internal/mcpserver"
)

// version is stamped at link time, exactly as in cmd/katra. It is reported to
// the MCP client in the initialize handshake, so a client log names the build.
var version = "dev"

func main() {
	if err := mcpserver.Run(buildinfo.Resolve(version)); err != nil {
		fmt.Fprintln(os.Stderr, "katra-mcp:", err)
		os.Exit(1)
	}
}
