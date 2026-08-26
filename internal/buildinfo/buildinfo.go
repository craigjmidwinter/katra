// Package buildinfo resolves the version string a katra binary reports.
//
// There are two ways a katra gets built and only one of them can stamp a
// version. `make build` and the release build pass
// -ldflags "-X main.version=…"; the `go` tool cannot, so a binary from
// `go install github.com/craigjmidwinter/katra/cmd/katra@v0.1.0` used to report
// "dev" — from the one install path that works before a release exists, which
// is exactly when a bug report is least able to spare the ambiguity.
//
// The module version is available at runtime regardless, so this fills the gap.
package buildinfo

import "runtime/debug"

// Stamped is the value a link-time -X assignment leaves behind when it did not
// happen. It is the sentinel, not a fallback anyone should see.
const Stamped = "dev"

// Resolve returns the version to report. A link-time stamp always wins: it is
// the more precise answer, carrying `git describe`'s commit and dirty marker on
// a development build. Only when that is absent does the module version fill
// in, which happens for `go install` and for anything built as a dependency.
//
// "(devel)" is what the module system reports for a build from a working tree
// with no version, which is no more informative than the sentinel, so it is
// treated as absent too.
func Resolve(stamped string) string {
	if stamped != "" && stamped != Stamped {
		return stamped
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return Stamped
	}
	return info.Main.Version
}
