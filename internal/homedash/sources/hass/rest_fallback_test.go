package hass

import (
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

func TestHAFallsBackToRESTPolling(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	ha.BlockWebsocket(true)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusPolling, ReasonWebsocketUnavailable) }, "polling after three failed handshakes")
	n := ha.RESTStateRequests()
	ha.SetState(hasstest.State{EntityID: "switch.coffee", State: "on", Attributes: map[string]any{"friendly_name": "Coffee machine"}}, false)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { st, ok := sink.last("switch.coffee"); return ok && st.Value == true }, "polled change")
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return ha.RESTStateRequests() > n+1 }, "polling every interval")

	ha.BlockWebsocket(false)
	sourcetest.WaitFor(t, 3*time.Second, func() bool { return sink.lastStatus().State == homedash.StatusLive }, "websocket again after the polling window")
	m := ha.RESTStateRequests()
	time.Sleep(150 * time.Millisecond)
	if ha.RESTStateRequests() != m {
		t.Fatal("polling continued while the websocket is live")
	}
}

func TestHAPollingNeedsInterest(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	ha.BlockWebsocket(true)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusPolling, "") }, "polling")
	n := ha.RESTStateRequests()
	time.Sleep(150 * time.Millisecond)
	if ha.RESTStateRequests() != n {
		t.Fatalf("polled without interest: %d -> %d", n, ha.RESTStateRequests())
	}
}

// A failing REST poll means Home Assistant itself is down: the adapter reports degraded, marks the
// subscribed entities stale once and leaves the retries to the websocket client's backoff.
func TestHAPollingFailureReturnsToWebsocketBackoff(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	ha.BlockWebsocket(true)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusPolling, ReasonWebsocketUnavailable) }, "polling")
	ha.Close()
	sourcetest.WaitFor(t, 2*time.Second, func() bool {
		return sink.lastStatus().State == homedash.StatusDegraded && sink.sawStale("switch.coffee")
	}, "degraded and stale after the poll failed")
	if n := sink.staleCount("switch.coffee"); n != 1 {
		t.Fatalf("stale updates = %d, want 1", n)
	}
}

// Subscribing while the source is already stale marks the new entity as well.
func TestHAStaleCoversLateSubscriptions(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	ha.Close()
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStale("switch.coffee") }, "stale")
	a.SetInterest([]string{"switch.coffee", "light.kitchen"})
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStale("light.kitchen") }, "late subscription marked stale")
	if n := sink.staleCount("switch.coffee"); n != 1 {
		t.Fatalf("switch.coffee stale updates = %d, want 1", n)
	}
}
