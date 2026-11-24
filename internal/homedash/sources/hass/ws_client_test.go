package hass

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"aurago/internal/testutil"
)

type fakeConn struct {
	mu sync.Mutex
	c  *websocket.Conn
}

func (f *fakeConn) send(v any) { f.mu.Lock(); defer f.mu.Unlock(); _ = f.c.WriteJSON(v) }

type hassFake struct {
	token      string
	states     []HAState
	ignorePing atomic.Bool
	mu         sync.Mutex
	conns      []*fakeConn
	srv        *httptest.Server

	// Test hooks. delayEntities delays the next entity-registry answer once; dropAreas drops the next
	// area-registry answer once; afterAuth (set before the client starts) runs after auth_ok with the
	// 1-based connection number and, when it returns true, ends the connection.
	delayEntities atomic.Int64 // nanoseconds
	dropAreas     atomic.Bool
	entityReqs    atomic.Int32
	afterAuth     func(n int) bool
	times         []time.Time // connect times, guarded by mu
}

func newHassFake(t *testing.T, token string, states []HAState) *hassFake {
	f := &hassFake{token: token, states: states}
	up := websocket.Upgrader{}
	f.srv = testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/websocket" {
			http.NotFound(w, r)
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		conn := &fakeConn{c: c}
		f.mu.Lock()
		f.conns = append(f.conns, conn)
		f.times = append(f.times, time.Now())
		n := len(f.conns)
		f.mu.Unlock()
		conn.send(map[string]any{"type": "auth_required", "ha_version": "2026.11.0"})
		var auth struct {
			Type        string `json:"type"`
			AccessToken string `json:"access_token"`
		}
		if c.ReadJSON(&auth) != nil {
			return
		}
		if auth.AccessToken != f.token {
			conn.send(map[string]any{"type": "auth_invalid", "message": "Invalid access token"})
			return
		}
		conn.send(map[string]any{"type": "auth_ok"})
		if f.afterAuth != nil && f.afterAuth(n) {
			return
		}
		for {
			var m struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			}
			if c.ReadJSON(&m) != nil {
				return
			}
			var result any
			switch m.Type {
			case "ping":
				if !f.ignorePing.Load() {
					conn.send(map[string]any{"id": m.ID, "type": "pong"})
				}
				continue
			case "get_states":
				result = f.states
			case "config/area_registry/list":
				if f.dropAreas.CompareAndSwap(true, false) {
					continue
				}
				result = []map[string]any{{"area_id": "kitchen", "name": "Kitchen"}}
			case "config/device_registry/list":
				result = []map[string]any{{"id": "dev1", "area_id": "kitchen"}}
			case "config/entity_registry/list":
				result = []map[string]any{{"entity_id": "light.kitchen", "device_id": "dev1", "area_id": nil}}
				f.entityReqs.Add(1)
				if d := f.delayEntities.Swap(0); d > 0 {
					id, res := m.ID, result
					go func() {
						time.Sleep(time.Duration(d))
						conn.send(map[string]any{"id": id, "type": "result", "success": true, "result": res})
					}()
					continue
				}
			}
			conn.send(map[string]any{"id": m.ID, "type": "result", "success": true, "result": result})
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *hassFake) push(eventType string, data any) {
	f.mu.Lock()
	c := f.conns[len(f.conns)-1]
	f.mu.Unlock()
	c.send(map[string]any{"id": 2, "type": "event", "event": map[string]any{"event_type": eventType, "data": data}})
}

func (f *hassFake) drop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.conns {
		c.c.Close()
	}
}

func (f *hassFake) connections() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.conns) }

type recHandler struct {
	mu       sync.Mutex
	states   [][]HAState
	changes  []StateChanged
	regs     []Registries
	statuses []WSStatus
}

func (r *recHandler) OnStates(s []HAState) {
	r.mu.Lock()
	r.states = append(r.states, s)
	r.mu.Unlock()
}
func (r *recHandler) OnStateChanged(c StateChanged) {
	r.mu.Lock()
	r.changes = append(r.changes, c)
	r.mu.Unlock()
}
func (r *recHandler) OnRegistries(g Registries) {
	r.mu.Lock()
	r.regs = append(r.regs, g)
	r.mu.Unlock()
}
func (r *recHandler) OnStatus(s WSStatus) {
	r.mu.Lock()
	r.statuses = append(r.statuses, s)
	r.mu.Unlock()
}
func (r *recHandler) count() (states, changes, regs int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.states), len(r.changes), len(r.regs)
}
func (r *recHandler) sawStatus(state, reason string, failures int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.ContainsFunc(r.statuses, func(s WSStatus) bool {
		return s.State == state && (reason == "" || s.Reason == reason) && s.Failures >= failures
	})
}

