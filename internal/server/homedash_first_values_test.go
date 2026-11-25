package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestMetricsCollectorSamplesAtStart(t *testing.T) {
	src, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`go func\(\) \{\s*//[^\n]*\n\s*s\.sampleSystemMetrics\(\)\s*\n\s*ticker := time\.NewTicker\(10 \* time\.Second\)`)
	if !re.Match(src) {
		t.Fatal("the collector goroutine must take its first sample before the 10 s ticker (decision D7)")
	}
	if strings.Count(string(src), "s.sampleSystemMetrics()") < 2 {
		t.Fatal("every tick must sample through sampleSystemMetrics")
	}
}

func TestSampleSystemMetricsStoresSnapshotRingAndBootTime(t *testing.T) {
	s := &Server{}
	if _, _, ok := s.sampleSystemMetrics(); !ok {
		t.Skip("host metrics are not readable in this environment")
	}
	if _, ok := s.loadSystemMetricsSnapshot(); !ok {
		t.Fatal("no snapshot after the first sample")
	}
	now := time.Now()
	if n := len(s.homeDashSystemHistory(now.Add(-time.Minute), now.Add(time.Minute))); n != 1 {
		t.Fatalf("ring entries = %d, want 1", n)
	}
}

func TestHomeDashSystemSnapshotCarriesServerRates(t *testing.T) {
	s := &Server{}
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	store := func(at time.Time, recv, sent uint64) {
		s.storeSystemMetricsSnapshot(tools.SystemMetrics{Available: map[string]bool{"cpu": true, "memory": true, "disk": true, "network": true},
			Network: tools.NetworkMetrics{BytesRecv: recv, BytesSent: sent}}, at)
		s.recordHomeDashMetrics()
	}
	store(t0, 1_000_000, 500_000)
	if snap, _ := s.homeDashSystemSnapshot(); snap.NetRxMbps != nil || snap.NetTxMbps != nil {
		t.Fatalf("rates after one sample = %v %v", snap.NetRxMbps, snap.NetTxMbps)
	}
	store(t0.Add(10*time.Second), 13_500_000, 3_000_000)
	snap, _ := s.homeDashSystemSnapshot()
	if snap.NetRxMbps == nil || *snap.NetRxMbps != 10 || snap.NetTxMbps == nil || *snap.NetTxMbps != 2 {
		t.Fatalf("rates after two samples = %v %v", snap.NetRxMbps, snap.NetTxMbps)
	}
}
