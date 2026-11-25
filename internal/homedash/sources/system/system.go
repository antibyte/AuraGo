// Package system is the HomeDash source for the AuraGo host: CPU, memory and disk use, network
// throughput and uptime. It reads the sample the server's metrics collector stores (no sampler of
// its own) and has no command.
package system

import (
	"context"
	"math"
	"sync"
	"time"

	"aurago/internal/homedash"
)

// SourceID is the id of the source and the prefix of its entity ids.
const SourceID homedash.SourceID = "system"

// Native ids of the six entities.
const (
	NativeCPU    = "cpu"
	NativeMemory = "memory"
	NativeDisk   = "disk"
	NativeNetRX  = "net_rx"
	NativeNetTX  = "net_tx"
	NativeUptime = "uptime"
)

// collectorInterval is the period of the server's metrics collector (internal/server/server_routes.go).
const collectorInterval = 10 * time.Second

// Snapshot is the server's latest metrics sample.
type Snapshot struct {
	SampledAt                              time.Time
	CPUPercent, MemoryPercent, DiskPercent float64
	CPUOK, MemoryOK, DiskOK, NetworkOK     bool
	NetBytesRecv, NetBytesSent             uint64    // cumulative counters of the host
	BootTime                               time.Time // zero when unknown
}

// SnapshotFunc returns the latest sample; ok is false before the collector's first tick.
type SnapshotFunc func() (Snapshot, bool)

// HistoryPoint is one collector tick kept by the server's history ring. NetRx and NetTx (Mbit/s)
// are nil until the server has seen two samples.
type HistoryPoint struct {
	T                 time.Time
	CPU, Memory, Disk float64
	NetRx, NetTx      *float64
}

// HistoryFunc returns the ring entries with from <= T <= to, oldest first.
type HistoryFunc func(from, to time.Time) []HistoryPoint

type entityDef struct{ native, name, unit, deviceClass, icon string }

var entityDefs = []entityDef{
	{NativeCPU, "CPU", "%", "", "cpu"},
	{NativeMemory, "Memory", "%", "", "memory"},
	{NativeDisk, "Disk", "%", "", "disk"},
	{NativeNetRX, "Network receive", "Mbit/s", "data_rate", "download"},
	{NativeNetTX, "Network send", "Mbit/s", "data_rate", "upload"},
	{NativeUptime, "Uptime", "s", "duration", "timer"},
}

func (d entityDef) entity() homedash.Entity {
	return homedash.Entity{
		ID: homedash.MakeEntityID(SourceID, d.native), Source: SourceID, Kind: homedash.KindSensor,
		Name: d.name, Unit: d.unit, DeviceClass: d.deviceClass, IconHint: d.icon,
		Capabilities: []homedash.CapabilitySpec{{Name: homedash.CapNumeric, Allowed: true}, {Name: homedash.CapHistory, Allowed: true}},
		ReadOnly:     true,
	}
}

// Source is the system adapter.
type Source struct {
	snap    SnapshotFunc
	history HistoryFunc
	poll    time.Duration
	now     func() time.Time

	mu       sync.Mutex
	base     *Snapshot // the sample the last rate was measured from
	rx, tx   *float64  // last throughput in Mbit/s; nil until two samples were seen
	lastSent map[string]homedash.State
}

// New builds the adapter; pollSeconds outside 2..60 becomes 5, a nil now becomes time.Now.
func New(snap SnapshotFunc, history HistoryFunc, pollSeconds int, now func() time.Time) *Source {
	if pollSeconds < 2 || pollSeconds > 60 {
		pollSeconds = 5
	}
	if now == nil {
		now = time.Now
	}
	return &Source{snap: snap, history: history, poll: time.Duration(pollSeconds) * time.Second, now: now, lastSent: map[string]homedash.State{}}
}

