package setup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Run performs the full first-time setup:
// 1. Extract resources.dat into the working directory
// 2. Generate a master key if not set
// 3. Install as OS service
func Run(logger *slog.Logger) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable symlinks: %w", err)
	}
	installDir := filepath.Dir(exePath)

	logger.Info("AuraGo Setup", "install_dir", installDir)

	// ── Step 1: Extract resources.dat (optional) ────────────────────────
	// resources.dat is only present during first installation. On subsequent
	// starts (e.g. after an update that replaced the binary but not the
	// resources) the file may have been deleted. We skip extraction in that
	// case — the app can run with existing files on disk.
	resPath := filepath.Join(installDir, "resources.dat")
	if _, err := os.Stat(resPath); err == nil {
		logger.Info("Extracting resources.dat ...")
		if err := extractTarGz(resPath, installDir); err != nil {
			return fmt.Errorf("failed to extract resources.dat: %w", err)
		}
		logger.Info("Resources extracted successfully")
	} else {
		logger.Warn("resources.dat not found — skipping extraction, using existing files on disk", "path", resPath)
	}

	// ── Step 2: Generate master key if not present ───────────────────────
	envFile := filepath.Join(installDir, ".env")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("failed to generate master key: %w", err)
		}
		hexKey := hex.EncodeToString(key)
		content := fmt.Sprintf("AURAGO_MASTER_KEY=%s\n", hexKey)
		if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write .env file: %w", err)
		}
		logger.Info("Generated new master key in .env")

		// Also set it for the current process so service install can validate
		os.Setenv("AURAGO_MASTER_KEY", hexKey)
	} else {
		logger.Info(".env already exists, skipping key generation")
	}

	// ── Step 3: Ensure required directories exist ────────────────────────
	dirs := []string{
		"data", "data/vectordb", "log",
		"agent_workspace/skills",
		"agent_workspace/tools",
		"agent_workspace/workdir",
		"agent_workspace/workdir/attachments",
	}
	for _, d := range dirs {
		p := filepath.Join(installDir, d)
		if err := os.MkdirAll(p, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", p, err)
		}
	}
	logger.Info("Directory structure verified")

	// ── Step 4: Install OS service ───────────────────────────────────────
	logger.Info("Installing system service ...")
	if err := installService(exePath, installDir, logger); err != nil {
		logger.Warn("Service installation failed (non-fatal — you can start AuraGo manually)", "error", err)
	} else {
		logger.Info("System service installed successfully")
	}

	logger.Info("━━━ Setup complete! ━━━")
	logger.Info("Next steps:")
	logger.Info("  1. Edit config.yaml with your LLM API key and preferences")
	logger.Info("  2. Set AURAGO_MASTER_KEY from .env in your shell (or the service reads it automatically)")
	logger.Info("  3. Start AuraGo: ./aurago  or  systemctl start aurago")

	return nil
}

// NeedsSetup returns true if essential runtime files are missing.
// Only config.yaml is checked — it is the one file that cannot be
// auto-generated. Other directories (agent_workspace/skills, data, etc.)
// are ensured by EnsureDirectories() or created on-the-fly during startup.
func NeedsSetup(installDir string) bool {
	if _, err := os.Stat(filepath.Join(installDir, "config.yaml")); os.IsNotExist(err) {
		return true
	}
	return false
}

// EnsureDirectories creates required runtime directories under installDir.
// This is safe to call on every startup — it is a no-op when directories
// already exist. It prevents missing-directory crashes after updates that
// replace the binary but don't re-extract resources.dat.
func EnsureDirectories(installDir string, logger *slog.Logger) {
	dirs := []string{
		"data", "data/vectordb", "log",
		"agent_workspace/skills",
		"agent_workspace/tools",
		"agent_workspace/workdir",
		"agent_workspace/workdir/attachments",
	}
	for _, d := range dirs {
		p := filepath.Join(installDir, d)
		if err := os.MkdirAll(p, 0750); err != nil {
			logger.Warn("Failed to create directory", "dir", p, "error", err)
		}
	}
}

