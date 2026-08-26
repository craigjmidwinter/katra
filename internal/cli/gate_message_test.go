package cli

import (
	"strings"
	"testing"
)

// The gate used to print one message for two situations. "You have a step left"
// and "this tool is broken, here is the way out" are different things to tell
// someone, and only the second lets them stop trying. A session in chesscast
// spent seven blocked commits discovering the difference the hard way.

func TestGateMessageSatisfiableCase(t *testing.T) {
	msg := gateMessage(nil)

	if !strings.Contains(msg, "katra reconcile --advance/--close") {
		t.Error("the ordinary message must name the command that fixes it")
	}
	// The person is not stuck, so the message must not tell them katra is broken.
	for _, wrong := range []string{"katra bug", "will not help", "Please report"} {
		if strings.Contains(msg, wrong) {
			t.Errorf("satisfiable case says %q, which sends the person to the issue tracker for nothing", wrong)
		}
	}
}

func TestGateMessageUnsatisfiableCase(t *testing.T) {
	msg := gateMessage([]string{"y.go", "gen/schema.go"})

	// Name the paths: "something is invisible" is not actionable, and the paths
	// are what a bug report needs.
	for _, p := range []string{"y.go", "gen/schema.go"} {
		if !strings.Contains(msg, p) {
			t.Errorf("message does not name the invisible path %q", p)
		}
	}
	// Say plainly that declaring is futile, that it is katra's fault, and how to
	// get out. Each of these is a thing the old message failed to say.
	for _, want := range []string{"declaring will not help", "katra bug", "not something you did wrong", "--no-verify", "issues"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message is missing %q", want)
		}
	}
}
