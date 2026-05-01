package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const launchAgentPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>io.aurago.remote</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.ExePath}}</string>
		<string>--foreground</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>WorkingDirectory</key>
	<string>{{.WorkDir}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath}}</string>
	<key>StandardOutPath</key>
	<string>{{.LogPath}}</string>
</dict>
</plist>
`

func getInstallPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Application Support", "AuraGo")
	return filepath.Join(dir, "aurago-remote"), nil
}

func installService(exePath string) error {
	exePath, _ = filepath.Abs(exePath)

	home, _ := os.UserHomeDir()
	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return err
	}

	plistPath := filepath.Join(plistDir, "io.aurago.remote.plist")
	tmpl, err := template.New("plist").Parse(launchAgentPlist)
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".aurago-remote")
	_ = os.MkdirAll(logDir, 0700)

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist: %w", err)
	}
	defer f.Close()

	data := struct {
		ExePath string
		WorkDir string
		LogPath string
	}{
		ExePath: exePath,
		WorkDir: filepath.Dir(exePath),
		LogPath: filepath.Join(logDir, "remote.log"),
	}
	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w", err)
	}
	return nil
}

func uninstallService() error {
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "io.aurago.remote.plist")
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	_ = os.Remove(plistPath)
	return nil
}
