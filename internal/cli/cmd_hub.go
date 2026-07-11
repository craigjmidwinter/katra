package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/craigjmidwinter/katra/internal/core"
	"github.com/craigjmidwinter/katra/internal/viewer"
	"github.com/spf13/cobra"
)

const hubLaunchLabel = "com.katra.hub"

// hubCmd groups cross-project ("hub") commands: list every registered katra and
// serve them all from one URL.
func hubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Work across every registered katra",
	}
	cmd.AddCommand(hubListCmd(), hubServeCmd(), hubInstallCmd(), hubUninstallCmd())
	return cmd
}

// hubPlistPath is the launchd agent file location.
func hubPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", hubLaunchLabel+".plist"), nil
}

// hubPlist renders the launchd agent plist that keeps `katra hub serve` running.
func hubPlist(binary string, port int, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>hub</string>
    <string>serve</string>
    <string>--port</string>
    <string>%d</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, hubLaunchLabel, binary, port, logPath, logPath)
}

func hubInstallCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install a launchd agent so the hub runs at login (macOS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("hub install is only supported on macOS")
			}
			bin, err := os.Executable()
			if err != nil {
				return err
			}
			home, _ := os.UserHomeDir()
			logPath := filepath.Join(home, "Library", "Logs", "katra-hub.log")
			plistPath, err := hubPlistPath()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(plistPath, []byte(hubPlist(bin, port, logPath)), 0o644); err != nil {
				return err
			}
			// Reload cleanly: unload any existing, then load.
			_ = exec.Command("launchctl", "unload", plistPath).Run()
			if out, err := exec.Command("launchctl", "load", "-w", plistPath).CombinedOutput(); err != nil {
				return fmt.Errorf("launchctl load: %v: %s", err, out)
			}
			fmt.Printf("✓ hub agent installed → %s\n", plistPath)
			fmt.Printf("  serving on http://localhost:%d/ at login; logs at %s\n", port, logPath)
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 4200, "port the agent serves on")
	return cmd
}

func hubUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the hub launchd agent (macOS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("hub uninstall is only supported on macOS")
			}
			plistPath, err := hubPlistPath()
			if err != nil {
				return err
			}
			_ = exec.Command("launchctl", "unload", plistPath).Run()
			if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Printf("✓ hub agent removed\n")
			return nil
		},
	}
}

// hubProjects builds the served project list from the global registry, pruning
// missing stores and assigning each a stable, unique URL id.
func hubProjects() ([]viewer.HubProject, error) {
	r, err := core.LoadRegistry()
	if err != nil {
		return nil, err
	}
	if len(r.Prune()) > 0 {
		_ = r.Save()
	}
	used := map[string]bool{}
	var projects []viewer.HubProject
	for _, dir := range r.Projects {
		s, err := core.FindStore(dir)
		if err != nil {
			continue
		}
		projects = append(projects, viewer.HubProject{ID: projectID(dir, used), Store: s})
	}
	return projects, nil
}

// projectID derives a stable URL slug from a store dir: the repo name (the dir
// holding the katra/), de-duplicated with a numeric suffix on collision.
func projectID(storeDir string, used map[string]bool) string {
	base := filepath.Base(storeDir)
	if base == core.DefaultDirName || base == core.LegacyDirName {
		base = filepath.Base(filepath.Dir(storeDir))
	}
	id := core.Slugify(base)
	if id == "" {
		id = "katra"
	}
	orig := id
	for n := 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", orig, n)
	}
	used[id] = true
	return id
}

func hubServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve every registered katra from one URL (LAN-reachable)",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := hubProjects()
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				return fmt.Errorf("no katras registered — run `katra init` in a repo first")
			}
			return viewer.ServeHub(projects, port)
		},
	}
	cmd.Flags().IntVar(&port, "port", 4200, "port to serve on")
	return cmd
}

func hubListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered katra projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := core.LoadRegistry()
			if err != nil {
				return err
			}
			if stale := r.Prune(); len(stale) > 0 {
				for _, p := range stale {
					fmt.Printf("  (dropped missing: %s)\n", p)
				}
				_ = r.Save()
			}
			if len(r.Projects) == 0 {
				fmt.Println("No katras registered yet — `katra init` registers one.")
				return nil
			}
			for _, dir := range r.Projects {
				s, err := core.FindStore(dir)
				if err != nil {
					fmt.Printf("%-28s %s  (unreadable: %v)\n", "?", dir, err)
					continue
				}
				entries, _ := s.ListNodes("entry")
				tasks, _ := s.ListNodes("task")
				var drafts, doing int
				for _, e := range entries {
					if e.IsDraft() {
						drafts++
					}
				}
				for _, t := range tasks {
					if t.FM.Status == "doing" {
						doing++
					}
				}
				fmt.Printf("%-28s %s  (%d entries, %d drafts, %d doing)\n",
					truncate(s.Config.Title, 28), dir, len(entries), drafts, doing)
			}
			return nil
		},
	}
}
