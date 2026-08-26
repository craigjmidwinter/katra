package cli

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/spf13/cobra"
)

//go:embed embed/SKILL.md
var katraSkill string

// Hook command strings written into .claude/settings.json. All are tagged with
// "katra" so setup can find and replace its own entries idempotently. Every hook
// routes through the single `katra agent-hook <event>` adapter (fail-open).
const (
	hookSessionStart = `katra agent-hook session-start`
	hookTurnStart    = `katra agent-hook turn-start`
	hookPostTool     = `katra agent-hook post-tool`
	hookStop         = `katra agent-hook stop`
	hookPreCompact   = `katra agent-hook snapshot --event pre-compact`
	hookSessionEnd   = `katra agent-hook snapshot --event session-end`
	// hookPreCommit is the coverage gate: a PreToolUse(Bash) check that blocks a
	// `git commit` whose staged code has no reconciliation receipt.
	hookPreCommit = `katra agent-hook pre-commit`
)

func setupCmd() *cobra.Command {
	var noGate bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install Katra's Claude Code integration plus portable Git auto-stamp",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, _ := os.Getwd()

			// 1. Ensure a store exists.
			s, err := resolveStore()
			if err != nil {
				base := wd
				if root := gitToplevel(wd); root != "" {
					base = root
				}
				dir := filepath.Join(base, core.DefaultDirName)
				title := titleCase(filepath.Base(base))
				s, err = core.InitStore(dir, title)
				if err != nil {
					return fmt.Errorf("init store: %w", err)
				}
				fmt.Printf("✓ created katra store → %s\n", rel(wd, dir))
			} else {
				fmt.Printf("✓ katra store → %s\n", rel(wd, s.Dir))
			}

			root := filepath.Dir(s.Dir) // repo dir containing the store
			if r, err := s.RepoRoot(); err == nil {
				root = r
			}

			// 2. Install the skill.
			skillPath := filepath.Join(root, ".claude", "skills", "katra", "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(skillPath, []byte(katraSkill), 0o644); err != nil {
				return err
			}
			fmt.Printf("✓ skill → %s\n", rel(wd, skillPath))

			// 3. Merge Claude Code hooks into .claude/settings.json.
			settingsPath := filepath.Join(root, ".claude", "settings.json")
			if err := mergeKatraHooks(settingsPath, !noGate); err != nil {
				return fmt.Errorf("settings.json: %w", err)
			}
			gate := "with commit gate"
			if noGate {
				gate = "no commit gate"
			}
			fmt.Printf("✓ hooks → %s (session nudges + %s)\n", rel(wd, settingsPath), gate)

			// 4. Git post-commit auto-stamp hook.
			if p, err := s.InstallHook(); err != nil {
				fmt.Printf("  (git hook not installed: %v)\n", err)
			} else {
				fmt.Printf("✓ git post-commit auto-stamp → %s\n", rel(wd, p))
			}

			// 5. Register with the hub.
			if err := core.Register(s.Dir); err == nil {
				fmt.Printf("✓ registered with the katra hub\n")
			}

			fmt.Printf("\nDone. Restart this session (or start a new one) to pick up the skill + hooks.\n")
			return nil
		},
	}
	cmd.Flags().BoolVar(&noGate, "no-gate", false, "install session nudges but not the blocking commit gate")
	return cmd
}

// mergeKatraHooks idempotently writes katra's hooks into a Claude Code
// settings.json, preserving any other settings and replacing only katra's own
// prior entries (identified by a "katra" substring in the command).
func mergeKatraHooks(path string, gate bool) error {
	settings := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &settings)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	// mine builds a single hook-group entry with our command.
	group := func(command string, matcher string) map[string]any {
		g := map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": command}},
		}
		if matcher != "" {
			g["matcher"] = matcher
		}
		return g
	}
	// groupAsync is like group but marks the hook async (non-blocking) — used for
	// the SessionEnd memory snapshot, which must never delay session teardown.
	groupAsync := func(command string, matcher string) map[string]any {
		g := map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": command, "async": true}},
		}
		if matcher != "" {
			g["matcher"] = matcher
		}
		return g
	}
	// setEvent drops any existing katra-owned groups for an event, then appends ours.
	setEvent := func(event string, mine map[string]any) {
		var kept []any
		if existing, ok := hooks[event].([]any); ok {
			for _, e := range existing {
				if !groupIsKatra(e) {
					kept = append(kept, e)
				}
			}
		}
		hooks[event] = append(kept, mine)
	}

	setEvent("SessionStart", group(hookSessionStart, ""))
	setEvent("UserPromptSubmit", group(hookTurnStart, ""))
	setEvent("PostToolUse", group(hookPostTool, "Edit|Write"))
	setEvent("Stop", group(hookStop, ""))
	setEvent("PreCompact", group(hookPreCompact, ""))
	setEvent("SessionEnd", groupAsync(hookSessionEnd, ""))
	if gate {
		setEvent("PreToolUse", group(hookPreCommit, "Bash"))
	} else if existing, ok := hooks["PreToolUse"].([]any); ok {
		// drop any prior katra gate, keep other PreToolUse hooks
		var kept []any
		for _, e := range existing {
			if !groupIsKatra(e) {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(hooks, "PreToolUse")
		} else {
			hooks["PreToolUse"] = kept
		}
	}

	settings["hooks"] = hooks
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

// groupIsKatra reports whether a hook-group entry is one katra manages.
func groupIsKatra(e any) bool {
	g, ok := e.(map[string]any)
	if !ok {
		return false
	}
	list, _ := g["hooks"].([]any)
	for _, h := range list {
		hm, _ := h.(map[string]any)
		if c, _ := hm["command"].(string); strings.Contains(c, "katra") {
			return true
		}
	}
	return false
}
