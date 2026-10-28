package dockerwatch

// Seam for the part of the hub that a later task adds: the stats sampler. It
// does nothing; the task that adds the sampler replaces it.

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
