package cli

import (
	"os/exec"
	"strings"
	"testing"
)

// Every hook katra writes must be a silent no-op where katra is absent.
//
// katra's own repository is public and commits .claude/settings.json, so before
// this the file handed every contributor seven hooks calling a binary they did
// not have — `katra: command not found` on every Bash command, every edit and
// every prompt. You needed katra installed to contribute to katra, and katra's
// installer put it there.
func TestEveryWrittenHookIsGuarded(t *testing.T) {
	hooks := map[string]string{
		"session-start": hookSessionStart,
		"turn-start":    hookTurnStart,
		"post-tool":     hookPostTool,
		"stop":          hookStop,
		"pre-compact":   hookPreCompact,
		"session-end":   hookSessionEnd,
		"pre-commit":    hookPreCommit,
	}
	for name, cmd := range hooks {
		if !strings.HasPrefix(cmd, "command -v katra ") {
			t.Errorf("%s hook does not check for katra first: %q", name, cmd)
		}
		if !strings.HasSuffix(cmd, "|| exit 0") {
			t.Errorf("%s hook does not fall back to a no-op: %q", name, cmd)
		}
		// exec, or the wrapping shell swallows the hook's exit code -- which for
		// pre-commit is how it blocks. A guard that disabled the gate it guards
		// would be worse than the bug it fixes.
		if !strings.Contains(cmd, "exec katra ") {
			t.Errorf("%s hook does not exec, so its exit code cannot reach the harness: %q", name, cmd)
		}
	}
}

// TestGuardedHookIsSilentWithoutKatra runs the real command line on a PATH that
// has no katra, which is the contributor's first five minutes.
func TestGuardedHookIsSilentWithoutKatra(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	cmd := exec.Command("sh", "-c", hookPreCommit)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.Stdin = strings.NewReader("")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("guarded hook failed with katra absent: %v\n%s", err, out)
	}
	if len(strings.TrimSpace(string(out))) != 0 {
		t.Errorf("guarded hook printed something with katra absent: %q", out)
	}
}
