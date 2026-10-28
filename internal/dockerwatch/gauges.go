package dockerwatch

import (
	"context"
	"math"
	"slices"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

const (
	infoTTL         = 5 * time.Minute // NCPU and MemTotal of a host change rarely
	gaugePrimeDelay = 2 * time.Second // the first cycle has no CPU deltas yet; the second follows soon
)

// gaugeLoop samples every running container of a host (up to MaxHostSample)
// every HostInterval and sends the sums as `host_stats`, for as long as a
// client asks for the host's gauges.
func (h *Hub) gaugeLoop(ctx context.Context, rt *hostRT) {
	defer h.wg.Done()
	delay := time.Duration(0)
	first := true
	for {
		if !sleepCtx(ctx, delay) {
			return
		}
		h.gaugeCycle(ctx, rt)
		delay = h.opts.HostInterval
		if first {
			first = false
			delay = min(delay, gaugePrimeDelay)
		}
	}
}

// gaugeCycle takes one reading of the host. A container that is also in a
// stats subscription is not sampled again while its newest point is at most
// HostInterval old.
func (h *Hub) gaugeCycle(ctx context.Context, rt *hostRT) {
	c, err := h.hosts.Client(ctx, rt.id)
	if err != nil {
		h.log.Debug("docker gauges: no client", "host", rt.id, "error", err)
		return
	}
	list, err := dockerhost.ListContainers(ctx, c)
	if err != nil {
		h.log.Debug("docker gauges: container list failed", "host", rt.id, "error", err)
		return
	}
	var ids []string
	running := 0
	for _, ctr := range list {
		if ctr.State != "running" {
			continue
		}
		running++
		if len(ids) < MaxHostSample {
			ids = append(ids, ctr.ID)
		}
	}
	oneShot, err := supportsOneShot(ctx, c)
	if err != nil {
		h.log.Debug("docker gauges: no API version", "host", rt.id, "error", err)
		return
	}
	info, haveInfo := h.hostInfo(ctx, rt, c)

	now := h.opts.Now()
	points := make([]*StatsPoint, len(ids))
	var stale []string
	var staleAt []int
	h.mu.Lock()
	_, _, interest := h.interestLocked(rt.id)
	h.sweepLocked(rt, interest) // the gauge loop alone must not let old samples pile up
	for i, id := range ids {
		if r := rt.st.rings[id]; r != nil && len(r.points) > 0 {
			if last := r.points[len(r.points)-1]; now.Sub(time.UnixMilli(last.T)) <= h.opts.HostInterval {
				points[i] = &last
				continue
			}
		}
		stale = append(stale, id)
		staleAt = append(staleAt, i)
	}
	h.mu.Unlock()
	h.sampleMany(ctx, rt, c, oneShot, stale, func(i int, _ string, pt StatsPoint, res sampleResult) {
		if res == sampled {
			points[staleAt[i]] = &pt // every worker writes its own index
		}
	})
	if ctx.Err() != nil {
		return
	}

	var cpuSum float64
	var mem uint64
	known, got := 0, 0
	for _, pt := range points {
		if pt == nil {
			continue
		}
		got++
		mem += pt.Mem
		if pt.CPU >= 0 {
			cpuSum += pt.CPU
			known++
		}
	}
	cpu := -1.0
	switch {
	case len(ids) == 0:
		cpu = 0
	case known > 0 && haveInfo && info.NCPU > 0:
		cpu = math.Min(math.Max(math.Round(cpuSum/float64(info.NCPU)*100)/100, 0), 100)
	}
	stats := HostStats{
		Host: rt.id, T: now.UnixMilli(), CPU: cpu, NCPU: info.NCPU, MemUsed: mem, MemTotal: uint64(max(info.MemTotal, 0)),
		Running: running, Sampled: got, Partial: running > len(ids),
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || ctx.Err() != nil {
		return
	}
	for _, cl := range h.clients {
		if slices.Contains(cl.sub.HostStats, rt.id) {
			cl.push(Message{Event: MsgHostStats, Data: stats})
		}
	}
}

// hostInfo returns NCPU and MemTotal of the host, read at most every infoTTL;
// a failing refresh keeps the previous values.
func (h *Hub) hostInfo(ctx context.Context, rt *hostRT, c *dockerutil.Client) (dockerhost.EngineInfo, bool) {
	now := h.opts.Now()
	h.mu.Lock()
	cached := rt.st.info
	h.mu.Unlock()
	if cached.ok && now.Sub(cached.at) < infoTTL {
		return cached.info, true
	}
	info, err := dockerhost.Info(ctx, c)
	if err != nil {
		h.log.Debug("docker gauges: info failed", "host", rt.id, "error", err)
		return cached.info, cached.ok
	}
	h.mu.Lock()
	rt.st.info = infoCache{info: info, at: now, ok: true}
	h.mu.Unlock()
	return info, true
}
