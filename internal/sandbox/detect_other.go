//go:build !linux

package sandbox

import "log/slog"

// Detect returns empty capabilities on non-Linux platforms.
func Detect() Capabilities {
	return Capabilities{}
}

// newPlatformSandbox returns nil on non-Linux platforms — no native sandbox available.
func newPlatformSandbox(_ ShellSandboxConfig, _ Capabilities, _ string, logger *slog.Logger) ShellSandbox {
	logger.Info("Shell sandbox not supported on this platform")
	return nil
}
