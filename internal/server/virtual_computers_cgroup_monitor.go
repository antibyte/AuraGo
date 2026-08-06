package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/planner"
)

const (
	virtualComputersCgroupWarningThreshold  = 10000
	virtualComputersCgroupCriticalThreshold = 50000
	virtualComputersCgroupCheckInterval     = 5 * time.Minute
)

// countBoringdCgroups returns the number of cgroup directories under
// boringd.service. A negative value indicates that the count could not be
// determined (e.g. cgroup v1 layout or missing path).
func countBoringdCgroups() int {
	base := "/sys/fs/cgroup/system.slice/boringd.service"
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return -1
	}
	count := 0
	// Walk only to a reasonable depth to avoid expensive recursion.
	err = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore permission errors in delegated subtrees
		}
		if info.IsDir() && path != base {
			count++
		}
		return nil
	})
	if err != nil {
		return -1
	}
	return count
}

// startVirtualComputersCgroupMonitor begins a background goroutine that
// periodically checks the number of cgroups under boringd.service and logs
// warnings or records operational issues when thresholds are crossed.
// The goroutine exits when ctx is cancelled.
func startVirtualComputersCgroupMonitor(ctx context.Context, s *Server, logger *slog.Logger) {
	if s == nil || logger == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(virtualComputersCgroupCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			count := countBoringdCgroups()
			if count < 0 {
				continue
			}
			if count < virtualComputersCgroupWarningThreshold {
				continue
			}
			logger.Warn("[VirtualComputers] boringd cgroup count is high", "count", count)
			if count < virtualComputersCgroupCriticalThreshold {
				continue
			}
			if s.PlannerDB == nil {
				continue
			}
			issue := planner.OperationalIssue{
				Source:      "virtual_computers",
				Title:       "boringd cgroup count approaching system limit",
				Detail:      fmt.Sprintf("boringd.service currently has %d child cgroups, which is close to the global cgroup limit. Auto-setup has been paused until the issue is resolved.", count),
				Severity:    "critical",
				Reference:   "boringd_cgroup_exhaustion",
				Fingerprint: fmt.Sprintf("boringd-cgroup-exhaustion-%d", count/1000*1000),
				OccurredAt:  time.Now(),
			}
			issueID, err := planner.RecordOperationalIssue(s.PlannerDB, issue)
			if err != nil {
				logger.Warn("[VirtualComputers] failed to record cgroup operational issue", "error", err)
				continue
			}
			if s.MissionManagerV2 != nil {
				s.MissionManagerV2.NotifyPlannerOperationalIssue(issueID, issue.Source, issue.Severity, issue.Title)
			}
		}
	}()
}
