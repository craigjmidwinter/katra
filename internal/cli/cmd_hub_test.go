package cli

import (
	"strings"
	"testing"
)

func TestHubPlist(t *testing.T) {
	p := hubPlist("/usr/local/bin/katra", 4200, "/tmp/katra-hub.log")
	for _, want := range []string{
		"<string>com.katra.hub</string>",
		"<string>/usr/local/bin/katra</string>",
		"<string>hub</string>",
		"<string>serve</string>",
		"<string>4200</string>",
		"<string>/tmp/katra-hub.log</string>",
		"<key>RunAtLoad</key><true/>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist missing %q\n%s", want, p)
		}
	}
}
