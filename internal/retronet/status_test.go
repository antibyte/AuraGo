package retronet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// probeClock is a manually advanced clock for StatusProber.Now.
type probeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newProbeClock() *probeClock {
	return &probeClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
}

func (c *probeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *probeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// probeResolver resolves every host to one documentation address.
type probeResolver struct{}

func (probeResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.ParseIP("192.0.2.10")}}, nil
}

// probeDialer counts dials and their peak concurrency. A blocking dialer holds every dial
// until release is called; offline makes dials fail.
type probeDialer struct {
	block   chan struct{}
	offline atomic.Bool
	calls   atomic.Int64
	current atomic.Int64
	peak    atomic.Int64
	closed  atomic.Int64 // connections handed out and closed again
}

// probeConn counts Close calls of the connections a probeDialer hands out.
type probeConn struct {
	net.Conn
	d *probeDialer
}

func (c probeConn) Close() error {
	c.d.closed.Add(1)
	return c.Conn.Close()
}

func newProbeDialer(blocking bool) *probeDialer {
	d := &probeDialer{}
	if blocking {
		d.block = make(chan struct{})
	}
	return d
}

func (d *probeDialer) release() { close(d.block) }

func (d *probeDialer) dialer() Dialer {
	return Dialer{Resolver: probeResolver{}, Dial: d.dial, AllowRestricted: true}
}

func (d *probeDialer) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	d.calls.Add(1)
	cur := d.current.Add(1)
	defer d.current.Add(-1)
	for {
		peak := d.peak.Load()
		if cur <= peak || d.peak.CompareAndSwap(peak, cur) {
			break
		}
	}
	if d.block != nil {
		select {
		case <-d.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if d.offline.Load() {
		return nil, errors.New("connection refused")
	}
	client, server := net.Pipe()
	_ = server.Close()
	return probeConn{Conn: client, d: d}, nil
}

func probeEntries(n int) []Entry {
	entries := make([]Entry, 0, n)
	for i := range n {
		id := fmt.Sprintf("probe-%02d", i)
		entries = append(entries, Entry{ID: id, Name: id, Category: CategoryBBS, Protocol: ProtocolTelnet, Host: id + ".example", Port: 23, Kind: KindBBS, Charset: CharsetCP437})
	}
	return entries
}

func TestStatusProberBoundsConcurrency(t *testing.T) {
	d := newProbeDialer(true)
	p := &StatusProber{Dialer: d.dialer(), Concurrency: 3}
	entries := probeEntries(10)
	result := make(chan map[string]Status, 1)
	go func() { result <- p.Refresh(context.Background(), entries, 5*time.Second) }()
	sessWaitFor(t, func() bool { return d.current.Load() == 3 }, func() string {
		return fmt.Sprintf("3 parallel probes, have %d", d.current.Load())
	})
	d.release()
	statuses := <-result
	if d.peak.Load() != 3 || d.calls.Load() != 10 {
		t.Fatalf("peak concurrency %d, calls %d; want 3 and 10", d.peak.Load(), d.calls.Load())
	}
	for _, e := range entries {
		if statuses[e.ID].State != "online" {
			t.Fatalf("%s = %+v, want online", e.ID, statuses[e.ID])
		}
	}
}

func TestStatusProberSnapshotStaleness(t *testing.T) {
	clock := newProbeClock()
	d := newProbeDialer(false)
	p := &StatusProber{Dialer: d.dialer(), Now: clock.Now}
	entries := probeEntries(3)

	statuses, stale := p.Snapshot(entries)
	if !stale {
		t.Fatal("empty cache reported fresh")
	}
	for _, e := range entries {
		if statuses[e.ID].State != "unknown" || statuses[e.ID].CheckedAt != nil {
			t.Fatalf("%s before any probe = %+v, want unknown", e.ID, statuses[e.ID])
		}
	}
	p.Refresh(context.Background(), entries, 5*time.Second) // waits for the background probe
	if d.calls.Load() != 3 {
		t.Fatalf("calls = %d, want one probe run (3)", d.calls.Load())
	}
	statuses, stale = p.Snapshot(entries)
	if stale {
		t.Fatal("fresh cache reported stale")
	}
	if st := statuses["probe-00"]; st.State != "online" || st.CheckedAt == nil || !st.CheckedAt.Equal(clock.Now()) || st.LastOnlineAt == nil || !st.LastOnlineAt.Equal(clock.Now()) {
		t.Fatalf("probe-00 = %+v", st)
	}

	clock.Advance(9 * time.Minute)
	if _, stale = p.Snapshot(entries); stale || d.calls.Load() != 3 {
		t.Fatalf("9 min old cache: stale %v, calls %d", stale, d.calls.Load())
	}
	clock.Advance(2 * time.Minute)
	if _, stale = p.Snapshot(entries); !stale {
		t.Fatal("11 min old cache reported fresh")
	}
	p.Refresh(context.Background(), entries, 5*time.Second)
	if d.calls.Load() != 6 {
		t.Fatalf("calls = %d, want a second probe run (6)", d.calls.Load())
	}
}

func TestStatusProberRunsOneProbeAtATime(t *testing.T) {
	d := newProbeDialer(true)
	p := &StatusProber{Dialer: d.dialer()}
	entries := probeEntries(2)
	if _, stale := p.Snapshot(entries); !stale {
		t.Fatal("empty cache reported fresh")
	}
	p.mu.Lock()
	first := p.inflight
	p.mu.Unlock()
	for range 3 {
		if _, stale := p.Snapshot(entries); !stale {
			t.Fatal("cache reported fresh while the first probe runs")
		}
	}
	p.mu.Lock()
	same := p.inflight == first
	p.mu.Unlock()
	if !same {
		t.Fatal("a second probe started while one was in flight")
	}
	d.release()
	p.Refresh(context.Background(), entries, 5*time.Second)
	if d.calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", d.calls.Load())
	}
}

