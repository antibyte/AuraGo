package system

import (
	"context"
	"errors"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func lastUpdate(t *testing.T, sink *sourcetest.RecordingSink, native string) homedash.State {
	t.Helper()
	for i := len(sink.Updates) - 1; i >= 0; i-- {
		if sink.Updates[i].Native == native {
			return sink.Updates[i].State
		}
	}
	t.Fatalf("no update for %s", native)
	return homedash.State{}
}

func lastStatus(t *testing.T, sink *sourcetest.RecordingSink) homedash.SourceStatus {
	t.Helper()
	if len(sink.Statuses) == 0 {
		t.Fatal("no status")
	}
	return sink.Statuses[len(sink.Statuses)-1]
}

func TestSystemAdapterMapsSnapshot(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	now, snap, have := t0, Snapshot{}, false
	src := New(func() (Snapshot, bool) { return snap, have }, nil, 5, func() time.Time { return now })
	sink := &sourcetest.RecordingSink{}

	// Before the collector's first sample: six read-only sensors, all unavailable.
	list, err := src.Catalog(context.Background())
	if err != nil || len(list) != 6 {
		t.Fatalf("catalog = %d entities, %v", len(list), err)
	}
	want := []homedash.EntityID{"system:cpu", "system:memory", "system:disk", "system:net_rx", "system:net_tx", "system:uptime"}
	for i, e := range list {
		if e.ID != want[i] || e.Kind != homedash.KindSensor || !e.ReadOnly || e.State.Available || len(e.Capabilities) != 2 {
			t.Fatalf("entity %d = %+v", i, e)
		}
	}
	src.pollOnce(sink)
	if st := lastStatus(t, sink); st.State != homedash.StatusUnavailable || st.Reason != "no_metrics" {
		t.Fatalf("status before a sample = %+v", st)
	}

	// First sample: percentages and uptime; no rate after one sample.
	snap, have = Snapshot{SampledAt: t0, CPUPercent: 12.34, MemoryPercent: 40, DiskPercent: 91.06,
		CPUOK: true, MemoryOK: true, DiskOK: true, NetworkOK: true,
		NetBytesRecv: 1_000_000, NetBytesSent: 500_000, BootTime: t0.Add(-2 * time.Hour)}, true
	now = t0.Add(time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeCPU); st.Value != 12.3 || !st.Available || st.Stale || !st.UpdatedAt.Equal(t0) {
		t.Fatalf("cpu = %+v", st)
	}
	if st := lastUpdate(t, sink, NativeDisk); st.Value != 91.1 {
		t.Fatalf("disk = %+v", st)
	}
	if st := lastUpdate(t, sink, NativeNetRX); st.Available {
		t.Fatalf("net_rx after one sample = %+v, want unavailable", st)
	}
	if st := lastUpdate(t, sink, NativeUptime); st.Value != 7201.0 || !st.Available {
		t.Fatalf("uptime = %+v", st)
	}
	if st := lastStatus(t, sink); st.State != homedash.StatusPolling {
		t.Fatalf("status = %+v", st)
	}

	// Second sample 10 s later: rates in Mbit/s.
	snap.SampledAt, snap.NetBytesRecv, snap.NetBytesSent = t0.Add(10*time.Second), 13_500_000, 3_000_000
	now = t0.Add(11 * time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeNetRX); st.Value != 10.0 || !st.Available {
		t.Fatalf("net_rx = %+v, want 10 Mbit/s", st)
	}
	if st := lastUpdate(t, sink, NativeNetTX); st.Value != 2.0 {
		t.Fatalf("net_tx = %+v, want 2 Mbit/s", st)
	}

	// Same sample again: only the uptime changes.
	n := len(sink.Updates)
	now = t0.Add(15 * time.Second)
	src.pollOnce(sink)
	if len(sink.Updates) != n+1 || sink.Updates[n].Native != NativeUptime {
		t.Fatalf("unchanged sample re-sent: %+v", sink.Updates[n:])
	}

	// 31 s after the last sample: stale (threshold max(3*5 s, 30 s)), status degraded.
	now = t0.Add(41 * time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeCPU); !st.Stale || !st.Available {
		t.Fatalf("cpu after 31 s = %+v, want stale", st)
	}
	if st := lastStatus(t, sink); st.State != homedash.StatusDegraded || st.Reason != "stale_metrics" {
		t.Fatalf("status = %+v", st)
	}

	// Counter reset: the rate keeps its last value and the baseline moves.
	snap.SampledAt, snap.NetBytesRecv = t0.Add(50*time.Second), 100
	now = t0.Add(51 * time.Second)
	src.pollOnce(sink)
	if st := lastUpdate(t, sink, NativeNetRX); st.Value != 10.0 || st.Stale {
		t.Fatalf("net_rx after a counter reset = %+v", st)
	}
}

func TestSystemFactoryAndCommand(t *testing.T) {
	snap := func() (Snapshot, bool) { return Snapshot{}, false }
	f := Factory(snap, nil)
	if _, ok := f(homedash.RuntimeConfig{}); ok {
		t.Fatal("no settings entry must disable the source")
	}
	off := homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: {Enabled: false}}}
	if _, ok := f(off); ok {
		t.Fatal("a switched-off source must stay disabled")
	}
	on := homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: {Enabled: true, PollSeconds: 2}}}
	src, ok := f(on)
	if !ok || src.ID() != SourceID || src.(*Source).poll != 2*time.Second {
		t.Fatalf("factory = %v %v", src, ok)
	}
	if _, ok := Factory(nil, nil)(on); ok {
		t.Fatal("no snapshot function must disable the source")
	}
	_, err := src.Command(context.Background(), homedash.Command{})
	if !errors.Is(err, &homedash.Error{Code: homedash.CodeUnsupportedCommand}) {
		t.Fatalf("command = %v, want unsupported_command", err)
	}
}
func fp(v float64) *float64 { return &v }

