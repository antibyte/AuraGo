package server

import (
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestDashboardMetricsSnapshotStoreAndLoad(t *testing.T) {
	s := &Server{}
	if _, ok := s.loadSystemMetricsSnapshot(); ok {
		t.Fatal("loadSystemMetricsSnapshot before the first sample: ok = true, want false")
	}
	at := time.Date(2026, 10, 17, 9, 41, 0, 0, time.UTC)
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"cpu": true, "memory": true, "disk": false},
		CPU:       tools.CPUMetrics{UsagePercent: 12.5},
		Memory:    tools.MemoryMetrics{UsedPercent: 40},
		Disk:      tools.DiskMetrics{UsedPercent: 91},
	}, at)
	got, ok := s.loadSystemMetricsSnapshot()
	if !ok {
		t.Fatal("loadSystemMetricsSnapshot after a sample: ok = false, want true")
	}
	want := systemMetricsSnapshot{
		CPUPercent:      12.5,
		MemoryPercent:   40,
		DiskPercent:     91,
		CPUAvailable:    true,
		MemoryAvailable: true,
		DiskAvailable:   false,
		SampledAt:       at,
	}
	if got != want {
		t.Fatalf("snapshot = %+v, want %+v", got, want)
	}

	var missing *Server
	missing.storeSystemMetricsSnapshot(tools.SystemMetrics{}, at)
	if _, ok := missing.loadSystemMetricsSnapshot(); ok {
		t.Fatal("nil server: ok = true, want false")
	}
}

func TestDashboardMetricsCollectorStoresSnapshot(t *testing.T) {
	source, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatalf("read server_routes.go: %v", err)
	}
	if !strings.Contains(string(source), "s.storeSystemMetricsSnapshot(metricsResult.Data, time.Now())") {
		t.Fatal("the 10 s system-metrics collector must store its sample for /api/dashboard/summary")
	}
}
