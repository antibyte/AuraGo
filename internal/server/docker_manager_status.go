package server

import (
	"context"
	"sync"
	"time"

	"aurago/internal/dockerhost"
)

const (
	// dockerStatusStaleAfter is the age after which GET /hosts starts a new
	// background connection test for a host.
	dockerStatusStaleAfter = 30 * time.Second
	// dockerStatusTestTimeout bounds one background test; Registry.Test limits
	// itself to 30 s.
	dockerStatusTestTimeout = 35 * time.Second
)

// dockerStatusRefresher keeps the cached host status fresh without making
// GET /hosts wait for an unreachable host: the list answers with the cached
// status at once and starts at most one background Registry.Test per stale
// host. close cancels the running tests and waits for them.
type dockerStatusRefresher struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	inflight map[string]bool
	wg       sync.WaitGroup
}

// context returns the refresher's own context; callers hold mu.
func (f *dockerStatusRefresher) context() context.Context {
	if f.ctx == nil {
		f.ctx, f.cancel = context.WithCancel(context.Background())
	}
	return f.ctx
}

// refreshStale starts a background test for every host that was never tested
// (state unknown, zero CheckedAt) or whose cached status is older than
// dockerStatusStaleAfter, and has no test running.
func (f *dockerStatusRefresher) refreshStale(reg *dockerhost.Registry, hosts []dockerhost.Host, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	base := f.context()
	if base.Err() != nil {
		return
	}
	for _, host := range hosts {
		status := reg.Status(host.ID)
		if status.State != dockerhost.StateUnknown && now.Sub(status.CheckedAt) < dockerStatusStaleAfter {
			continue
		}
		if f.inflight == nil {
			f.inflight = map[string]bool{}
		}
		if f.inflight[host.ID] {
			continue
		}
		f.inflight[host.ID] = true
		f.wg.Add(1)
		go func(id string) {
			defer f.wg.Done()
			ctx, cancel := context.WithTimeout(base, dockerStatusTestTimeout)
			defer cancel()
			reg.Test(ctx, id)
			f.mu.Lock()
			delete(f.inflight, id)
			f.mu.Unlock()
		}(host.ID)
	}
}

// close cancels the running tests, waits for them and refuses new ones.
func (f *dockerStatusRefresher) close() {
	f.mu.Lock()
	f.context()
	f.cancel()
	f.mu.Unlock()
	f.wg.Wait()
}