var testWSOptions = WSOptions{HandshakeTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Minute,
	AuthRetry: time.Hour, RegistryDebounce: 20 * time.Millisecond, Backoff: []time.Duration{30 * time.Millisecond}, Jitter: -1}

// startClient runs a client until the test ends; cleanup cancels it and fails if Run does not return.
func startClient(t *testing.T, base, token string, o WSOptions) *recHandler {
	t.Helper()
	return startClientWith(t, base, token, o, nil)
}

// startClientWith is startClient with a hook that may set unexported timing fields before Run.
func startClientWith(t *testing.T, base, token string, o WSOptions, tweak func(*WSClient)) *recHandler {
	t.Helper()
	h := &recHandler{}
	c, err := NewWSClient(base, token, h, o)
	if err != nil {
		t.Fatal(err)
	}
	if tweak != nil {
		tweak(c)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = c.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Run did not return after context cancel")
		}
	})
	return h
}

func waitUntil(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHAWebsocketAuthGetStatesAndStateChanged(t *testing.T) {
	f := newHassFake(t, "tok", []HAState{{EntityID: "light.kitchen", State: "on"}})
	h := startClient(t, f.srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { s, _, g := h.count(); return s == 1 && g == 1 && h.sawStatus(WSLive, "", 0) }, "states, registries, live")
	h.mu.Lock()
	reg := h.regs[0]
	h.mu.Unlock()
	if len(reg.Areas) != 1 || reg.Devices[0].AreaID != "kitchen" || reg.Entities[0].DeviceID != "dev1" {
		t.Fatalf("registries %+v", reg)
	}
	f.push("state_changed", map[string]any{"entity_id": "light.kitchen", "new_state": map[string]any{"entity_id": "light.kitchen", "state": "off"}})
	waitUntil(t, func() bool { _, c, _ := h.count(); return c == 1 }, "state_changed")
	h.mu.Lock()
	change := h.changes[0]
	h.mu.Unlock()
	if change.NewState == nil || change.NewState.State != "off" {
		t.Fatalf("change %+v", change)
	}
	f.push("area_registry_updated", map[string]any{})
	f.push("entity_registry_updated", map[string]any{})
	waitUntil(t, func() bool { _, _, g := h.count(); return g == 2 }, "debounced re-list")
	time.Sleep(60 * time.Millisecond)
	if _, _, g := h.count(); g != 2 {
		t.Fatalf("re-listed %d times, want one debounced re-list", g-1)
	}
}

func TestHAWebsocketReconnectBackoffAndResync(t *testing.T) {
	f := newHassFake(t, "tok", []HAState{{EntityID: "switch.a", State: "on"}})
	h := startClient(t, f.srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { s, _, _ := h.count(); return s == 1 }, "first states")
	dropped := time.Now()
	f.drop()
	waitUntil(t, func() bool { s, _, _ := h.count(); return s == 2 }, "resync after reconnect")
	if time.Since(dropped) < 30*time.Millisecond || f.connections() != 2 || !h.sawStatus(WSDegraded, "", 0) {
		t.Fatalf("elapsed %v connections %d", time.Since(dropped), f.connections())
	}
}

func TestHAAuthInvalidStopsTightRetry(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	h := startClient(t, f.srv.URL, "wrong", testWSOptions)
	waitUntil(t, func() bool { return h.sawStatus(WSAuthFailed, ReasonAuthFailed, 0) }, "auth_failed")
	time.Sleep(150 * time.Millisecond)
	if n := f.connections(); n != 1 {
		t.Fatalf("%d connections after auth_invalid, want 1", n)
	}
}

func TestHAWebsocketMissingPongReconnects(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	f.ignorePing.Store(true)
	o := testWSOptions
	o.PingInterval, o.PongTimeout = 40*time.Millisecond, 20*time.Millisecond
	h := startClient(t, f.srv.URL, "tok", o)
	waitUntil(t, func() bool { return f.connections() >= 2 && h.sawStatus(WSDegraded, ReasonTimeout, 0) }, "reconnect after missing pong")
}

func TestHAWebsocketCountsFailedHandshakes(t *testing.T) {
	srv := testutil.NewHTTPServer(t, http.NotFoundHandler())
	defer srv.Close()
	h := startClient(t, srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { return h.sawStatus(WSDegraded, ReasonUnreachable, 3) }, "three failed handshakes")
}

func completeRegistries(g Registries) bool {
	return len(g.Areas) == 1 && len(g.Devices) == 1 && len(g.Entities) == 1
}

func TestHAWebsocketRegistryBatchesDoNotOverlap(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	f.delayEntities.Store(int64(250 * time.Millisecond)) // far beyond the 20 ms debounce
	h := startClient(t, f.srv.URL, "tok", testWSOptions)
	waitUntil(t, func() bool { return f.entityReqs.Load() == 1 }, "first entity list request")
	f.push("area_registry_updated", map[string]any{})
	waitUntil(t, func() bool { _, _, g := h.count(); return g == 2 }, "two deliveries")
	time.Sleep(150 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.regs) != 2 {
		t.Fatalf("%d deliveries, want 2 (one per completed batch)", len(h.regs))
	}
	for i, g := range h.regs {
		if !completeRegistries(g) {
			t.Fatalf("delivery %d incomplete or mixed: %+v", i, g)
		}
	}
	if n := f.entityReqs.Load(); n != 2 {
		t.Fatalf("%d batches started, want 2", n)
	}
}

func TestHAWebsocketRegistryBatchTimeoutRecovers(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	f.dropAreas.Store(true)
	h := startClientWith(t, f.srv.URL, "tok", testWSOptions, func(c *WSClient) { c.registryTimeout = 150 * time.Millisecond })
	waitUntil(t, func() bool { _, _, g := h.count(); return g == 1 }, "delivery after the timed-out batch")
	time.Sleep(300 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.regs) != 1 || !completeRegistries(h.regs[0]) {
		t.Fatalf("deliveries %+v, want one complete delivery", h.regs)
	}
	if n := f.entityReqs.Load(); n != 2 {
		t.Fatalf("%d batches started, want 2", n)
	}
}

func TestHAWebsocketBackoffGrowsWhenSessionsDropEarly(t *testing.T) {
	f := newHassFake(t, "tok", nil)
	f.afterAuth = func(n int) bool {
		if n == 4 { // outlives stableAfter, so the next delay starts over
			time.Sleep(150 * time.Millisecond)
		}
		return n <= 4
	}
	o := testWSOptions
	o.Backoff = []time.Duration{40 * time.Millisecond, 80 * time.Millisecond, 160 * time.Millisecond, 320 * time.Millisecond, 640 * time.Millisecond}
	startClientWith(t, f.srv.URL, "tok", o, func(c *WSClient) { c.stableAfter = 100 * time.Millisecond })
	waitUntil(t, func() bool { return f.connections() >= 5 }, "five connections")
	f.mu.Lock()
	ts := slices.Clone(f.times)
	f.mu.Unlock()
	g1, g3 := ts[1].Sub(ts[0]), ts[3].Sub(ts[2])
	if g3 < g1+80*time.Millisecond {
		t.Fatalf("delays did not grow: first gap %v, third gap %v", g1, g3)
	}
	// Connection 4 held on for 150 ms (stable), so the delay before 5 restarts at 40 ms, not 640 ms.
	if g4 := ts[4].Sub(ts[3]); g4 < 150*time.Millisecond || g4 > 450*time.Millisecond {
		t.Fatalf("gap after the stable session %v, want about 190 ms", g4)
	}
}

func TestWebsocketURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://ha.lan:8123":       "ws://ha.lan:8123/api/websocket",
		"https://ha.example/":      "wss://ha.example/api/websocket",
		"https://proxy.lan/hass":   "wss://proxy.lan/hass/api/websocket",
		"ftp://ha.lan":             "",
		"http://user:pw@ha.lan":    "",
		"http://ha.lan/?token=abc": "",
	} {
		got, err := WebsocketURL(in)
		if (want == "") != (err != nil) || got != want {
			t.Errorf("%s: %q %v", in, got, err)
		}
	}
}
