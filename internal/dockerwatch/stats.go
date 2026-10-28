package dockerwatch

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

const (
	ringTTL         = 5 * time.Minute // a ring nobody has sampled for this long is freed
	maxStatsCalls   = 4               // Engine stats calls in flight per host
	oneShotMinMinor = 41              // stats?one-shot=true needs API 1.41
)

// statsState is the stats side of a host runtime; Hub.mu guards it.
type statsState struct {
	rings  map[string]*ring    // by 12-char id
	raws   map[string]rawEntry // previous sample per container, for the CPU delta
	gone   map[string]bool     // ids whose stats call said gone; not sampled until re-subscribed or started
	loop   bool                // the 3 s sampling loop runs
	gauges context.CancelFunc  // the host gauge loop; nil when stopped
	kick   chan struct{}       // wakes the 3 s loop
	sem    chan struct{}       // at most maxStatsCalls Engine calls in flight, both loops together
	info   infoCache
}

type rawEntry struct {
	raw dockerhost.RawStats
	at  time.Time
}

type infoCache struct {
	info dockerhost.EngineInfo
	at   time.Time
	ok   bool
}

func newStatsState() *statsState {
	return &statsState{
		rings: map[string]*ring{},
		raws:  map[string]rawEntry{},
		gone:  map[string]bool{},
		kick:  make(chan struct{}, 1),
		sem:   make(chan struct{}, maxStatsCalls),
	}
}

// idle reports that nothing of the stats side runs.
func (s *statsState) idle() bool { return !s.loop && s.gauges == nil }

// wake makes the sampling loop take a sample now.
func (s *statsState) wake() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// ring holds the newest RingSize points of a container.
type ring struct {
	points []StatsPoint
	last   time.Time // when the newest point was taken
}

func (r *ring) add(pt StatsPoint, now time.Time) {
	r.points = append(r.points, pt)
	if len(r.points) > RingSize {
		r.points = append(r.points[:0], r.points[len(r.points)-RingSize:]...)
	}
	r.last = now
}

func (r *ring) snapshot() []StatsPoint { return slices.Clone(r.points) }

// reconcileStatsLocked starts the sampling loops a host needs. The 3 s loop
// stops itself once nobody wants stats and its rings are freed.
func (h *Hub) reconcileStatsLocked(rt *hostRT, wantStats, wantGauges bool) {
	st := rt.st
	if wantStats && !st.loop {
		st.loop = true
		h.wg.Add(1)
		go h.statsLoop(rt)
	}
	switch {
	case wantGauges && st.gauges == nil:
		ctx, cancel := context.WithCancel(h.ctx)
		st.gauges = cancel
		h.wg.Add(1)
		go h.gaugeLoop(ctx, rt)
	case !wantGauges && st.gauges != nil:
		st.gauges()
		st.gauges = nil
	}
}

// subscribeStatsLocked is what a client gets when it starts asking for a
// container: the history of an existing ring first, live points after.
func (h *Hub) subscribeStatsLocked(rt *hostRT, c *Client, id string) {
	delete(rt.st.gone, id)
	if r := rt.st.rings[id]; r != nil && len(r.points) > 0 {
		c.push(Message{Event: MsgStatsHistory, Data: StatsHistoryMsg{Host: rt.id, ID: id, Points: r.snapshot()}})
	}
}

func (h *Hub) publishStatsLocked(host, id string, msg Message) {
	for _, c := range h.clients {
		if slices.Contains(c.sub.Stats[host], id) {
			c.push(msg)
		}
	}
}

// statsLoop samples the containers clients ask for every StatsInterval.
func (h *Hub) statsLoop(rt *hostRT) {
	defer h.wg.Done()
	ticker := time.NewTicker(h.opts.StatsInterval)
	defer ticker.Stop()
	for {
		if !h.statsTick(h.ctx, rt) {
			return
		}
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
		case <-rt.st.kick:
		}
	}
}

// statsTick takes one round of samples. It reports false when the loop has
// nothing left to do and has ended itself.
func (h *Hub) statsTick(ctx context.Context, rt *hostRT) bool {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return false
	}
	_, _, interest := h.interestLocked(rt.id)
	h.sweepLocked(rt, interest)
	if len(interest) == 0 && len(rt.st.rings) == 0 {
		rt.st.loop = false
		h.gcLocked(rt)
		h.mu.Unlock()
		return false
	}
	ids := make([]string, 0, len(interest))
	for _, id := range interest {
		if !rt.st.gone[id] {
			ids = append(ids, id)
		}
	}
	h.mu.Unlock()
	if len(ids) == 0 {
		return true
	}
	c, err := h.hosts.Client(ctx, rt.id)
	if err != nil {
		h.log.Debug("docker stats: no client", "host", rt.id, "error", err)
		return true
	}
	oneShot, err := supportsOneShot(ctx, c)
	if err != nil {
		h.log.Debug("docker stats: no API version", "host", rt.id, "error", err)
		return true
	}
	h.sampleMany(ctx, rt, c, oneShot, ids, func(_ int, id string, pt StatsPoint, res sampleResult) {
		switch res {
		case sampled:
			h.recordPoint(rt, id, pt)
		case sampleGone:
			h.markGone(rt, id)
		}
	})
	return true
}

// supportsOneShot tells whether the host's Engine takes stats?one-shot=true.
func supportsOneShot(ctx context.Context, c *dockerutil.Client) (bool, error) {
	version, err := c.APIVersion(ctx)
	if err != nil {
		return false, err
	}
	major, minor, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(minor)
	if err != nil {
		return false, err
	}
	return major == "1" && n >= oneShotMinMinor, nil
}

type sampleResult int

const (
	sampleFailed sampleResult = iota // a transient error: try again next round
	sampled
	sampleGone // 404/409 or a container that is not running
)