func TestStatusProberForcedRefreshFloor(t *testing.T) {
	clock := newProbeClock()
	d := newProbeDialer(false)
	p := &StatusProber{Dialer: d.dialer(), Now: clock.Now}
	entries := probeEntries(2)
	p.Refresh(context.Background(), entries, 5*time.Second)
	if d.calls.Load() != 2 {
		t.Fatalf("first refresh: calls = %d, want 2", d.calls.Load())
	}
	clock.Advance(30 * time.Second)
	if statuses := p.Refresh(context.Background(), entries, 5*time.Second); d.calls.Load() != 2 || statuses["probe-01"].State != "online" {
		t.Fatalf("refresh within 60s probed again (calls %d) or lost the cache: %+v", d.calls.Load(), statuses)
	}
	clock.Advance(31 * time.Second)
	p.Refresh(context.Background(), entries, 5*time.Second)
	if d.calls.Load() != 4 {
		t.Fatalf("refresh after 61s: calls = %d, want 4", d.calls.Load())
	}
}

func TestStatusProberOfflineKeepsLastOnline(t *testing.T) {
	clock := newProbeClock()
	d := newProbeDialer(false)
	p := &StatusProber{Dialer: d.dialer(), Now: clock.Now}
	entries := probeEntries(1)
	onlineAt := clock.Now()
	p.Refresh(context.Background(), entries, 5*time.Second)
	d.offline.Store(true)
	clock.Advance(61 * time.Second)
	st := p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
	if st.State != "offline" || st.CheckedAt == nil || !st.CheckedAt.Equal(clock.Now()) || st.LastOnlineAt == nil || !st.LastOnlineAt.Equal(onlineAt) {
		t.Fatalf("status = %+v, want offline checked now, last online %v", st, onlineAt)
	}
}

func TestStatusProberUnknownForMissingEntries(t *testing.T) {
	d := newProbeDialer(false)
	p := &StatusProber{Dialer: d.dialer()}
	known := probeEntries(2)
	p.Refresh(context.Background(), known, 5*time.Second)
	added := append(probeEntries(2), Entry{ID: "own-newentry1", Protocol: ProtocolTelnet, Host: "new.example", Port: 23, Kind: KindWorld, Charset: CharsetUTF8})
	statuses, stale := p.Snapshot(added)
	if statuses["own-newentry1"].State != "unknown" || statuses["probe-00"].State != "online" {
		t.Fatalf("statuses = %+v", statuses)
	}
	if !stale {
		t.Fatal("an entry that was never probed must make the snapshot stale")
	}
	p.Refresh(context.Background(), added, 5*time.Second) // waits for the probe Snapshot started
	if st := p.Refresh(context.Background(), added, 0)["own-newentry1"]; st.State != "online" {
		t.Fatalf("new entry after the background probe = %+v", st)
	}
}

