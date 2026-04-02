//go:build linux

package tools

import (
	"log/slog"

	"golang.org/x/sys/unix"
)

// ApplySkillLimits sets POSIX rlimits on a running child process via prlimit(2).
// Must be called immediately after cmd.Start(). Fails gracefully — logs warnings
// but does not return errors, since context-based timeout is the primary safeguard.
func ApplySkillLimits(pid, memoryMB, cpuSeconds int) {
	if memoryMB <= 0 {
		memoryMB = 1024
	}
	if cpuSeconds <= 0 {
		cpuSeconds = 120
	}

	memBytes := uint64(memoryMB) * 1024 * 1024

	limits := []struct {
		resource int
		rlimit   unix.Rlimit
		name     string
	}{
		{unix.RLIMIT_AS, unix.Rlimit{Cur: memBytes, Max: memBytes}, "RLIMIT_AS"},
		{unix.RLIMIT_CPU, unix.Rlimit{Cur: uint64(cpuSeconds), Max: uint64(cpuSeconds)}, "RLIMIT_CPU"},
		{unix.RLIMIT_NPROC, unix.Rlimit{Cur: 50, Max: 50}, "RLIMIT_NPROC"},
	}

	for _, l := range limits {
		if err := unix.Prlimit(pid, l.resource, &l.rlimit, nil); err != nil {
			slog.Debug("[SkillLimits] Failed to set rlimit", "limit", l.name, "pid", pid, "error", err)
		}
	}
}
