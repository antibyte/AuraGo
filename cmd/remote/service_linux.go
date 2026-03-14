package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const systemdUnit = `[Unit]
Description=AuraGo Remote Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExePath}} --foreground
Restart=always
RestartSec=10
User={{.User}}
WorkingDirectory={{.WorkDir}}

[Install]
WantedBy=multi-user.target
`

func installService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, _ = filepath.Abs(exePath)

	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}

	unitPath := "/etc/systemd/system/aurago-remote.service"
	tmpl, err := template.New("unit").Parse(systemdUnit)
	if err != nil {
		return err
	}

	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("failed to create unit file (run as root?): %w", err)
	}
	defer f.Close()

	data := struct {
		ExePath string
		User    string
		WorkDir string
	}{
		ExePath: exePath,
		User:    user,
		WorkDir: filepath.Dir(exePath),
	}
	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w", err)
	}
	if err := exec.Command("systemctl", "enable", "aurago-remote").Run(); err != nil {
		return fmt.Errorf("systemctl enable failed: %w", err)
	}
	if err := exec.Command("systemctl", "start", "aurago-remote").Run(); err != nil {
		return fmt.Errorf("systemctl start failed: %w", err)
	}
	return nil
}

func uninstallService() error {
	_ = exec.Command("systemctl", "stop", "aurago-remote").Run()
	_ = exec.Command("systemctl", "disable", "aurago-remote").Run()
	_ = os.Remove("/etc/systemd/system/aurago-remote.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
}
