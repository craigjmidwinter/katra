// Command katra is a committed, rich-component dev log you write as you build.
package main

import "github.com/craigjmidwinter/katra/internal/cli"

// version is stamped at link time:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always --dirty)"
//
// `make build` and the release build both do this. The `go` tool cannot, so a
// `go install` binary reports "dev" — which is why the bug template asks for a
// commit as well as a version.
var version = "dev"

func main() {
	cli.Execute(version)
}
