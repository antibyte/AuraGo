package server

import (
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash/sources/system"
	"aurago/internal/tools"
)

func TestHomeDashSystemSnapshotFromServerMetrics(t *testing.T) {
	s := &Server{}
	if _, ok := s.homeDashSystemSnapshot(); ok {
		t.Fatal("snapshot before the first collector sample")
	}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"cpu": true, "memory": true, "disk": true, "network": true},
		CPU:       tools.CPUMetrics{UsagePercent: 12.5},
		Memory:    tools.MemoryMetrics{UsedPercent: 40},
		Disk:      tools.DiskMetrics{UsedPercent: 91},
		Network:   tools.NetworkMetrics{BytesSent: 10, BytesRecv: 20},
	}, at)
	got, ok := s.homeDashSystemSnapshot()
	if !ok || !got.SampledAt.Equal(at) || got.CPUPercent != 12.5 || got.MemoryPercent != 40 || got.DiskPercent != 91 ||
		!got.CPUOK || !got.MemoryOK || !got.DiskOK || !got.NetworkOK || got.NetBytesRecv != 20 || got.NetBytesSent != 10 ||
		!got.BootTime.IsZero() {
		t.Fatalf("snapshot = %+v %v", got, ok)
	}
	s.storeHostBootTime(at.Add(-time.Hour))
	if got, _ := s.homeDashSystemSnapshot(); !got.BootTime.Equal(at.Add(-time.Hour)) {
		t.Fatalf("boot time = %v", got.BootTime)
	}
}

func TestHomeDashSourceSpecsListSystem(t *testing.T) {
	specs := (&Server{}).homeDashSourceSpecs()
	if len(specs) != 1 || specs[0].ID != system.SourceID || specs[0].ConfigKey != "system" || specs[0].Factory == nil || specs[0].Settings == nil {
		t.Fatalf("specs = %+v", specs)
	}
	cfg := &config.Config{}
	cfg.HomeDash.Sources.System = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 7}
	if set := specs[0].Settings(cfg); !set.Enabled || set.Control || set.PollSeconds != 7 {
		t.Fatalf("settings = %+v (control must stay off: the source is read-only)", set)
	}
}
