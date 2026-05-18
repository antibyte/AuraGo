package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
)

func getInstallPath() (string, error) {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	dir := filepath.Join(pf, "AuraGo")
	return filepath.Join(dir, "aurago-remote.exe"), nil
}

func installService(exePath string) error {
	exePath, _ = filepath.Abs(exePath)

	// Use sc.exe to create a Windows service
	err := exec.Command("sc", "create", "AuraGoRemote",
		"binpath=", windowsServiceBinPath(exePath),
		"start=", "auto",
		"DisplayName=", "AuraGo Remote Agent",
	).Run()
	if err != nil {
		return fmt.Errorf("sc create failed (run as Administrator?): %w", err)
	}

	if err := exec.Command("sc", "start", "AuraGoRemote").Run(); err != nil {
		return fmt.Errorf("sc start failed: %w", err)
	}
	return nil
}

func uninstallService() error {
	_ = exec.Command("sc", "stop", "AuraGoRemote").Run()
	_ = exec.Command("sc", "delete", "AuraGoRemote").Run()
	return nil
}

func windowsServiceBinPath(exePath string) string {
	return fmt.Sprintf(`""%s" --foreground"`, exePath)
}

func isRunningAsService() bool {
	return isRunningAsWindowsService(svc.IsWindowsService)
}

func isRunningAsWindowsService(probe func() (bool, error)) bool {
	running, err := probe()
	return err == nil && running
}
