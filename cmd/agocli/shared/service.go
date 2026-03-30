package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const serviceTemplate = `[Unit]
Description=AuraGo AI Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User={{.User}}
Group={{.Group}}
WorkingDirectory={{.WorkDir}}
ExecStart={{.ExecStart}}
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=0
{{- if .EnvFile}}
EnvironmentFile={{.EnvFile}}
{{- end}}
{{- if .HTTPS}}
AmbientCapabilities=CAP_NET_BIND_SERVICE
{{- end}}
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths={{.WorkDir}} /etc/aurago
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`

// ServiceConfig holds the configuration for generating a systemd service unit.
type ServiceConfig struct {
	User      string
	Group     string
	WorkDir   string
	ExecStart string
	EnvFile   string
	HTTPS     bool
}

// InstallService creates and enables a systemd service for AuraGo.
func InstallService(cfg ServiceConfig) error {
	tmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		return fmt.Errorf("parse service template: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, cfg); err != nil {
		return fmt.Errorf("render service template: %w", err)
	}

	unitContent := sb.String()
	unitPath := "/etc/systemd/system/aurago.service"

	// Write via sudo tee
	cmd := exec.Command("sudo", "tee", unitPath)
	cmd.Stdin = strings.NewReader(unitContent)
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	// Fix ownership on work directory
	if cfg.User != "root" {
		sudoExec("chown", "-R", cfg.User+":"+cfg.Group, cfg.WorkDir)
	}

	// Reload, enable, but don't start yet
	for _, args := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "aurago.service"},
	} {
		sudoExec(args...)
	}
	return nil
}

// PatchServiceEnvFile updates the EnvironmentFile line in an existing service.
func PatchServiceEnvFile(envFile string) error {
	unitPath := "/etc/systemd/system/aurago.service"
	data, err := os.ReadFile(unitPath)
	if err != nil {
		return err
	}

	content := string(data)
	// Replace existing EnvironmentFile or add it
	if strings.Contains(content, "EnvironmentFile=") {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "EnvironmentFile=") {
				lines[i] = "EnvironmentFile=" + envFile
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		content = strings.Replace(content, "[Service]\n", "[Service]\nEnvironmentFile="+envFile+"\n", 1)
	}

	cmd := exec.Command("sudo", "tee", unitPath)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return err
	}

	return sudoExec("systemctl", "daemon-reload")
}

// StartService starts the AuraGo service (systemd or nohup fallback).
func StartService(installDir string) error {
	if HasSystemd() {
		return sudoExec("systemctl", "start", "aurago")
	}
	return startNohup(installDir)
}

// StopService stops the AuraGo service.
func StopService() error {
	if HasSystemd() {
		return sudoExec("systemctl", "stop", "aurago")
	}
	// Fallback: try to kill by process name
	exec.Command("pkill", "-f", "bin/aurago_linux").Run()
	exec.Command("pkill", "-f", "bin/aurago").Run()
	return nil
}

// RestartService restarts the AuraGo service.
func RestartService(installDir string) error {
	if HasSystemd() {
		return sudoExec("systemctl", "restart", "aurago")
	}
	exec.Command("pkill", "-f", "bin/aurago_linux").Run()
	exec.Command("pkill", "-f", "bin/aurago").Run()
	return startNohup(installDir)
}

func startNohup(installDir string) error {
	binPath := filepath.Join(installDir, "bin", "aurago_linux")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		binPath = filepath.Join(installDir, "bin", "aurago")
	}

	// Load master key from .env if present
	envPath := filepath.Join(installDir, ".env")
	masterKey := ReadEnvKey(envPath, "AURAGO_MASTER_KEY")

	logPath := filepath.Join(installDir, "log", "aurago.log")
	os.MkdirAll(filepath.Dir(logPath), 0750)

	cmd := exec.Command("nohup", binPath)
	cmd.Dir = installDir
	if masterKey != "" {
		cmd.Env = append(os.Environ(), "AURAGO_MASTER_KEY="+masterKey)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	return cmd.Start()
}

// GenerateStartScript creates a start.sh script for non-systemd environments.
func GenerateStartScript(installDir string) error {
	content := fmt.Sprintf(`#!/bin/bash
# AuraGo start script
cd "%s"
if [ -f .env ]; then
    source .env
fi
if [ -f /etc/aurago/master.key ]; then
    source /etc/aurago/master.key
fi
nohup ./bin/aurago_linux > log/aurago.log 2>&1 &
echo "AuraGo started (PID: $!)"
`, installDir)
	path := filepath.Join(installDir, "start.sh")
	return os.WriteFile(path, []byte(content), 0755)
}
