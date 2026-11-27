package hass

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

type lockedSink struct {
	mu       sync.Mutex
	catalogs [][]homedash.Entity
	updates  []homedash.StateUpdate
	statuses []homedash.SourceStatus
	closed   atomic.Bool // set once Run returned; any later call is a bug
	late     atomic.Int32
}

func (s *lockedSink) touch() {
	if s.closed.Load() {
		s.late.Add(1)
	}
}

func (s *lockedSink) Catalog(e []homedash.Entity) {
	s.touch()
	s.mu.Lock()
	s.catalogs = append(s.catalogs, e)
	s.mu.Unlock()
}
func (s *lockedSink) Update(u ...homedash.StateUpdate) {
	s.touch()
	s.mu.Lock()
	s.updates = append(s.updates, u...)
	s.mu.Unlock()
}
func (s *lockedSink) Status(st homedash.SourceStatus) {
	s.touch()
	s.mu.Lock()
	s.statuses = append(s.statuses, st)
	s.mu.Unlock()
}

func (s *lockedSink) sawStatus(state, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.statuses {
		if st.State == state && (reason == "" || st.Reason == reason) {
			return true
		}
	}
	return false
}

func (s *lockedSink) lastStatus() homedash.SourceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.statuses) == 0 {
		return homedash.SourceStatus{}
	}
	return s.statuses[len(s.statuses)-1]
}

func (s *lockedSink) last(native string) (homedash.State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.updates) - 1; i >= 0; i-- {
		if s.updates[i].Native == native {
			return s.updates[i].State, true
		}
	}
	return homedash.State{}, false
}

func (s *lockedSink) sawStale(native string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.updates {
		if u.Native == native && u.State.Stale {
			return true
		}
	}
	return false
}

func (s *lockedSink) staleCount(native string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, u := range s.updates {
		if u.Native == native && u.State.Stale {
			n++
		}
	}
	return n
}

func (s *lockedSink) catalogCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.catalogs)
}

func (s *lockedSink) entity(id homedash.EntityID) (homedash.Entity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.catalogs) == 0 {
		return homedash.Entity{}, false
	}
	for _, e := range s.catalogs[len(s.catalogs)-1] {
		if e.ID == id {
			return e, true
		}
	}
	return homedash.Entity{}, false
}

func newTestAdapter(t *testing.T, ha *hasstest.Server, cfg tools.HAConfig) *Adapter {
	t.Helper()
	if cfg.URL == "" {
		cfg.URL = ha.URL
	}
	if cfg.AccessToken == "" {
		cfg.AccessToken = ha.Token
	}
	return New(cfg, 5, FactoryOptions{Logger: discardLog, WS: fastWS, RetryWebsocket: 300 * time.Millisecond,
		PollOverride: 30 * time.Millisecond, FetchTimeout: time.Second})
}

// startAdapter runs the adapter and returns a stop function that cancels it, waits for Run and marks
// the sink closed (calls after that are counted as late).
func startAdapter(t *testing.T, a *Adapter, sink *lockedSink) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = a.Run(ctx, sink); close(done) }()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			select {
			case <-done:
				sink.closed.Store(true)
			case <-time.After(3 * time.Second):
				t.Error("Run did not return after cancel")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func runAdapter(t *testing.T, a *Adapter, sink *lockedSink) { t.Helper(); startAdapter(t, a, sink) }

func TestHAFactoryGates(t *testing.T) {
	f := Factory(FactoryOptions{})
	rc := func(mut func(*homedash.SourceSettings)) homedash.RuntimeConfig {
		set := homedash.SourceSettings{Enabled: true, Control: true, PollSeconds: 5, ConfigKey: "home_assistant",
			Params: map[string]string{ParamURL: "http://ha.lan:8123", ParamToken: "tok", ParamReadOnly: "false"},
			Lists:  map[string][]string{ListAllowedServices: {"light.turn_on"}}}
		if mut != nil {
			mut(&set)
		}
		return homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: set}}
	}
	src, ok := f(rc(nil))
	if !ok || src.ID() != SourceID || src.(*Adapter).ha.AllowedServices[0] != "light.turn_on" || src.(*Adapter).ha.ReadOnly {
		t.Fatalf("factory = %v %v", src, ok)
	}
	for name, mut := range map[string]func(*homedash.SourceSettings){
		"off":      func(s *homedash.SourceSettings) { s.Enabled = false },
		"no token": func(s *homedash.SourceSettings) { s.Params[ParamToken] = "" },
		"no url":   func(s *homedash.SourceSettings) { s.Params[ParamURL] = "  " },
	} {
		if _, ok := f(rc(mut)); ok {
			t.Errorf("%s: enabled", name)
		}
	}
	if _, ok := f(homedash.RuntimeConfig{}); ok {
		t.Error("missing settings entry enabled the source")
	}
	ro, _ := f(rc(func(s *homedash.SourceSettings) { s.Params[ParamReadOnly] = "true"; s.PollSeconds = 1 }))
	if a := ro.(*Adapter); !a.ha.ReadOnly || a.poll != 5*time.Second {
		t.Fatalf("read-only/poll = %+v %v", a.ha, a.poll)
	}
}

