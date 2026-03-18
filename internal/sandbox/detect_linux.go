//go:build linux

package sandbox

import (
	"log/slog"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Detect probes the current Linux system for sandbox capabilities.
func Detect() Capabilities {
	caps := Capabilities{}

	// Detect kernel version
	var uname unix.Utsname
	if err := unix.Uname(&uname); err == nil {
		caps.KernelVersion = strings.TrimRight(string(uname.Release[:]), "\x00")
	}

	// Detect Landlock ABI version by creating a minimal ruleset
	caps.LandlockABI = detectLandlockABI()

	// Detect Docker environment
	caps.InDocker = detectDocker()

	return caps
}

// detectLandlockABI probes the Landlock ABI version using LANDLOCK_CREATE_RULESET_VERSION.
// Returns 0 if Landlock is not available.
func detectLandlockABI() int {
	// Passing nil attr + size 0 with LANDLOCK_CREATE_RULESET_VERSION flag
	// returns the highest supported ABI version as the fd value (or error).
	fd, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0, // NULL attr
		0, // size 0
		unix.LANDLOCK_CREATE_RULESET_VERSION,
	)
	if errno != 0 {
		return 0
	}
	// fd is the ABI version (not a real fd)
	return int(fd)
}

// detectDocker checks whether the process is running inside a Docker container.
func detectDocker() bool {
	// Method 1: /.dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Method 2: /proc/1/cgroup contains "docker" or "containerd"
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		if strings.Contains(s, "docker") || strings.Contains(s, "containerd") {
			return true
		}
	}
	return false
}

// newPlatformSandbox returns a Linux-specific sandbox implementation.
func newPlatformSandbox(cfg ShellSandboxConfig, caps Capabilities, workspaceDir string, logger *slog.Logger) ShellSandbox {
	if caps.LandlockABI < 1 {
		logger.Warn("Landlock not available (kernel too old or disabled)", "kernel", caps.KernelVersion)
		return nil
	}
	return NewLandlockSandbox(cfg, caps, workspaceDir, logger)
}
