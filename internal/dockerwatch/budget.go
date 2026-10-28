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
