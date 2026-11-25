package server

import (
	"time"

	"aurago/internal/tools"
)

// systemMetricsSnapshot is the latest sample of the 10 s system-metrics
// collector in (*Server).run (server_routes.go). GET /api/dashboard/summary
// reads it and never samples itself: tools.GetSystemMetrics blocks for at
// least one second.
type systemMetricsSnapshot struct {
	CPUPercent      float64
	MemoryPercent   float64
	DiskPercent     float64
	CPUAvailable    bool
	MemoryAvailable bool
	DiskAvailable   bool
	SampledAt       time.Time

	// Raw cumulative network counters (HomeDash system source computes rates).
	NetBytesSent     uint64
	NetBytesRecv     uint64
	NetworkAvailable bool
}

// storeSystemMetricsSnapshot publishes one parsed collector sample.
func (s *Server) storeSystemMetricsSnapshot(m tools.SystemMetrics, at time.Time) {
	if s == nil {
		return
	}
	s.latestSystemMetrics.Store(&systemMetricsSnapshot{
		CPUPercent:      m.CPU.UsagePercent,
		MemoryPercent:   m.Memory.UsedPercent,
		DiskPercent:     m.Disk.UsedPercent,
		CPUAvailable:    m.Available["cpu"],
		MemoryAvailable: m.Available["memory"],
		DiskAvailable:   m.Available["disk"],
		SampledAt:       at,

		NetBytesSent:     m.Network.BytesSent,
		NetBytesRecv:     m.Network.BytesRecv,
		NetworkAvailable: m.Available["network"],
	})
}

// loadSystemMetricsSnapshot returns the latest sample. ok is false before the
// collector's first tick.
func (s *Server) loadSystemMetricsSnapshot() (systemMetricsSnapshot, bool) {
	if s == nil {
		return systemMetricsSnapshot{}, false
	}
	latest := s.latestSystemMetrics.Load()
	if latest == nil {
		return systemMetricsSnapshot{}, false
	}
	return *latest, true
}

// storeHostBootTime publishes the host boot time the collector derived from the host uptime.
func (s *Server) storeHostBootTime(boot time.Time) {
	if s == nil || boot.IsZero() {
		return
	}
	s.hostBootUnix.Store(boot.Unix())
}

// loadHostBootTime returns the host boot time (UTC, whole seconds); ok is false before the first sample.
func (s *Server) loadHostBootTime() (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}
	sec := s.hostBootUnix.Load()
	if sec <= 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, 0).UTC(), true
}