func TestSystemHistoryBucketsRangeFilterAndAveraging(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	from, to := t0, t0.Add(60*time.Second)
	entries := []HistoryPoint{
		{T: t0.Add(-time.Second), CPU: 99},
		{T: t0.Add(10 * time.Second), CPU: 10},
		{T: t0.Add(20 * time.Second), CPU: 20},
		{T: t0.Add(50 * time.Second), CPU: 40},
		{T: to, CPU: 60},
		{T: to.Add(time.Second), CPU: 99},
	}
	src := New(func() (Snapshot, bool) { return Snapshot{}, false },
		func(f, e time.Time) []HistoryPoint { return entries }, 5, nil)
	got, err := src.History(context.Background(), NativeCPU, from, to, 2)
	if err != nil || len(got) != 2 {
		t.Fatalf("history = %+v, %v", got, err)
	}
	// bucket 0 = [0,30): 10 and 20 averaged, T = last entry; bucket 1 = [30,60]: 40 and 60 (entry at to).
	if !got[0].T.Equal(t0.Add(20*time.Second)) || got[0].V != 15.0 {
		t.Fatalf("bucket 0 = %+v", got[0])
	}
	if !got[1].T.Equal(to) || got[1].V != 50.0 {
		t.Fatalf("bucket 1 = %+v", got[1])
	}
}

func TestSystemHistoryBucketsPointsCap(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	var entries []HistoryPoint
	for i := range 30 {
		entries = append(entries, HistoryPoint{T: t0.Add(time.Duration(i) * 10 * time.Second), Memory: float64(i)})
	}
	src := New(nil, func(f, e time.Time) []HistoryPoint { return entries }, 5, nil)
	got, err := src.History(context.Background(), NativeMemory, t0, t0.Add(300*time.Second), 3)
	if err != nil || len(got) != 3 {
		t.Fatalf("history = %d samples, %v", len(got), err)
	}
	for i := 1; i < len(got); i++ {
		if !got[i].T.After(got[i-1].T) {
			t.Fatalf("not oldest first: %+v", got)
		}
	}
	if got[0].V != 4.5 || got[1].V != 14.5 || got[2].V != 24.5 {
		t.Fatalf("averages = %+v", got)
	}
}

func TestSystemHistoryBucketsEmptyResults(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	to := t0.Add(time.Minute)
	one := []HistoryPoint{{T: t0.Add(time.Second), CPU: 1}}
	hist := func(f, e time.Time) []HistoryPoint { return one }
	cases := []struct {
		name    string
		history HistoryFunc
		native  string
		from    time.Time
		to      time.Time
		points  int
	}{
		{"nil history", nil, NativeCPU, t0, to, 5},
		{"empty input", func(f, e time.Time) []HistoryPoint { return nil }, NativeCPU, t0, to, 5},
		{"uptime", hist, NativeUptime, t0, to, 5},
		{"unknown", hist, "nope", t0, to, 5},
		{"no points", hist, NativeCPU, t0, to, 0},
		{"to not after from", hist, NativeCPU, to, t0, 5},
		{"nil rates skipped", hist, NativeNetRX, t0, to, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := New(nil, c.history, 5, nil).History(context.Background(), c.native, c.from, c.to, c.points)
			if err != nil || got == nil || len(got) != 0 {
				t.Fatalf("history = %#v, %v, want empty non-nil", got, err)
			}
		})
	}
}

func TestSystemHistoryBucketsRatesSkipNilAndRound(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	entries := []HistoryPoint{
		{T: t0.Add(5 * time.Second)},
		{T: t0.Add(15 * time.Second), NetRx: fp(1.0001), NetTx: fp(2)},
		{T: t0.Add(25 * time.Second), NetRx: fp(2.0004), NetTx: fp(4)},
	}
	src := New(nil, func(f, e time.Time) []HistoryPoint { return entries }, 5, nil)
	got, _ := src.History(context.Background(), NativeNetRX, t0, t0.Add(30*time.Second), 1)
	if len(got) != 1 || got[0].V != 1.5 || !got[0].T.Equal(t0.Add(25*time.Second)) {
		t.Fatalf("net_rx = %+v", got)
	}
	got, _ = src.History(context.Background(), NativeNetTX, t0, t0.Add(30*time.Second), 3)
	if len(got) != 2 || got[0].V != 2 || got[1].V != 4 {
		t.Fatalf("net_tx = %+v", got)
	}
}
