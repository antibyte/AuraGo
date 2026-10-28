package dockerwatch

import (
	"context"
	"errors"
	"time"

	"aurago/internal/dockerhost"
)

const (
	dfTTL          = 5 * time.Minute  // how long a disk-usage answer is served from the cache
	dfRefreshAfter = 60 * time.Second // a forced refresh re-reads only when the cache is older than this
)

// dfEntry is the cached disk usage of one host. gate admits one reader at a
// time, so concurrent requests share one slow /system/df call.
type dfEntry struct {
	gate  chan struct{}
	usage dockerhost.DiskUsage
	at    time.Time
	have  bool
}

func (h *Hub) dfEntryFor(host string) *dfEntry {
	h.dfMu.Lock()
	defer h.dfMu.Unlock()
	e := h.df[host]
	if e == nil {
		e = &dfEntry{gate: make(chan struct{}, 1)}
		h.df[host] = e
	}
	return e
}

// DiskUsage returns Docker's disk use on host, the time it was read and
// whether it came from the cache. An answer younger than 5 minutes is served
// from the cache; refresh asks for a new reading but is honoured only when the
// cached one is older than 60 seconds (the Engine call is slow on large
// hosts). A failed reading leaves the cache as it was and returns the error
// unchanged.
func (h *Hub) DiskUsage(ctx context.Context, host string, refresh bool) (dockerhost.DiskUsage, time.Time, bool, error) {
	e := h.dfEntryFor(host)
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return dockerhost.DiskUsage{}, time.Time{}, false, ctx.Err()
	}
	defer func() { <-e.gate }()

	if e.have {
		age := h.opts.Now().Sub(e.at)
		if age < dfTTL && (!refresh || age < dfRefreshAfter) {
			return e.usage, e.at, true, nil
		}
	}
	c, err := h.hosts.Client(ctx, host)
	if err != nil {
		if errors.Is(err, dockerhost.ErrHostNotFound) {
			h.dfMu.Lock()
			delete(h.df, host)
			h.dfMu.Unlock()
		}
		return dockerhost.DiskUsage{}, time.Time{}, false, err
	}
	usage, err := dockerhost.SystemDF(ctx, c)
	if err != nil {
		return dockerhost.DiskUsage{}, time.Time{}, false, err
	}
	e.usage, e.at, e.have = usage, h.opts.Now(), true
	return usage, e.at, false, nil
}