// Factory builds the adapter from the runtime configuration. It is disabled without a settings
// entry, when the source is switched off, or without a snapshot function. It never blocks.
func Factory(snap SnapshotFunc, history HistoryFunc) homedash.Factory {
	return func(cfg homedash.RuntimeConfig) (homedash.Source, bool) {
		set, ok := cfg.Sources[SourceID]
		if !ok || !set.Enabled || snap == nil {
			return nil, false
		}
		return New(snap, history, set.PollSeconds, nil), true
	}
}

var _ homedash.HistorySource = (*Source)(nil)

func (s *Source) ID() homedash.SourceID { return SourceID }

// SetInterest is ignored: one sample serves all six entities.
func (s *Source) SetInterest([]string) {}

// Command always refuses: the source is read-only.
func (s *Source) Command(context.Context, homedash.Command) (homedash.CommandResult, error) {
	return homedash.CommandResult{}, &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "the system source is read-only"}
}

// Catalog returns the six entities with their current state; it does not move the rate baseline.
func (s *Source) Catalog(context.Context) ([]homedash.Entity, error) {
	snap, ok := s.snap()
	states := s.compute(snap, ok, s.now(), false)
	out := make([]homedash.Entity, 0, len(entityDefs))
	for _, d := range entityDefs {
		e := d.entity()
		e.State = states[d.native]
		out = append(out, e)
	}
	return out, nil
}

// Run sends the catalog once, then the status and the changed states every poll interval.
func (s *Source) Run(ctx context.Context, sink homedash.Sink) error {
	list, _ := s.Catalog(ctx)
	sink.Catalog(list)
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	for {
		s.pollOnce(sink)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Source) staleAfter() time.Duration { return max(3*s.poll, 3*collectorInterval) }

// pollOnce moves the rate baseline, reports the status and sends the states that changed.
func (s *Source) pollOnce(sink homedash.Sink) {
	snap, ok := s.snap()
	now := s.now()
	status := homedash.SourceStatus{Source: SourceID, State: homedash.StatusPolling, At: now}
	switch {
	case !ok:
		status.State, status.Reason = homedash.StatusUnavailable, "no_metrics"
	case now.Sub(snap.SampledAt) > s.staleAfter():
		status.State, status.Reason = homedash.StatusDegraded, "stale_metrics"
	}
	sink.Status(status)
	states := s.compute(snap, ok, now, true)
	var ups []homedash.StateUpdate
	s.mu.Lock()
	for _, d := range entityDefs {
		next := states[d.native]
		if prev, sent := s.lastSent[d.native]; sent && sameState(prev, next) {
			continue
		}
		s.lastSent[d.native] = next
		ups = append(ups, homedash.StateUpdate{Native: d.native, State: next})
	}
	s.mu.Unlock()
	if len(ups) > 0 {
		sink.Update(ups...)
	}
}

// compute maps a sample to the six states. UpdatedAt is the sample time for sample values (it never
// goes backwards, so the hub keeps every update) and now for the uptime and for "no sample yet".
func (s *Source) compute(snap Snapshot, ok bool, now time.Time, advance bool) map[string]homedash.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ok && advance {
		s.advanceLocked(snap)
	}
	off := homedash.State{Value: nil, Available: false, UpdatedAt: now}
	if ok {
		off.UpdatedAt = snap.SampledAt
	}
	stale := ok && now.Sub(snap.SampledAt) > s.staleAfter()
	sample := func(v float64, avail bool) homedash.State {
		if !ok || !avail {
			return off
		}
		return homedash.State{Value: v, Available: true, Stale: stale, UpdatedAt: snap.SampledAt}
	}
	out := map[string]homedash.State{
		NativeCPU:    sample(round(snap.CPUPercent, 1), snap.CPUOK),
		NativeMemory: sample(round(snap.MemoryPercent, 1), snap.MemoryOK),
		NativeDisk:   sample(round(snap.DiskPercent, 1), snap.DiskOK),
		NativeNetRX:  off,
		NativeNetTX:  off,
		NativeUptime: {Value: nil, Available: false, UpdatedAt: now},
	}
	if s.rx != nil {
		out[NativeNetRX] = sample(*s.rx, snap.NetworkOK)
	}
	if s.tx != nil {
		out[NativeNetTX] = sample(*s.tx, snap.NetworkOK)
	}
	if ok && !snap.BootTime.IsZero() && now.After(snap.BootTime) {
		out[NativeUptime] = homedash.State{Value: math.Floor(now.Sub(snap.BootTime).Seconds()), Available: true, UpdatedAt: now}
	}
	return out
}

