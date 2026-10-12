package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"aurago/internal/fileutil"
)

const launchAgentPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>io.aurago.remote</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.ExePath | xml}}</string>
		<string>--foreground</string>
		<string>--config</string>
		<string>{{.ConfigPath | xml}}</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>WorkingDirectory</key>
	<string>{{.WorkDir | xml}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath | xml}}</string>
	<key>StandardOutPath</key>
	<string>{{.LogPath | xml}}</string>
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
	tmpl, err := template.New("plist").Funcs(template.FuncMap{"xml": html.EscapeString}).Parse(launchAgentPlist)
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".aurago-remote")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return err
	}

	var buffer bytes.Buffer

	data := struct {
		ExePath    string
		ConfigPath string
		WorkDir    string
		LogPath    string
	}{
		ExePath:    exePath,
		ConfigPath: configPath(),
		WorkDir:    filepath.Dir(exePath),
		LogPath:    filepath.Join(logDir, "remote.log"),
	}
	if err := tmpl.Execute(&buffer, data); err != nil {
		return err
	}

	if err := fileutil.WriteFileContext(context.Background(), plistPath, buffer.Bytes(), 0600); err != nil {
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
	if err := exec.Command("launchctl", "unload", plistPath).Run(); err != nil {
		return fmt.Errorf("unload service: %w", err)
	}
	return os.Remove(plistPath)
}

func isRunningAsService() bool {
	return isRunningAsDarwinService(os.Getppid())
}

func isRunningAsDarwinService(ppid int) bool {
	return ppid == 1
}

func stopInstalledService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "Library", "LaunchAgents", "io.aurago.remote.plist")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := exec.Command("launchctl", "list", "io.aurago.remote").Run(); err != nil {
		return nil
	}
	return exec.Command("launchctl", "unload", path).Run()
}
