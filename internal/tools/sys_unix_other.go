//go:build !windows && !linux

package tools

// ApplySkillLimits is a no-op on non-Linux Unix systems (e.g. macOS, FreeBSD).
// prlimit(2) is Linux-specific; no equivalent for setting rlimits on another
// process exists on these platforms. The context-based timeout still applies.
func ApplySkillLimits(pid, memoryMB, cpuSeconds int) {
	// No-op: prlimit(2) is not available outside Linux.
}