// advanceLocked measures the throughput between the baseline and a newer sample. A counter that
// went down (host reboot, interface reset) moves the baseline and keeps the last rate.
func (s *Source) advanceLocked(snap Snapshot) {
	if !snap.NetworkOK {
		return
	}
	b := s.base
	if b != nil && !snap.SampledAt.After(b.SampledAt) {
		return
	}
	if b != nil && snap.NetBytesRecv >= b.NetBytesRecv && snap.NetBytesSent >= b.NetBytesSent {
		if dt := snap.SampledAt.Sub(b.SampledAt).Seconds(); dt > 0 {
			rx := round(float64(snap.NetBytesRecv-b.NetBytesRecv)*8/1e6/dt, 3)
			tx := round(float64(snap.NetBytesSent-b.NetBytesSent)*8/1e6/dt, 3)
			s.rx, s.tx = &rx, &tx
		}
	}
	cp := snap
	s.base = &cp
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// sameState reports a state that need not be sent again (unavailable states ignore UpdatedAt).
func sameState(a, b homedash.State) bool {
	return a.Available == b.Available && a.Stale == b.Stale && a.Value == b.Value &&
		(!a.Available || a.UpdatedAt.Equal(b.UpdatedAt))
}

// History serves the server's ring of collector samples (lead amendment A1): the entries of the
// metric inside [from, to], averaged into at most points equal time buckets. Empty buckets are
// skipped, a bucket's T is its last entry's T (like the hub's downsample). The adapter keeps no
// history of its own and does not hold s.mu while reading.
func (s *Source) History(_ context.Context, native string, from, to time.Time, points int) ([]homedash.Sample, error) {
	out := []homedash.Sample{}
	if s.history == nil || points < 1 || !to.After(from) {
		return out, nil
	}
	var pick func(HistoryPoint) (float64, bool)
	places := 1
	switch native {
	case NativeCPU:
		pick = func(p HistoryPoint) (float64, bool) { return p.CPU, true }
	case NativeMemory:
		pick = func(p HistoryPoint) (float64, bool) { return p.Memory, true }
	case NativeDisk:
		pick = func(p HistoryPoint) (float64, bool) { return p.Disk, true }
	case NativeNetRX:
		places = 3
		pick = func(p HistoryPoint) (float64, bool) {
			if p.NetRx == nil {
				return 0, false
			}
			return *p.NetRx, true
		}
	case NativeNetTX:
		places = 3
		pick = func(p HistoryPoint) (float64, bool) {
			if p.NetTx == nil {
				return 0, false
			}
			return *p.NetTx, true
		}
	default:
		return out, nil
	}
	type bucket struct {
		sum  float64
		n    int
		last time.Time
	}
	buckets := make([]bucket, points)
	span := float64(to.Sub(from))
	for _, p := range s.history(from, to) {
		if p.T.Before(from) || p.T.After(to) {
			continue
		}
		v, ok := pick(p)
		if !ok {
			continue
		}
		i := int(float64(p.T.Sub(from)) / span * float64(points))
		i = min(max(i, 0), points-1)
		b := &buckets[i]
		b.sum += v
		b.n++
		if b.n == 1 || p.T.After(b.last) {
			b.last = p.T
		}
	}
	for _, b := range buckets {
		if b.n > 0 {
			out = append(out, homedash.Sample{T: b.last, V: round(b.sum/float64(b.n), places)})
		}
	}
	return out, nil
}