// sampleMany samples ids with at most maxStatsCalls workers and hands every
// result to done (called from the workers; i is the index in ids).
func (h *Hub) sampleMany(ctx context.Context, rt *hostRT, c *dockerutil.Client, oneShot bool, ids []string, done func(i int, id string, pt StatsPoint, res sampleResult)) {
	work := make(chan int)
	var wg sync.WaitGroup
	for range min(maxStatsCalls, len(ids)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				pt, res := h.sampleOne(ctx, rt, c, oneShot, ids[i])
				done(i, ids[i], pt, res)
			}
		}()
	}
	for i := range ids {
		work <- i
	}
	close(work)
	wg.Wait()
}

// sampleOne takes one stats sample of a container, within the host's limit of
// calls in flight, and turns it into a point.
func (h *Hub) sampleOne(ctx context.Context, rt *hostRT, c *dockerutil.Client, oneShot bool, id string) (StatsPoint, sampleResult) {
	select {
	case rt.st.sem <- struct{}{}:
	case <-ctx.Done():
		return StatsPoint{}, sampleFailed
	}
	raw, err := dockerhost.ContainerStats(ctx, c, id, oneShot)
	<-rt.st.sem
	if err != nil {
		if status := dockerhost.StatusOf(err); status == 404 || status == 409 {
			return StatsPoint{}, sampleGone
		}
		if ctx.Err() == nil {
			h.log.Debug("docker stats call failed", "host", rt.id, "container", id, "error", err)
		}
		return StatsPoint{}, sampleFailed
	}
	if notRunning(raw) {
		return StatsPoint{}, sampleGone
	}
	now := h.opts.Now()
	h.mu.Lock()
	prev, hadPrev := rt.st.raws[id]
	rt.st.raws[id] = rawEntry{raw: raw, at: now}
	h.mu.Unlock()
	var before *dockerhost.RawStats
	if hadPrev {
		before = &prev.raw
	}
	return pointOf(now, raw, cpuPercent(before, raw, oneShot)), sampled
}

// notRunning recognises the all-zero sample the Engine answers for a container
// that is not running.
func notRunning(raw dockerhost.RawStats) bool {
	return raw.Read.IsZero() && raw.MemUsage == 0 && raw.CPUTotal == 0
}

func pointOf(now time.Time, raw dockerhost.RawStats, cpu float64) StatsPoint {
	mem := raw.MemUsage
	if raw.MemCache < mem {
		mem -= raw.MemCache
	}
	return StatsPoint{
		T: now.UnixMilli(), CPU: cpu, Mem: mem, MemLimit: raw.MemLimit,
		NetRx: raw.NetRx, NetTx: raw.NetTx, BlkR: raw.BlkRead, BlkW: raw.BlkWrite, Pids: raw.Pids,
	}
}

// cpuPercent is the container's CPU use in percent of one core (a container
// that fills two cores reads 200): the share of the system's CPU time it used,
// times the number of online CPUs. A one-shot sample carries no previous
// sample, so the delta is against our own previous one (before; nil on the
// first sample, which is unknown); otherwise the Engine filled precpu_stats.
// A system delta that is not positive, or a counter that went backwards (the
// container restarted), is unknown (-1). A container that used no CPU reads 0.
func cpuPercent(before *dockerhost.RawStats, cur dockerhost.RawStats, oneShot bool) float64 {
	prevTotal, prevSystem := cur.PreCPUTotal, cur.PreSystem
	if oneShot {
		if before == nil {
			return -1
		}
		prevTotal, prevSystem = before.CPUTotal, before.SystemCPU
	} else if prevSystem == 0 {
		return -1 // the Engine did not fill precpu_stats
	}
	if cur.SystemCPU <= prevSystem || cur.CPUTotal < prevTotal {
		return -1
	}
	cpus := cur.OnlineCPUs
	if cpus <= 0 {
		cpus = 1
	}
	pct := float64(cur.CPUTotal-prevTotal) / float64(cur.SystemCPU-prevSystem) * float64(cpus) * 100
	return math.Round(pct*100) / 100
}

// recordPoint stores a point in the container's ring and sends it to the
// clients that ask for the container.
func (h *Hub) recordPoint(rt *hostRT, id string, pt StatsPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || rt.st.gone[id] {
		return
	}
	r := rt.st.rings[id]
	if r == nil {
		r = &ring{}
		rt.st.rings[id] = r
	}
	r.add(pt, h.opts.Now())
	h.publishStatsLocked(rt.id, id, Message{Event: MsgStats, Data: StatsMsg{Host: rt.id, ID: id, Point: pt}})
}

// markGone stops sampling a container that no longer exists or is not running
// and tells the clients that asked for it, once.
func (h *Hub) markGone(rt *hostRT, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || rt.st.gone[id] {
		return
	}
	rt.st.gone[id] = true
	delete(rt.st.rings, id)
	delete(rt.st.raws, id)
	h.publishStatsLocked(rt.id, id, Message{Event: MsgStatsGone, Data: StatsGoneMsg{Host: rt.id, ID: id}})
}

// sweepLocked frees what nobody has sampled for ringTTL and forgets gone marks
// of containers no client asks for any more.
func (h *Hub) sweepLocked(rt *hostRT, interest []string) {
	now := h.opts.Now()
	for id, r := range rt.st.rings {
		if now.Sub(r.last) > ringTTL {
			delete(rt.st.rings, id)
			delete(rt.st.raws, id)
		}
	}
	for id, e := range rt.st.raws {
		if now.Sub(e.at) > ringTTL {
			delete(rt.st.raws, id)
		}
	}
	for id := range rt.st.gone {
		if !slices.Contains(interest, id) {
			delete(rt.st.gone, id)
		}
	}
}