// ── tar.gz extraction ────────────────────────────────────────────────────

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name))

		// Security: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("illegal path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return err
			}
			// Don't overwrite config.yaml if it already exists (user may have edited it)
			if filepath.Base(target) == "config.yaml" {
				if _, err := os.Stat(target); err == nil {
					continue
				}
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0640)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

// ── Platform service installation ────────────────────────────────────────

func installService(exePath, installDir string, logger *slog.Logger) error {
	switch runtime.GOOS {
	case "linux":
		return installSystemd(exePath, installDir, logger)
	case "darwin":
		return installLaunchd(exePath, installDir, logger)
	case "windows":
		return installWindowsTask(exePath, installDir, logger)
	default:
		return fmt.Errorf("unsupported OS for service installation: %s", runtime.GOOS)
	}
}

// ── Linux: systemd ──────────────────────────────────────────────────────

func installSystemd(exePath, installDir string, logger *slog.Logger) error {
	// Determine the actual user (fallback from sudo if applicable)
	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	if user == "" {
		user = "root" // absolute fallback
	}

	unit := fmt.Sprintf(`[Unit]
Description=AuraGo AI Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=10
EnvironmentFile=-%s/.env

[Install]
WantedBy=multi-user.target
`, user, user, installDir, exePath, installDir)

	unitPath := "/etc/systemd/system/aurago.service"
	if err := os.WriteFile(unitPath, []byte(unit), 0600); err != nil {
		return fmt.Errorf("failed to write systemd unit (run setup as root?): %w", err)
	}

	for _, cmd := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "aurago.service"},
	} {
		if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
			logger.Warn("systemctl command failed", "cmd", strings.Join(cmd, " "), "output", string(out), "error", err)
		}
	}
	return nil
}

// ── macOS: launchd ──────────────────────────────────────────────────────

func installLaunchd(exePath, installDir string, logger *slog.Logger) error {
	masterKey := readEnvKey(filepath.Join(installDir, ".env"), "AURAGO_MASTER_KEY")

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.aurago.agent</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>AURAGO_MASTER_KEY</key>
		<string>%s</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s/log/aurago_stdout.log</string>
	<key>StandardErrorPath</key>
	<string>%s/log/aurago_stderr.log</string>
</dict>
</plist>
`, exePath, installDir, masterKey, installDir, installDir)

	// Install for current user (no root needed)
	home, _ := os.UserHomeDir()
	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	os.MkdirAll(plistDir, 0755)
	plistPath := filepath.Join(plistDir, "com.aurago.agent.plist")

	if err := os.WriteFile(plistPath, []byte(plist), 0600); err != nil {
		return fmt.Errorf("failed to write launchd plist: %w", err)
	}

	if out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
		logger.Warn("launchctl load failed", "output", string(out), "error", err)
	}
	return nil
}

// ── Windows: Task Scheduler ─────────────────────────────────────────────

func installWindowsTask(exePath, installDir string, logger *slog.Logger) error {
	// Use schtasks to create a task that runs at logon
	taskName := "AuraGo"

	// Delete existing task if present (ignore errors)
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

	cmd := exec.Command("schtasks", "/Create",
		"/TN", taskName,
		"/TR", fmt.Sprintf(`"%s"`, exePath),
		"/SC", "ONLOGON",
		"/RL", "HIGHEST",
		"/F",
	)
	cmd.Dir = installDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks failed: %s — %w", string(out), err)
	}

	// Also create a batch wrapper that sets the env and starts the binary
	batContent := fmt.Sprintf(`@echo off
cd /d "%s"
for /f "tokens=1,* delims==" %%%%a in (.env) do set "%%%%a=%%%%b"
start "" "%s"
`, installDir, exePath)
	batPath := filepath.Join(installDir, "start_aurago.bat")
	os.WriteFile(batPath, []byte(batContent), 0644)

	logger.Info("Windows scheduled task created", "task", taskName)
	return nil
}

// ── Helpers ─────────────────────────────────────────────────────────────

func readEnvKey(envPath, key string) string {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			val := strings.TrimPrefix(line, key+"=")
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}