// A configured but unusable URL builds an enabled adapter that reports unavailable/invalid_url and
// never dials (amendment X2).
func TestHAFactoryBadURLReportsInvalidURL(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions()) // proves nothing dials: the URL below never points here
	f := Factory(FactoryOptions{Logger: discardLog, WS: fastWS})
	for name, u := range map[string]string{"bad scheme": "ftp://ha.lan", "credentials": "http://u:p@ha.lan", "garbage": "::not a url"} {
		set := homedash.SourceSettings{Enabled: true, PollSeconds: 5, Params: map[string]string{ParamURL: u, ParamToken: "tok"}}
		src, ok := f(homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{SourceID: set}})
		if !ok {
			t.Fatalf("%s: disabled, want enabled", name)
		}
		if _, err := src.Catalog(context.Background()); err == nil {
			t.Errorf("%s: catalog succeeded", name)
		}
		sink := &lockedSink{}
		stop := startAdapter(t, src.(*Adapter), sink)
		sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusUnavailable, ReasonInvalidURL) }, name+": invalid_url")
		time.Sleep(50 * time.Millisecond)
		stop()
		sink.mu.Lock()
		for _, st := range sink.statuses {
			if st.State != homedash.StatusUnavailable || st.Reason != ReasonInvalidURL {
				t.Errorf("%s: unexpected status %+v", name, st)
			}
		}
		n := len(sink.updates) + len(sink.catalogs)
		sink.mu.Unlock()
		if n != 0 || sink.late.Load() != 0 {
			t.Errorf("%s: sink got %d states/catalogs, %d late calls", name, n, sink.late.Load())
		}
	}
	if ha.WebsocketConnects() != 0 || ha.RESTStateRequests() != 0 {
		t.Fatalf("fake saw traffic: %d websocket, %d REST", ha.WebsocketConnects(), ha.RESTStateRequests())
	}
}

func TestHAAdapterCatalogWithoutRun(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	ctx := context.Background()
	list, err := a.Catalog(ctx)
	if err != nil || len(list) != 14 || ha.WebsocketConnects() != 1 {
		t.Fatalf("catalog = %d entities, %v, %d websocket sessions", len(list), err, ha.WebsocketConnects())
	}
	area := func(list []homedash.Entity, id homedash.EntityID) string {
		for _, e := range list {
			if e.ID == id {
				return e.AreaID
			}
		}
		return "missing"
	}
	if area(list, "ha:light.kitchen") != "kitchen" {
		t.Fatalf("kitchen area = %q", area(list, "ha:light.kitchen"))
	}
	ha.BlockWebsocket(true)
	list, err = a.Catalog(ctx)
	if err != nil || ha.RESTStateRequests() != 1 || area(list, "ha:light.kitchen") != "kitchen" {
		t.Fatalf("REST catalog keeps the last registries: %v rest=%d area=%q", err, ha.RESTStateRequests(), area(list, "ha:light.kitchen"))
	}
	b := newTestAdapter(t, ha, tools.HAConfig{})
	if list, err = b.Catalog(ctx); err != nil || area(list, "ha:light.kitchen") != "" || len(list) != 15 {
		t.Fatalf("REST catalog without registries: %v area=%q n=%d", err, area(list, "ha:light.kitchen"), len(list))
	}
}

func TestHAAdapterStreamsStatesAndReconnects(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool {
		e, ok := sink.entity("ha:light.kitchen")
		return sink.sawStatus(homedash.StatusLive, "") && ok && e.AreaID == "kitchen"
	}, "live catalog with areas")

	ha.SetState(hasstest.State{EntityID: "switch.coffee", State: "on", Attributes: map[string]any{"friendly_name": "Coffee machine", "device_class": "outlet"}}, true)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { st, ok := sink.last("switch.coffee"); return ok && st.Value == true }, "state_changed update")

	ha.DropWebsockets()
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusDegraded, "") && sink.sawStale("switch.coffee") }, "degraded and stale")
	sourcetest.WaitFor(t, 2*time.Second, func() bool {
		st, ok := sink.last("switch.coffee")
		return sink.lastStatus().State == homedash.StatusLive && ok && !st.Stale && st.Value == true
	}, "resync after reconnect")

	ha.SetEntityArea("switch.coffee", "living_room")
	sourcetest.WaitFor(t, 2*time.Second, func() bool { e, ok := sink.entity("ha:switch.coffee"); return ok && e.AreaID == "living_room" }, "catalog after a registry update")
}

