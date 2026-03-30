// Package updater implements the native Go update engine for AuraGo,
// replacing the previous shell-script wrapper (update.sh --yes).
package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aurago/cmd/agocli/shared"
)

// Config holds parameters for an update run.
type Config struct {
	InstallDir string
	NoRestart  bool
	Yes        bool // skip confirmation prompts
	LogFn      func(string)
}

func (c *Config) log(msg string) {
	if c.LogFn != nil {
		c.LogFn(msg)
	}
}

// IsGitMode returns true if the install dir is a git repo.
func (c *Config) IsGitMode() bool {
	return shared.IsGitRepo(c.InstallDir)
}

// CheckForUpdates checks whether an update is available.
// Returns (updateAvailable, latestVersion, changelog, error).
func CheckForUpdates(cfg *Config) (bool, string, string, error) {
	if cfg.IsGitMode() {
		cfg.log("Git repository detected — checking remote...")
		cmd := exec.Command("git", "fetch", "origin", "main")
		cmd.Dir = cfg.InstallDir
		if err := cmd.Run(); err != nil {
			return false, "", "", fmt.Errorf("git fetch: %w", err)
		}

		localHash := gitOutput(cfg.InstallDir, "rev-parse", "HEAD")
		remoteHash := gitOutput(cfg.InstallDir, "rev-parse", "origin/main")

		if localHash == remoteHash {
			return false, localHash[:8], "", nil
		}

		changelog := gitOutput(cfg.InstallDir, "log", "HEAD..origin/main", "--oneline")
		return true, remoteHash[:min(len(remoteHash), 8)], changelog, nil
	}

	// Binary mode — check GitHub releases
	cfg.log("Checking GitHub releases...")
	tag, err := shared.GetLatestReleaseTag()
	if err != nil {
		return false, "", "", err
	}
	return true, tag, "", nil // can't easily compare versions without local tag
}

// Run executes the full update process.
func Run(cfg *Config) error {
	// 1. Acquire lock
	cfg.log("Acquiring update lock...")
	lock, err := AcquireLock()
	if err != nil {
		return fmt.Errorf("another update is running: %w", err)
	}
	defer lock.Release()

	// 2. Stop service
	cfg.log("Stopping AuraGo service...")
	if err := shared.StopService(); err != nil {
		cfg.log("WARNING: Could not stop service: " + err.Error())
	}

	// 3. Clean up stale files
	cfg.log("Cleaning up stale locks...")
	CleanLockFiles(cfg.InstallDir)
	CleanPorts()

	// 4. Backup
	cfg.log("Creating backup...")
	backupDir, err := CreateBackup(cfg.InstallDir)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	cfg.log("Backup created at " + backupDir)

	// 5. Apply update
	if cfg.IsGitMode() {
		cfg.log("Applying git update...")
		if err := ApplyGit(cfg); err != nil {
			cfg.log("ERROR: Git update failed, restoring backup...")
			RestoreBackup(backupDir, cfg.InstallDir)
			return fmt.Errorf("git update: %w", err)
		}
	} else {
		cfg.log("Downloading latest release...")
		if err := ApplyBinary(cfg); err != nil {
			cfg.log("ERROR: Binary update failed, restoring backup...")
			RestoreBackup(backupDir, cfg.InstallDir)
			return fmt.Errorf("binary update: %w", err)
		}
	}

	// 6. Run config-merger
	cfg.log("Merging configuration...")
	if err := MergeConfig(cfg); err != nil {
		cfg.log("WARNING: Config merge failed: " + err.Error())
	}

	// 7. Build binaries (git mode)
	if cfg.IsGitMode() {
		cfg.log("Building binaries...")
		if err := BuildBinaries(cfg); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
	}

	// 8. Restart
	if !cfg.NoRestart {
		cfg.log("Starting AuraGo...")
		if err := shared.StartService(cfg.InstallDir); err != nil {
			cfg.log("WARNING: Could not start service: " + err.Error())
		} else {
			cfg.log("AuraGo started successfully!")
		}
	} else {
		cfg.log("Skipping restart (--no-restart)")
	}

	// 9. Clean up backup
	os.RemoveAll(backupDir)
	cfg.log("Update complete!")
	return nil
}

func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CleanLockFiles removes stale lock files.
func CleanLockFiles(installDir string) {
	locks := []string{
		filepath.Join(installDir, "data", "aurago.lock"),
		filepath.Join(installDir, "data", "maintenance.lock"),
		filepath.Join(installDir, ".git", "index.lock"),
	}
	for _, l := range locks {
		os.Remove(l)
	}
}

// CleanPorts tries to free ports 8080-8099.
func CleanPorts() {
	// Best effort — fuser may not be available
	for port := 8080; port <= 8099; port++ {
		exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port)).Run()
	}
}
