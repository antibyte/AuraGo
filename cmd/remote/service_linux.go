package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"aurago/internal/fileutil"
)

const systemdUnit = `[Unit]
Description=AuraGo Remote Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExePath}} --foreground --config {{.ConfigPath}}
Restart=always
RestartSec=10
User={{.User}}
WorkingDirectory={{.WorkDir}}
UMask=0077

[Install]
WantedBy=multi-user.target
`

func getInstallPath() (string, error) {
	return "/usr/local/bin/aurago-remote", nil
}

func installService(exePath string) error {
	exePath, _ = filepath.Abs(exePath)

	account, err := user.Current()
	if err != nil {
		return fmt.Errorf("current service account: %w", err)
	}

	unitPath := "/etc/systemd/system/aurago-remote.service"
	tmpl, err := template.New("unit").Parse(systemdUnit)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer

	data := struct {
		ExePath    string
		ConfigPath string
		User       string
		WorkDir    string
	}{
		ExePath:    strconv.Quote(strings.ReplaceAll(exePath, "%", "%%")),
		ConfigPath: strconv.Quote(strings.ReplaceAll(configPath(), "%", "%%")),
		User:       account.Username,
		WorkDir:    strconv.Quote(strings.ReplaceAll(filepath.Dir(exePath), "%", "%%")),
	}
	if err := tmpl.Execute(&buffer, data); err != nil {
		return err
	}

	if err := fileutil.WriteFileContext(context.Background(), unitPath, buffer.Bytes(), 0644); err != nil {
		return err
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w", err)
	}
	if err := exec.Command("systemctl", "enable", "aurago-remote").Run(); err != nil {
		return fmt.Errorf("systemctl enable failed: %w", err)
	}
	if err := exec.Command("systemctl", "restart", "aurago-remote").Run(); err != nil {
		return fmt.Errorf("systemctl start failed: %w", err)
	}
	return nil
}

func uninstallService() error {
	if err := exec.Command("systemctl", "stop", "aurago-remote").Run(); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	if err := exec.Command("systemctl", "disable", "aurago-remote").Run(); err != nil {
		return fmt.Errorf("disable service: %w", err)
	}
	if err := os.Remove("/etc/systemd/system/aurago-remote.service"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return exec.Command("systemctl", "daemon-reload").Run()
}

func isRunningAsService() bool {
	return isRunningAsLinuxService(os.Getenv("INVOCATION_ID"), os.Getenv("NOTIFY_SOCKET"), parentProcessName, os.Getppid())
}

func isRunningAsLinuxService(invocationID, notifySocket string, parentName func(int) string, ppid int) bool {
	if invocationID != "" || notifySocket != "" {
		return true
	}
	return parentName(ppid) == "systemd"
}

func parentProcessName(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func stopInstalledService() error {
	if err := exec.Command("systemctl", "is-active", "--quiet", "aurago-remote").Run(); err != nil {
		return nil
	}
	return exec.Command("systemctl", "stop", "aurago-remote").Run()
}
