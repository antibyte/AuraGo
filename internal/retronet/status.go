package retronet

import (
	"context"
	"sync"
	"time"
)

// Reachability states.
const (
	statusOnline  = "online"
	statusOffline = "offline"
	statusUnknown = "unknown"
)

// Prober defaults (spec: 3 s TCP connect, 16 parallel probes, 10 min cache, 60 s refresh floor).
const (
	defaultProbeTimeout     = 3 * time.Second
	defaultProbeConcurrency = 16
	defaultStatusMaxAge     = 10 * time.Minute
	defaultRefreshInterval  = 60 * time.Second
)

// Status is the cached reachability of one entry.
type Status struct {
	State        string     `json:"state"` // "online" | "offline" | "unknown"
	CheckedAt    *time.Time `json:"checked_at,omitempty"`
	LastOnlineAt *time.Time `json:"last_online_at,omitempty"`
}

// StatusProber caches TCP reachability of directory entries in memory. The zero value is
// ready to use; a StatusProber must not be copied after first use.
type StatusProber struct {
	Dialer      Dialer
	Timeout     time.Duration    // 0 -> 3 * time.Second per probe
	Concurrency int              // 0 -> 16
	MaxAge      time.Duration    // 0 -> 10 * time.Minute
	MinInterval time.Duration    // 0 -> 60 * time.Second between forced refreshes
	Now         func() time.Time // nil -> time.Now

	mu       sync.Mutex
	cache    map[string]Status
	started  time.Time     // start of the most recent probe run (zero: never)
	finished time.Time     // end of the most recent completed probe run (zero: never)
	inflight chan struct{} // closed when the running probe ends; nil when idle
}

// Snapshot returns cached status for entries (missing -> "unknown") and whether the cache is
// stale; when stale it starts one background refresh (never more than one in flight).
//
// The cache is stale before the first completed probe, when the last probe finished more than
// MaxAge ago, and when an entry has never been probed (for example a new own entry).
func (p *StatusProber) Snapshot(entries []Entry) (map[string]Status, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	statuses, missing := p.statusesLocked(entries)
	stale := missing || p.finished.IsZero() || now.Sub(p.finished) > p.maxAge()
	if stale && p.inflight == nil {
		p.startLocked(entries, now)
	}
	return statuses, stale
}

// Refresh forces a probe unless the last one started < MinInterval ago, waits up to wait for
// the in-flight probe and returns the status map.
func (p *StatusProber) Refresh(ctx context.Context, entries []Entry, wait time.Duration) map[string]Status {
	p.mu.Lock()
	now := p.now()
	if p.inflight == nil && (p.started.IsZero() || now.Sub(p.started) >= p.minInterval()) {
		p.startLocked(entries, now)
	}
	done := p.inflight
	p.mu.Unlock()
	if done != nil {
		timer := time.NewTimer(wait)
		select {
		case <-done:
		case <-timer.C:
		case <-ctx.Done():
		}
		timer.Stop()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	statuses, _ := p.statusesLocked(entries)
	return statuses
}

// statusesLocked copies the cached status of every entry and reports whether one is missing.
func (p *StatusProber) statusesLocked(entries []Entry) (map[string]Status, bool) {
	out := make(map[string]Status, len(entries))
	missing := false
	for _, e := range entries {
		st, ok := p.cache[e.ID]
		if !ok {
			out[e.ID] = Status{State: statusUnknown}
			missing = true
			continue
		}
		out[e.ID] = cloneStatus(st)
	}
	return out, missing
}

// startLocked launches one probe run over a copy of entries.
func (p *StatusProber) startLocked(entries []Entry, now time.Time) {
	done := make(chan struct{})
	p.inflight = done
	p.started = now
	go p.probeAll(append([]Entry(nil), entries...), done)
}

// probeAll probes entries with at most Concurrency dials at a time, then publishes the run.
func (p *StatusProber) probeAll(entries []Entry, done chan struct{}) {
	sem := make(chan struct{}, p.concurrency())
	var wg sync.WaitGroup
	for _, e := range entries {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			p.record(e.ID, p.probe(e))
		})
	}
	wg.Wait()
	p.mu.Lock()
	p.finished = p.now()
	p.inflight = nil
	p.pruneLocked(entries)
	p.mu.Unlock()
	close(done)
}

// probe is a guarded TCP connect, closed immediately.
func (p *StatusProber) probe(e Entry) bool {
	timeout := p.timeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	dialer := p.Dialer
	dialer.Timeout = timeout
	conn, _, err := dialer.DialEntry(ctx, e)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// record stores one probe outcome; offline results keep the last online time.
func (p *StatusProber) record(id string, online bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cache == nil {
		p.cache = make(map[string]Status)
	}
	checked := p.now()
	st := p.cache[id]
	st.CheckedAt = &checked
	if online {
		st.State = statusOnline
		lastOnline := checked
		st.LastOnlineAt = &lastOnline
	} else {
		st.State = statusOffline
	}
	p.cache[id] = st
}

// pruneLocked drops cached entries that were not part of the last probe run.
func (p *StatusProber) pruneLocked(entries []Entry) {
	keep := make(map[string]bool, len(entries))
	for _, e := range entries {
		keep[e.ID] = true
	}
	for id := range p.cache {
		if !keep[id] {
			delete(p.cache, id)
		}
	}
}

// cloneStatus copies s so callers never share time values with the cache.
func cloneStatus(s Status) Status {
	if s.CheckedAt != nil {
		checked := *s.CheckedAt
		s.CheckedAt = &checked
	}
	if s.LastOnlineAt != nil {
		lastOnline := *s.LastOnlineAt
		s.LastOnlineAt = &lastOnline
	}
	return s
}

func (p *StatusProber) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p *StatusProber) timeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return defaultProbeTimeout
}

func (p *StatusProber) concurrency() int {
	if p.Concurrency > 0 {
		return p.Concurrency
	}
	return defaultProbeConcurrency
}

func (p *StatusProber) maxAge() time.Duration {
	if p.MaxAge > 0 {
		return p.MaxAge
	}
	return defaultStatusMaxAge
}

func (p *StatusProber) minInterval() time.Duration {
	if p.MinInterval > 0 {
		return p.MinInterval
	}
	return defaultRefreshInterval
}
