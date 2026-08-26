// Command katra is a committed, rich-component dev log you write as you build.
package main

import (
	"github.com/craigjmidwinter/katra/internal/buildinfo"
	"github.com/craigjmidwinter/katra/internal/cli"
)

// version is stamped at link time:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always --dirty)"
//
// `make build` and the release build both do this. The `go` tool cannot, so a
// `go install` binary falls back to the module version — see internal/buildinfo.
var version = "dev"

func main() {
	cli.Execute(buildinfo.Resolve(version))
}
