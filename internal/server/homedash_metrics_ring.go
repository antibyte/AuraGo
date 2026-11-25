package server

import (
	"math"
	"sync"
	"time"

	"aurago/internal/homedash/sources/system"
)

// homeDashMetricsRingCap is 24 h of collector samples at one entry per 10 s tick.
const homeDashMetricsRingCap = 8640

// homeDashRingEntry stores the rates by value so reads never alias ring memory.
type homeDashRingEntry struct {
	t                 time.Time
	cpu, memory, disk float64
	rx, tx            float64
	hasRate           bool
}

// homeDashMetricsRing keeps the collector's recent samples for the HomeDash system history.
// The zero value works; storage is allocated on the first append.
type homeDashMetricsRing struct {
	mu    sync.Mutex
	buf   []homeDashRingEntry
	start int // index of the oldest entry
	n     int // number of entries

	prev    systemMetricsSnapshot // raw baseline for the rate computation
	hasPrev bool
}

func (r *homeDashMetricsRing) push(e homeDashRingEntry) {
	if r.buf == nil {
		r.buf = make([]homeDashRingEntry, homeDashMetricsRingCap)
	}
	if r.n < len(r.buf) {
		r.buf[(r.start+r.n)%len(r.buf)] = e
		r.n++
		return
	}
	r.buf[r.start] = e
	r.start = (r.start + 1) % len(r.buf)
}

func (r *homeDashMetricsRing) appendLocked(p system.HistoryPoint) {
	e := homeDashRingEntry{t: p.T, cpu: p.CPU, memory: p.Memory, disk: p.Disk}
	if p.NetRx != nil && p.NetTx != nil {
		e.rx, e.tx, e.hasRate = *p.NetRx, *p.NetTx, true
	}
	r.push(e)
}

// append adds one point, dropping the oldest when the ring is full.
func (r *homeDashMetricsRing) append(p system.HistoryPoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendLocked(p)
}

// between returns a copy of the entries with from <= T <= to, oldest first (never nil).
func (r *homeDashMetricsRing) between(from, to time.Time) []system.HistoryPoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []system.HistoryPoint{}
	for i := 0; i < r.n; i++ {
		e := r.buf[(r.start+i)%len(r.buf)]
		if e.t.Before(from) || e.t.After(to) {
			continue
		}
		p := system.HistoryPoint{T: e.t, CPU: e.cpu, Memory: e.memory, Disk: e.disk}
		if e.hasRate {
			rx, tx := e.rx, e.tx
			p.NetRx, p.NetTx = &rx, &tx
		}
		out = append(out, p)
	}
	return out
}

func roundMbit(v float64) float64 { return math.Round(v*1000) / 1000 }

// record turns one snapshot into a ring entry; the rate mirrors the system adapter's advanceLocked.
func (r *homeDashMetricsRing) record(snap systemMetricsSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// A metric the collector could not read is NaN so history never plots it as 0 %.
	pct := func(v float64, ok bool) float64 {
		if !ok {
			return math.NaN()
		}
		return v
	}
	p := system.HistoryPoint{
		T:      snap.SampledAt,
		CPU:    pct(snap.CPUPercent, snap.CPUAvailable),
		Memory: pct(snap.MemoryPercent, snap.MemoryAvailable),
		Disk:   pct(snap.DiskPercent, snap.DiskAvailable),
	}
	if snap.NetworkAvailable {
		if r.hasPrev && snap.NetBytesRecv >= r.prev.NetBytesRecv && snap.NetBytesSent >= r.prev.NetBytesSent {
			if dt := snap.SampledAt.Sub(r.prev.SampledAt).Seconds(); dt > 0 {
				rx := roundMbit(float64(snap.NetBytesRecv-r.prev.NetBytesRecv) * 8 / 1e6 / dt)
				tx := roundMbit(float64(snap.NetBytesSent-r.prev.NetBytesSent) * 8 / 1e6 / dt)
				p.NetRx, p.NetTx = &rx, &tx
			}
		}
		r.prev, r.hasPrev = snap, true
	}
	r.appendLocked(p)
}

// latestRates returns the rates of the newest entry when it belongs to the sample taken at at.
func (r *homeDashMetricsRing) latestRates(at time.Time) (rx, tx *float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return nil, nil
	}
	e := r.buf[(r.start+r.n-1)%len(r.buf)]
	if !e.hasRate || !e.t.Equal(at) {
		return nil, nil
	}
	a, b := e.rx, e.tx
	return &a, &b
}

// recordHomeDashMetrics appends the just-stored collector sample to the history ring.
func (s *Server) recordHomeDashMetrics() {
	snap, ok := s.loadSystemMetricsSnapshot()
	if !ok {
		return
	}
	s.homeDashMetrics.record(snap)
}

// homeDashSystemHistory serves the HomeDash system source's History (lead amendment A1).
func (s *Server) homeDashSystemHistory(from, to time.Time) []system.HistoryPoint {
	if s == nil {
		return []system.HistoryPoint{}
	}
	return s.homeDashMetrics.between(from, to)
}
