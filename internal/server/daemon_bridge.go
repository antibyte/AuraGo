package server

// daemonSSEAdapter bridges tools.DaemonEventBroadcaster to the concrete SSEBroadcaster.
// This avoids an import cycle: tools cannot import server, so the supervisor
// receives a DaemonEventBroadcaster interface; this adapter satisfies it.
type daemonSSEAdapter struct {
	sse *SSEBroadcaster
}

func newDaemonSSEAdapter(sse *SSEBroadcaster) *daemonSSEAdapter {
	return &daemonSSEAdapter{sse: sse}
}

func (a *daemonSSEAdapter) BroadcastDaemonEvent(eventType string, payload any) {
	a.sse.BroadcastType(SSEEventType(eventType), payload)
}
