package dockerwatch

// Seams for the parts of the hub that later tasks add: the events watcher and
// the stats sampler. They do nothing; the tasks that add those parts replace
// them.

// watcher follows the events of one host.
type watcher struct{}

func (w *watcher) stop() {}

func (h *Hub) startWatcherLocked(rt *hostRT) {}

// statsState is the stats side of a host runtime.
type statsState struct {
	loop bool
	gone map[string]bool
}

func newStatsState() *statsState { return &statsState{gone: map[string]bool{}} }

func (s *statsState) idle() bool { return true }

func (s *statsState) wake() {}

func (h *Hub) reconcileStatsLocked(rt *hostRT, wantStats, wantGauges bool) {}

func (h *Hub) subscribeStatsLocked(rt *hostRT, c *Client, id string) {}
