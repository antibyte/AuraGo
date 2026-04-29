package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ServiceManager handles native service operations (systemctl, launchctl, sc.exe)
type ServiceManager struct{}

// NewServiceManager creates a new instance
func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

// ManageService performs the requested operation on the service
func (sm *ServiceManager) ManageService(operation, service string) (string, error) {
	if err := requireShellPermission(); err != nil {
		return "", err
	}
	if service == "" {
		return "", fmt.Errorf("service name cannot be empty")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = sm.buildLinuxCommand(operation, service)
	case "darwin":
		cmd = sm.buildMacCommand(operation, service)
	case "windows":
		cmd = sm.buildWindowsCommand(operation, service)
	default:
		return "", fmt.Errorf("unsupported OS for service manager: %s", runtime.GOOS)
	}

	if cmd == nil {
		return "", fmt.Errorf("unsupported operation '%s' for OS '%s'", operation, runtime.GOOS)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		if outStr != "" {
			return outStr, fmt.Errorf("command failed: %v, stderr: %s", err, errStr)
		}
		if errStr != "" {
			return errStr, fmt.Errorf("command failed: %v", err)
		}
		return "", fmt.Errorf("command failed: %v", err)
	}

	if outStr != "" {
		return outStr, nil
	}
	return "Operation completed successfully.", nil
}

func (sm *ServiceManager) buildLinuxCommand(operation, service string) *exec.Cmd {
	// Use systemctl
	switch operation {
	case "status":
		return exec.Command("systemctl", "status", service)
	case "start":
		return exec.Command("systemctl", "start", service) // Note: may need sudo in real usage, assuming running as root or agent handles auth
	case "stop":
		return exec.Command("systemctl", "stop", service)
	case "restart":
		return exec.Command("systemctl", "restart", service)
	case "enable":
		return exec.Command("systemctl", "enable", service)
	case "disable":
		return exec.Command("systemctl", "disable", service)
	default:
		return nil
	}
}

func (sm *ServiceManager) buildMacCommand(operation, service string) *exec.Cmd {
	// launchctl handles lists, print, load, unload, start, stop
	switch operation {
	case "status":
		return exec.Command("launchctl", "list", service)
	case "start":
		return exec.Command("launchctl", "start", service)
	case "stop":
		return exec.Command("launchctl", "stop", service)
	case "enable":
		return exec.Command("launchctl", "load", "-w", service)
	case "disable":
		return exec.Command("launchctl", "unload", "-w", service)
	case "restart":
		return nil // launchctl has no direct restart, could implement as stop then start
	default:
		return nil
	}
}

func (sm *ServiceManager) buildWindowsCommand(operation, service string) *exec.Cmd {
	// sc.exe works well
	switch operation {
	case "status":
		return exec.Command("sc.exe", "query", service)
	case "start":
		return exec.Command("sc.exe", "start", service)
	case "stop":
		return exec.Command("sc.exe", "stop", service)
	case "restart":
		return nil // sc.exe has no direct restart command
	case "enable":
		return exec.Command("sc.exe", "config", service, "start=", "auto")
	case "disable":
		return exec.Command("sc.exe", "config", service, "start=", "disabled")
	default:
		return nil
	}
}
