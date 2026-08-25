package buildinfo

import "testing"

// TestResolvePrefersLinkTimeStamp: a stamped build is the more precise answer,
// so the module version must never override it.
func TestResolvePrefersLinkTimeStamp(t *testing.T) {
	for _, stamped := range []string{"v0.1.0", "1c9c9ac-dirty", "v1.2.3-4-gabc1234"} {
		if got := Resolve(stamped); got != stamped {
			t.Errorf("Resolve(%q) = %q, want the stamp back unchanged", stamped, got)
		}
	}
}

// TestResolveFallsBackToModuleVersion is the bug this package exists for: an
// unstamped build must still be able to name itself.
//
// Under `go test` the module version reads "(devel)", so the honest assertion
// is that Resolve degrades to the sentinel rather than to empty or to a panic —
// the real fallback path is exercised by TestVersionInGoInstallBuild, which
// actually runs `go install`.
func TestResolveFallsBackToModuleVersion(t *testing.T) {
	for _, absent := range []string{"", Stamped} {
		got := Resolve(absent)
		if got == "" {
			t.Errorf("Resolve(%q) returned empty; a version string is always expected", absent)
		}
		if got != Stamped && got == "(devel)" {
			t.Errorf("Resolve(%q) = %q; the module system's placeholder is not a version", absent, got)
		}
	}
}
