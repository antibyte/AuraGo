package server

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash/sources/system"
	"aurago/internal/tools"
)

func TestMetricsSnapshotCarriesNetworkAndBootTime(t *testing.T) {
	s := &Server{}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"network": true},
		Network:   tools.NetworkMetrics{BytesSent: 10, BytesRecv: 20},
	}, at)
	got, ok := s.loadSystemMetricsSnapshot()
	if !ok || !got.NetworkAvailable || got.NetBytesSent != 10 || got.NetBytesRecv != 20 {
		t.Fatalf("snapshot = %+v %v", got, ok)
	}
	if _, ok := s.loadHostBootTime(); ok {
		t.Fatal("boot time known before the collector stored one")
	}
	s.storeHostBootTime(at.Add(-time.Hour))
	boot, ok := s.loadHostBootTime()
	if !ok || !boot.Equal(at.Add(-time.Hour)) || boot.Location() != time.UTC {
		t.Fatalf("boot time = %v %v", boot, ok)
	}
	var none *Server
	none.storeHostBootTime(at)
	if _, ok := none.loadHostBootTime(); ok {
		t.Fatal("nil server: ok = true")
	}
}

func TestMetricsCollectorStoresHostBootTime(t *testing.T) {
	source, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "s.storeHostBootTime(") {
		t.Fatal("the 10 s collector must store the host boot time for the HomeDash system source")
	}
	if !strings.Contains(string(source), "s.recordHomeDashMetrics()") {
		t.Fatal("the 10 s collector must append each sample to the HomeDash history ring")
	}
}

func TestMetricsRingCapacityOrderAndRange(t *testing.T) {
	var r homeDashMetricsRing
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	if got := r.between(t0, t0.Add(time.Hour)); got == nil || len(got) != 0 {
		t.Fatalf("empty ring = %#v", got)
	}
	total := homeDashMetricsRingCap + 5
	for i := 0; i < total; i++ {
		rate := float64(i)
		r.append(system.HistoryPoint{T: t0.Add(time.Duration(i) * 10 * time.Second), CPU: float64(i), NetRx: &rate, NetTx: &rate})
	}
	all := r.between(t0, t0.Add(48*time.Hour))
	if len(all) != homeDashMetricsRingCap {
		t.Fatalf("len = %d, want %d", len(all), homeDashMetricsRingCap)
	}
	if all[0].CPU != 5 || all[len(all)-1].CPU != float64(total-1) {
		t.Fatalf("first/last = %v / %v", all[0].CPU, all[len(all)-1].CPU)
	}
	for i := 1; i < len(all); i++ {
		if !all[i].T.After(all[i-1].T) {
			t.Fatalf("not oldest first at %d", i)
		}
	}
	// inclusive range: entries 100..102
	from, to := t0.Add(100*10*time.Second), t0.Add(102*10*time.Second)
	got := r.between(from, to)
	if len(got) != 3 || got[0].CPU != 100 || got[2].CPU != 102 {
		t.Fatalf("range = %+v", got)
	}
	// returned pointers must not alias ring memory
	*got[0].NetRx = -1
	again := r.between(from, from)
	if len(again) != 1 || *again[0].NetRx != 100 {
		t.Fatalf("ring entry mutated through a returned pointer: %+v", again)
	}
}

func TestMetricsRingRatesAcrossSamples(t *testing.T) {
	s := &Server{}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	store := func(d time.Duration, sent, recv uint64) {
		s.storeSystemMetricsSnapshot(tools.SystemMetrics{
			Available: map[string]bool{"network": true, "cpu": true},
			CPU:       tools.CPUMetrics{UsagePercent: 12},
			Network:   tools.NetworkMetrics{BytesSent: sent, BytesRecv: recv},
		}, at.Add(d))
		s.recordHomeDashMetrics()
	}
	store(0, 1_000_000, 1_000_000)
	store(10*time.Second, 2_000_000, 13_500_000) // 12.5 MB in 10 s = 10 Mbit/s rx, 0.8 Mbit/s tx
	store(20*time.Second, 1_000, 2_000)          // counters decreased
	store(30*time.Second, 126_000, 1_252_000)    // 1.25 MB rx, 125 kB tx in 10 s

	pts := s.homeDashSystemHistory(at, at.Add(time.Minute))
	if len(pts) != 4 {
		t.Fatalf("len = %d", len(pts))
	}
	if pts[0].NetRx != nil || pts[0].NetTx != nil {
		t.Fatalf("first entry must have nil rates: %+v", pts[0])
	}
	if pts[0].CPU != 12 || !pts[1].T.Equal(at.Add(10*time.Second)) {
		t.Fatalf("sample fields/time: %+v", pts[:2])
	}
	if pts[1].NetRx == nil || *pts[1].NetRx != 10 || *pts[1].NetTx != 0.8 {
		t.Fatalf("second entry rates = %v %v", pts[1].NetRx, pts[1].NetTx)
	}
	if pts[2].NetRx != nil || pts[2].NetTx != nil {
		t.Fatalf("counter decrease must give nil rates: %+v", pts[2])
	}
	if pts[3].NetRx == nil || *pts[3].NetRx != 1 || *pts[3].NetTx != 0.1 {
		t.Fatalf("rate after a reset is measured from the reset sample: %v %v", pts[3].NetRx, pts[3].NetTx)
	}

	var none *Server
	if got := none.homeDashSystemHistory(at, at.Add(time.Hour)); got == nil || len(got) != 0 {
		t.Fatalf("nil server = %#v", got)
	}
}

func TestMetricsRingStoresNaNForUnreadableMetrics(t *testing.T) {
	r := &homeDashMetricsRing{}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	r.record(systemMetricsSnapshot{SampledAt: at, CPUPercent: 0, MemoryPercent: 41, DiskPercent: 63, MemoryAvailable: true, DiskAvailable: true})
	pts := r.between(at, at)
	if len(pts) != 1 {
		t.Fatalf("len = %d", len(pts))
	}
	if !math.IsNaN(pts[0].CPU) || pts[0].Memory != 41 || pts[0].Disk != 63 {
		t.Fatalf("point = %+v, want NaN cpu with memory 41 and disk 63", pts[0])
	}
}
