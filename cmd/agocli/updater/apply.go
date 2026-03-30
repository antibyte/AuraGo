package updater

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/cmd/agocli/shared"
)

// ApplyGit updates via git: fetch → clean tracked changes → merge --ff-only.
func ApplyGit(cfg *Config) error {
	dir := cfg.InstallDir

	// Reset any tracked changes that would prevent merge
	cfg.log("Resetting tracked changes...")
	gitRun(dir, "checkout", "--", ".")

	// Merge
	cfg.log("Merging latest changes...")
	cmd := exec.Command("git", "merge", "--ff-only", "origin/main")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git merge: %s: %w", strings.TrimSpace(string(out)), err)
	}
	cfg.log("Git merge successful")
	return nil
}

// ApplyBinary updates by downloading binaries from the latest GitHub release.
func ApplyBinary(cfg *Config) error {
	release, err := shared.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("get latest release: %w", err)
	}
	cfg.log("Latest release: " + release.TagName)

	arch := runtime.GOARCH
	binDir := filepath.Join(cfg.InstallDir, "bin")

	// Download architecture-specific binaries
	binaries := []struct {
		assetName string
		destName  string
	}{
		{fmt.Sprintf("aurago_linux_%s", arch), "aurago_linux"},
		{fmt.Sprintf("lifeboat_linux_%s", arch), "lifeboat_linux"},
		{fmt.Sprintf("config-merger_linux_%s", arch), "config-merger_linux"},
		{fmt.Sprintf("aurago-remote_linux_%s", arch), "aurago-remote_linux"},
		{fmt.Sprintf("agocli_linux_%s", arch), "agocli_linux"},
	}

	for _, bin := range binaries {
		asset := release.FindAsset(bin.assetName)
		if asset == nil {
			cfg.log("  Asset not found: " + bin.assetName + " — skipping")
			continue
		}
		dest := filepath.Join(binDir, bin.destName)
		cfg.log("  Downloading " + bin.assetName + "...")
		if err := shared.DownloadFile(asset.BrowserDownloadURL, dest, nil); err != nil {
			cfg.log("  WARNING: Failed to download " + bin.assetName + ": " + err.Error())
			continue
		}
		// Make executable
		exec.Command("chmod", "+x", dest).Run()
	}

	// Download resources.dat if present
	resAsset := release.FindAsset("resources.dat")
	if resAsset != nil {
		cfg.log("  Downloading resources.dat...")
		dest := filepath.Join(cfg.InstallDir, "resources.dat")
		if err := shared.DownloadFile(resAsset.BrowserDownloadURL, dest, nil); err != nil {
			cfg.log("  WARNING: resources.dat download failed: " + err.Error())
		}
	}

	return nil
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
