package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

// ServiceManager handles native service operations (systemctl, launchctl, sc.exe)
type ServiceManager struct{}

// NewServiceManager creates a new instance
func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

// ManageService performs the requested operation on the service
func (sm *ServiceManager) ManageService(operation, service string) (string, error) {
	return sm.ManageServiceContext(context.Background(), operation, service)
}

func (sm *ServiceManager) ManageServiceContext(ctx context.Context, operation, service string) (string, error) {
	if err := requireShellPermissionContext(ctx); err != nil {
		return "", err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]{0,255}$`).MatchString(service) {
		return "", fmt.Errorf("invalid service name")
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

	if runtime.GOOS == "linux" {
		cmd = sandbox.Get().PrepareExecCommand(cmd.Path, cmd.Args[1:], "")
	}
	cmd.Env = sandbox.FilterEnv(os.Environ())
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	runner := NewForegroundRunner(cmd, ForegroundOptions{Timeout: 30 * time.Second, ScrubOutput: true})
	stdout, stderr, err := runner.Run(ctx)
	outStr := strings.TrimSpace(stdout)
	errStr := strings.TrimSpace(stderr)

	if err != nil {
		if outStr != "" {
			return outStr, fmt.Errorf("service command failed: %w, stderr: %s", err, errStr)
		}
		if errStr != "" {
			return errStr, fmt.Errorf("service command failed: %w", err)
		}
		return "", fmt.Errorf("service command failed: %w", err)
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