func TestStatusProberRefreshStopsWaiting(t *testing.T) {
	d := newProbeDialer(true)
	p := &StatusProber{Dialer: d.dialer()}
	entries := probeEntries(2)
	started := time.Now()
	statuses := p.Refresh(context.Background(), entries, 50*time.Millisecond)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("Refresh waited %v despite a 50ms budget", elapsed)
	}
	if statuses["probe-00"].State != "unknown" {
		t.Fatalf("status while the probe runs = %+v, want unknown", statuses["probe-00"])
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Refresh(ctx, entries, 5*time.Second) // a cancelled context returns at once
	d.release()
	p.Refresh(context.Background(), entries, 5*time.Second)
	if d.calls.Load() != 2 {
		t.Fatalf("calls = %d, want one probe run", d.calls.Load())
	}
}

// A probe that never gets an answer gives up after StatusProber.Timeout (not after the dialer's
// own 10 s default), reports offline and leaves no dial running.
func TestStatusProberTimesOutProbesThatNeverAnswer(t *testing.T) {
	d := newProbeDialer(true) // never released: every dial waits for its context
	p := &StatusProber{Dialer: d.dialer(), Timeout: 50 * time.Millisecond}
	entries := probeEntries(3)
	started := time.Now()
	statuses := p.Refresh(context.Background(), entries, 5*time.Second)
	if elapsed := time.Since(started); elapsed < 40*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("probe run took %v, want about the 50ms probe timeout", elapsed)
	}
	for _, e := range entries {
		if st := statuses[e.ID]; st.State != "offline" || st.CheckedAt == nil || st.LastOnlineAt != nil {
			t.Fatalf("%s = %+v, want offline, checked, never online", e.ID, st)
		}
	}
	if d.calls.Load() != 3 || d.current.Load() != 0 {
		t.Fatalf("dials started %d, still running %d; want 3 and 0", d.calls.Load(), d.current.Load())
	}
}

// Probes that connect close the connection at once, online or not afterwards.
func TestStatusProberClosesProbeConnections(t *testing.T) {
	d := newProbeDialer(false)
	p := &StatusProber{Dialer: d.dialer()}
	p.Refresh(context.Background(), probeEntries(4), 5*time.Second)
	if d.calls.Load() != 4 || d.closed.Load() != 4 {
		t.Fatalf("dials %d, closed connections %d; want 4 and 4", d.calls.Load(), d.closed.Load())
	}
}

func TestStatusProberDefaultsMatchTheSpec(t *testing.T) {
	zero := &StatusProber{}
	if zero.timeout() != 3*time.Second || zero.concurrency() != 16 || zero.maxAge() != 10*time.Minute || zero.minInterval() != 60*time.Second {
		t.Errorf("defaults: timeout %v, concurrency %d, max age %v, min interval %v; want 3s, 16, 10m, 60s",
			zero.timeout(), zero.concurrency(), zero.maxAge(), zero.minInterval())
	}
	set := &StatusProber{Timeout: time.Second, Concurrency: 2, MaxAge: time.Minute, MinInterval: time.Hour}
	if set.timeout() != time.Second || set.concurrency() != 2 || set.maxAge() != time.Minute || set.minInterval() != time.Hour {
		t.Errorf("configured values ignored: timeout %v, concurrency %d, max age %v, min interval %v",
			set.timeout(), set.concurrency(), set.maxAge(), set.minInterval())
	}
	negative := &StatusProber{Timeout: -1, Concurrency: -1, MaxAge: -1, MinInterval: -1}
	if negative.timeout() != 3*time.Second || negative.concurrency() != 16 || negative.maxAge() != 10*time.Minute || negative.minInterval() != 60*time.Second {
		t.Errorf("negative values must fall back to the defaults")
	}
}
