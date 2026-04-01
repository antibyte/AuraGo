// Package sandbox provides sandboxed shell command execution using OS-level
// isolation mechanisms. On Linux (Kernel 5.13+) it leverages Landlock LSM for
// filesystem access control and prlimit for resource limits. On other platforms
// it falls back to unsandboxed execution with a warning.
//
// Architecture: The sandbox uses a re-exec pattern — the AuraGo binary is
// invoked with "--sandbox-exec" causing it to enter helper mode, apply
// Landlock + rlimits to itself, and then exec the shell command. This ensures
// the main AuraGo process is never restricted.
package sandbox

import (
	"log/slog"
	"os/exec"
	"sync"
)

// ShellSandbox provides sandboxed shell command execution.
type ShellSandbox interface {
	// Available reports whether this sandbox backend is functional on the current system.
	Available() bool
	// Name returns the backend identifier (e.g. "landlock", "fallback").
	Name() string
	// PrepareCommand creates an exec.Cmd that will execute the given shell command
	// inside the sandbox. The caller is responsible for starting and waiting on the command.
	PrepareCommand(command, workDir string) *exec.Cmd
	// PrepareExecCommand creates an exec.Cmd that will execute the given binary and
	// arguments inside the sandbox without going through a shell.
	PrepareExecCommand(binary string, args []string, workDir string) *exec.Cmd
}

// ShellSandboxConfig holds configuration for the shell sandbox.
type ShellSandboxConfig struct {
	Enabled       bool
	MaxMemoryMB   int
	MaxCPUSeconds int
	MaxProcesses  int
	MaxFileSizeMB int
	AllowedPaths  []PathRule
	ExtraEnv      []string // Additional env vars passed into the sandboxed process (e.g. DOCKER_HOST)
}

// PathRule defines a filesystem path and its access mode for the sandbox.
type PathRule struct {
	Path     string
	ReadOnly bool
}

// Capabilities describes what sandbox features are available on the current system.
type Capabilities struct {
	LandlockABI   int    // Landlock ABI version (0 = not available)
	InDocker      bool   // running inside a Docker container
	KernelVersion string // Linux kernel version (empty on non-Linux)
}

// ── Global sandbox instance ────────────────────────────────────────────────

var (
	mu       sync.RWMutex
	instance ShellSandbox = &FallbackSandbox{}
)

// Init initializes the global shell sandbox based on config and detected capabilities.
// Call once at startup. Safe for concurrent use after initialization.
func Init(cfg ShellSandboxConfig, workspaceDir string, logger *slog.Logger) {
	mu.Lock()
	defer mu.Unlock()

	caps := Detect()
	logger.Info("Shell sandbox detection",
		"landlock_abi", caps.LandlockABI,
		"in_docker", caps.InDocker,
		"kernel", caps.KernelVersion,
	)

	if !cfg.Enabled {
		logger.Info("Shell sandbox disabled by config")
		instance = &FallbackSandbox{}
		return
	}

	if caps.InDocker {
		logger.Warn("Shell sandbox disabled — running inside Docker container")
		instance = &FallbackSandbox{}
		return
	}

	sb := newPlatformSandbox(cfg, caps, workspaceDir, logger)
	if sb == nil || !sb.Available() {
		logger.Warn("Shell sandbox not available on this platform, using fallback")
		instance = &FallbackSandbox{}
		return
	}

	instance = sb
	logger.Info("Shell sandbox initialized", "backend", sb.Name())
}

// Get returns the current global ShellSandbox instance. Never nil.
func Get() ShellSandbox {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

// GetCapabilities returns the detected sandbox capabilities for the current system.
func GetCapabilities() Capabilities {
	return Detect()
}

// ── FallbackSandbox ────────────────────────────────────────────────────────

// FallbackSandbox provides no isolation — it runs commands directly via /bin/sh.
type FallbackSandbox struct{}

func (f *FallbackSandbox) Available() bool { return false }
func (f *FallbackSandbox) Name() string    { return "fallback" }
func (f *FallbackSandbox) PrepareCommand(command, workDir string) *exec.Cmd {
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = workDir
	return cmd
}

func (f *FallbackSandbox) PrepareExecCommand(binary string, args []string, workDir string) *exec.Cmd {
	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	return cmd
}