// A state change alone must not push a catalog: only meta changes do (the hub forwards state frames
// from updates).
func TestHAAdapterCatalogOnlyForMetaChanges(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool {
		e, ok := sink.entity("ha:light.kitchen")
		return ok && e.AreaID == "kitchen"
	}, "catalog with areas")
	n := sink.catalogCount()
	ha.SetState(hasstest.State{EntityID: "switch.coffee", State: "on", Attributes: map[string]any{"friendly_name": "Coffee machine", "device_class": "outlet"}}, true)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { st, ok := sink.last("switch.coffee"); return ok && st.Value == true }, "update")
	if sink.catalogCount() != n {
		t.Fatalf("a state change pushed %d extra catalogs", sink.catalogCount()-n)
	}
}

// An entity that leaves Home Assistant or becomes hidden or disabled drops out of the next catalog,
// which the hub reports as removed. The fake has no deletion, so the bridge is driven directly.
func TestHAAdapterRemovesVanishedAndHiddenEntities(t *testing.T) {
	a := New(tools.HAConfig{URL: "http://ha.lan:8123", AccessToken: "tok"}, 5, FactoryOptions{Logger: discardLog})
	sink := &lockedSink{}
	a.attach(sink)
	b := &wsBridge{a: a, limit: wsFailuresBeforePolling, probe: make(chan string, 1)}
	now := time.Now().UTC()
	mk := func(id string) HAState {
		return HAState{EntityID: id, State: "off", Attributes: map[string]any{}, LastUpdated: now, LastChanged: now}
	}
	b.OnStates([]HAState{mk("switch.a"), mk("switch.b"), mk("switch.c")})
	if _, ok := sink.entity("ha:switch.b"); !ok {
		t.Fatal("switch.b missing from the first catalog")
	}
	b.OnStateChanged(StateChanged{EntityID: "switch.b", OldState: &HAState{EntityID: "switch.b"}})
	if _, ok := sink.entity("ha:switch.b"); ok {
		t.Fatal("switch.b still cataloged after it left Home Assistant")
	}
	b.OnRegistries(Registries{Entities: []EntityEntry{{EntityID: "switch.c", DisabledBy: "user"}}})
	if _, ok := sink.entity("ha:switch.c"); ok {
		t.Fatal("disabled switch.c still cataloged")
	}
	if _, ok := sink.entity("ha:switch.a"); !ok {
		t.Fatal("switch.a was dropped")
	}
	b.OnRegistries(Registries{Entities: []EntityEntry{{EntityID: "switch.a", HiddenBy: "user"}}})
	if _, ok := sink.entity("ha:switch.a"); ok {
		t.Fatal("hidden switch.a still cataloged")
	}
}

func TestHAAdapterInvalidURLMakesNoNetworkAttempt(t *testing.T) {
	a := New(tools.HAConfig{URL: "ftp://ha.lan", AccessToken: "tok"}, 5, FactoryOptions{Logger: discardLog, WS: fastWS})
	if _, err := a.Catalog(context.Background()); err == nil {
		t.Fatal("catalog of an invalid URL succeeded")
	}
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusUnavailable, ReasonInvalidURL) }, "invalid_url")
}

func TestHAAdapterAuthFailed(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{AccessToken: "wrong-token"})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusUnavailable, ReasonAuthFailed) }, "auth_failed")
	if _, err := a.Catalog(context.Background()); err == nil {
		t.Fatal("catalog with a rejected token succeeded")
	}
}

func TestHAAdapterStopsCleanly(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	stop := startAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	stop()
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return ha.OpenWebsockets() == 0 }, "websocket closed")
	// Events after the stop must not reach the sink.
	ha.SetState(hasstest.State{EntityID: "switch.coffee", State: "on"}, true)
	ha.SetEntityArea("switch.coffee", "kitchen")
	time.Sleep(100 * time.Millisecond)
	if n := sink.late.Load(); n != 0 || ha.OpenWebsockets() != 0 {
		t.Fatalf("after Run: %d late sink calls, %d open websockets", n, ha.OpenWebsockets())
	}
}

// Every Run of the same adapter starts with a catalog and full states, even when the previous run
// (or a Catalog call) left identical states behind.
func TestHAAdapterRerunResendsEverything(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	for i := 0; i < 2; i++ {
		sink := &lockedSink{}
		stop := startAdapter(t, a, sink)
		sourcetest.WaitFor(t, 2*time.Second, func() bool {
			_, ok := sink.last("switch.coffee")
			return ok && sink.catalogCount() > 0 && sink.sawStatus(homedash.StatusLive, "")
		}, "catalog and state in every run")
		stop()
	}
}

func TestHAAdapterStaleOnceWhileDegraded(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	ha.Close() // Home Assistant is gone: several failed reconnects
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStale("switch.coffee") }, "stale")
	time.Sleep(200 * time.Millisecond)
	if n := sink.staleCount("switch.coffee"); n != 1 {
		t.Fatalf("stale updates = %d, want exactly 1", n)
	}
	st := sink.lastStatus()
	if st.State != homedash.StatusDegraded {
		t.Fatalf("status = %+v", st)
	}
}
