package dockerwatch

import "sync"

// Acquire takes one of the host's MaxStreamsPerHost streaming-connection
// slots. The events watcher, every followed container log and every terminal
// holds one while it runs. It fails with ErrTooManyStreams when all are taken;
// release gives the slot back and may be called more than once.
func (h *Hub) Acquire(host string) (release func(), err error) {
	h.budgetMu.Lock()
	defer h.budgetMu.Unlock()
	if h.streams[host] >= MaxStreamsPerHost {
		return nil, ErrTooManyStreams
	}
	h.streams[host]++
	var once sync.Once
	return func() {
		once.Do(func() {
			h.budgetMu.Lock()
			defer h.budgetMu.Unlock()
			if h.streams[host]--; h.streams[host] <= 0 {
				delete(h.streams, host)
			}
		})
	}, nil
}

// acquireProjectFollow takes one of the host's MaxProjectFollowsPerHost project
// log follows. A stack or project follow holds it for its whole run, next to the
// stream slot of every container it follows; the cap keeps the history reads
// (every container's tail, merged) of many requests from piling up. It fails
// with ErrTooManyProjectFollows when all are taken; release gives the slot back
// and may be called more than once.
func (h *Hub) acquireProjectFollow(host string) (release func(), err error) {
	h.budgetMu.Lock()
	defer h.budgetMu.Unlock()
	if h.projects[host] >= MaxProjectFollowsPerHost {
		return nil, ErrTooManyProjectFollows
	}
	h.projects[host]++
	var once sync.Once
	return func() {
		once.Do(func() {
			h.budgetMu.Lock()
			defer h.budgetMu.Unlock()
			if h.projects[host]--; h.projects[host] <= 0 {
				delete(h.projects, host)
			}
		})
	}, nil
}
