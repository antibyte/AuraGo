package homedash_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func session(key string) homedash.Principal {
	return homedash.Principal{Kind: homedash.PrincipalSession, Key: key}
}

func newTestHub(t *testing.T, mutate func(*homedash.RuntimeConfig), sources ...*sourcetest.FakeSource) *homedash.Hub {
	t.Helper()
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: 60 * time.Millisecond, CatalogTTL: time.Second, Backoff: []time.Duration{10 * time.Millisecond}})
	t.Cleanup(h.Close)
	cfg := homedash.RuntimeConfig{Enabled: true, KioskEnabled: true, KioskAllowControl: true, Sources: map[homedash.SourceID]homedash.SourceSettings{}}
	for _, f := range sources {
		f := f
		h.Register(f.ID(), func(homedash.RuntimeConfig) (homedash.Source, bool) { return f, true })
		cfg.Sources[f.ID()] = homedash.SourceSettings{Enabled: true, Control: true, ConfigKey: string(f.ID())}
	}
	if mutate != nil {
		mutate(&cfg)
	}
	h.Reconfigure(cfg)
	sourcetest.WaitFor(t, time.Second, func() bool {
		for _, st := range h.Sources() {
			if st.State == homedash.StatusDisabled {
				return false
			}
		}
		return true
	}, "sources configured")
	return h
}

func waitClosed(t *testing.T, s *homedash.Subscriber) string {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if b := s.Drain(); b.Closed != "" {
			return b.Closed
		}
		select {
		case <-s.Ready():
		case <-deadline:
			t.Fatal("stream not closed")
		}
	}
}

func ids(list ...string) []homedash.EntityID {
	out := make([]homedash.EntityID, len(list))
	for i, s := range list {
		out[i] = homedash.EntityID(s)
	}
	return out
}

func TestHubStartsSourceOnInterestAndStopsAfterIdle(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	if fa.Runs() != 0 {
		t.Fatal("source started without interest")
	}
	sub, err := h.Attach(session("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, rej, _ := sub.Set(ids("fa:x")); len(rej) != 0 {
		t.Fatalf("rejected %v", rej)
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 && slices.Equal(fa.Interest(), []string{"x"}) }, "running with interest x")
	sub.Set(nil)
	time.Sleep(20 * time.Millisecond)
	sub.Set(ids("fa:x")) // interest returns before the idle stop
	time.Sleep(100 * time.Millisecond)
	if fa.Running() != 1 || fa.Runs() != 1 {
		t.Fatalf("idle stop not cancelled: running=%d runs=%d", fa.Running(), fa.Runs())
	}
	sub.Set(nil)
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 0 }, "idle stop")
	sourcetest.WaitFor(t, time.Second, func() bool { return h.Sources()[0].State == homedash.StatusIdle }, "idle status")
}

func TestHubCoalescesLatestStatePerEntity(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor), sourcetest.Entity("fa:y", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x", "fa:y"))
	if b := sub.Drain(); len(b.Snapshot) != 2 {
		t.Fatalf("snapshot %d", len(b.Snapshot))
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")
	for v := 1.0; v <= 3; v++ {
		fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: v, Available: true}})
	}
	fa.Push(homedash.StateUpdate{Native: "y", State: homedash.State{Value: "on", Available: true}})
	b := sub.Drain()
	if len(b.States) != 2 || b.States["fa:x"].Value != 3.0 || b.States["fa:y"].Value != "on" {
		t.Fatalf("states %+v", b.States)
	}
	if again := sub.Drain(); len(again.States) != 0 {
		t.Fatalf("states delivered twice: %+v", again.States)
	}
}

func TestHubSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	slow, _ := h.Attach(session("slow"))
	fast, _ := h.Attach(session("fast"))
	slow.Set(ids("fa:x"))
	fast.Set(ids("fa:x"))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")
	start := time.Now()
	for i := 0; i < 10000; i++ {
		fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: float64(i), Available: true}})
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("updates blocked behind a slow subscriber")
	}
	if b := fast.Drain(); b.States["fa:x"].Value != 9999.0 {
		t.Fatalf("fast: %+v", b.States)
	}
	if b := slow.Drain(); len(b.States) != 1 || b.States["fa:x"].Value != 9999.0 {
		t.Fatalf("slow kept more than the latest value: %+v", b.States)
	}
}

func TestHubReconfigureRebuildsOnlyChangedSource(t *testing.T) {
	var mu sync.Mutex
	built := map[homedash.SourceID]int{}
	count := func(id homedash.SourceID) int { mu.Lock(); defer mu.Unlock(); return built[id] }
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: time.Second})
	t.Cleanup(h.Close)
	for _, id := range []homedash.SourceID{"fa", "fb"} {
		id := id
		h.Register(id, func(homedash.RuntimeConfig) (homedash.Source, bool) {
			mu.Lock()
			built[id]++
			mu.Unlock()
			return sourcetest.New(id), true
		})
	}
	cfg := func(poll int) homedash.RuntimeConfig {
		return homedash.RuntimeConfig{Enabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{
			"fa": {Enabled: true, PollSeconds: 5}, "fb": {Enabled: true, PollSeconds: poll}}}
	}
	h.Reconfigure(cfg(10))
	sourcetest.WaitFor(t, time.Second, func() bool { return count("fa") == 1 && count("fb") == 1 }, "first build")
	h.Reconfigure(cfg(10))
	time.Sleep(50 * time.Millisecond)
	h.Reconfigure(cfg(20))
	sourcetest.WaitFor(t, time.Second, func() bool { return count("fb") == 2 }, "fb rebuilt")
	if count("fa") != 1 {
		t.Fatalf("unchanged source rebuilt %d times", count("fa"))
	}
}

func TestHubSourcePanicIsIsolatedAndRestarted(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	fa.Panics = 1
	fb := sourcetest.New("fb", sourcetest.Entity("fb:y", homedash.KindSensor))
	h := newTestHub(t, nil, fa, fb)
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x", "fb:y"))
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fa.Runs() >= 2 && fa.Running() == 1 && fb.Running() == 1 }, "restart after panic")
	if fb.Runs() != 1 {
		t.Fatalf("healthy source restarted %d times", fb.Runs())
	}
}

func TestHubCloseEndsSubscribersWithReason(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x"))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")
	h.Close()
	if reason := waitClosed(t, sub); reason != homedash.CloseShutdown {
		t.Fatalf("reason %q", reason)
	}
	if fa.Running() != 0 {
		t.Fatal("source still running")
	}
	if _, err := h.Attach(session("late")); !errors.Is(err, homedash.ErrHubClosed) {
		t.Fatalf("attach after close: %v", err)
	}
}

func TestHubDisableClosesStreams(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, nil, fa)
	sub, _ := h.Attach(session("s1"))
	h.Reconfigure(homedash.RuntimeConfig{Enabled: false})
	if reason := waitClosed(t, sub); reason != homedash.CloseDisabled {
		t.Fatalf("reason %q", reason)
	}
	var he *homedash.Error
	if _, err := h.Attach(session("s2")); !errors.As(err, &he) || he.Code != homedash.CodeHomeDashDisabled {
		t.Fatalf("attach while disabled: %v", err)
	}
}

func TestHubSetRejections(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.Limits.MaxSubscriptions = 2 }, fa)
	kiosk := homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd_1", DeviceID: "kd_1",
		Allowed: &homedash.AllowedSet{Commands: map[homedash.EntityID]map[homedash.Capability]bool{"fa:x": {}}}}
	for _, tc := range []struct {
		p        homedash.Principal
		in       []homedash.EntityID
		accepted []homedash.EntityID
		rejected []homedash.Rejection
	}{
		{session("s1"), ids("fa:x", "bad id", "zz:x", "fa:y", "fa:z"), ids("fa:x", "fa:y"),
			[]homedash.Rejection{{ID: "bad id", Reason: homedash.RejectInvalidID}, {ID: "zz:x", Reason: homedash.RejectUnknownSource}, {ID: "fa:z", Reason: homedash.RejectLimit}}},
		{kiosk, ids("fa:x", "fa:y"), ids("fa:x"), []homedash.Rejection{{ID: "fa:y", Reason: homedash.RejectForbidden}}},
	} {
		sub, err := h.Attach(tc.p)
		if err != nil {
			t.Fatal(err)
		}
		acc, rej, err := sub.Set(tc.in)
		if err != nil || !slices.Equal(acc, tc.accepted) || !slices.Equal(rej, tc.rejected) {
			t.Errorf("%s: accepted %v rejected %v err %v", tc.p.Kind, acc, rej, err)
		}
	}
}

// lampCaps are three command capabilities, so gate tests check every one of them.
func lampCaps() []homedash.CapabilitySpec {
	return []homedash.CapabilitySpec{
		{Name: homedash.CapToggle, Allowed: true},
		{Name: homedash.CapTurnOn, Allowed: true},
		{Name: homedash.CapTurnOff, Allowed: true},
	}
}

// A source whose control switch is off (or missing) shows its command capabilities as blocked; the
// blocked_by value names the config key when there is one and the source id otherwise.
func TestHubControlGateFailsClosed(t *testing.T) {
	fc := sourcetest.New("fc", sourcetest.Entity("fc:lamp", homedash.KindLight, lampCaps()...))
	fd := sourcetest.New("fd", sourcetest.Entity("fd:box", homedash.KindSwitch, lampCaps()...))
	fe := sourcetest.New("fe", sourcetest.Entity("fe:plug", homedash.KindSwitch, lampCaps()...))
	h := newTestHub(t, func(c *homedash.RuntimeConfig) {
		c.Sources["fc"] = homedash.SourceSettings{Enabled: true, Control: false}
		c.Sources["fd"] = homedash.SourceSettings{Enabled: true, Control: false, ConfigKey: "docker"}
		c.Sources["fe"] = homedash.SourceSettings{Enabled: true, Control: true, ConfigKey: "plugs"}
	}, fc, fd, fe)
	items, total, _ := h.Entities(context.Background(), homedash.Filter{})
	if total != 3 {
		t.Fatalf("total %d", total)
	}
	want := map[homedash.EntityID]string{"fc:lamp": "homedash.sources.fc", "fd:box": "homedash.sources.docker.control", "fe:plug": ""}
	for _, e := range items {
		w := want[e.ID]
		if len(e.Capabilities) != 3 || e.ReadOnly != (w != "") {
			t.Errorf("%s: caps %d readonly=%v", e.ID, len(e.Capabilities), e.ReadOnly)
		}
		for _, c := range e.Capabilities {
			if c.BlockedBy != w || c.Allowed != (w == "") {
				t.Errorf("%s/%s: allowed=%v blocked_by=%q", e.ID, c.Name, c.Allowed, c.BlockedBy)
			}
		}
	}
}

// waitEntityPush drains the stream until it carries the entity and returns it.
func waitEntityPush(t *testing.T, s *homedash.Subscriber, id homedash.EntityID) homedash.Entity {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		b := s.Drain()
		for _, e := range append(b.Snapshot, b.Entities...) {
			if e.ID == id {
				return e
			}
		}
		select {
		case <-s.Ready():
		case <-deadline:
			t.Fatalf("no push for %s", id)
		}
	}
}

func requireBlocked(t *testing.T, e homedash.Entity, blockedBy string) {
	t.Helper()
	if len(e.Capabilities) != 3 || !e.ReadOnly {
		t.Fatalf("%s: caps %d readonly=%v", e.ID, len(e.Capabilities), e.ReadOnly)
	}
	for _, c := range e.Capabilities {
		if c.Allowed || (blockedBy != "" && c.BlockedBy != blockedBy) {
			t.Fatalf("%s/%s: allowed=%v blocked_by=%q", e.ID, c.Name, c.Allowed, c.BlockedBy)
		}
	}
}

// Gate changes re-send subscribed entities with the corrected capabilities.
func TestHubGateChangeResendsEntities(t *testing.T) {
	fe := sourcetest.New("fe", sourcetest.Entity("fe:plug", homedash.KindSwitch, lampCaps()...))
	h := newTestHub(t, nil, fe)
	h.Entities(context.Background(), homedash.Filter{}) // load the catalog
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fe:plug"))
	if b := sub.Drain(); len(b.Snapshot) != 1 || b.Snapshot[0].ReadOnly {
		t.Fatalf("initial snapshot %+v", b.Snapshot)
	}
	cfg := func(mutate func(*homedash.RuntimeConfig)) homedash.RuntimeConfig {
		c := homedash.RuntimeConfig{Enabled: true, KioskEnabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{
			"fe": {Enabled: true, Control: true, ConfigKey: "fe"}}}
		mutate(&c)
		return c
	}
	// A global switch changes no source settings: the catalog stays and the entity is re-sent.
	h.Reconfigure(cfg(func(c *homedash.RuntimeConfig) { c.ReadOnly = true }))
	requireBlocked(t, waitEntityPush(t, sub, "fe:plug"), "homedash.readonly")
	h.Reconfigure(cfg(func(*homedash.RuntimeConfig) {}))
	if e := waitEntityPush(t, sub, "fe:plug"); e.ReadOnly {
		t.Fatalf("gate not lifted: %+v", e.Capabilities)
	}
	// The per-source control switch rebuilds the source; the fresh catalog arrives with the new gate.
	h.Reconfigure(cfg(func(c *homedash.RuntimeConfig) {
		c.Sources["fe"] = homedash.SourceSettings{Enabled: true, Control: false, ConfigKey: "fe"}
	}))
	requireBlocked(t, waitEntityPush(t, sub, "fe:plug"), "homedash.sources.fe.control")
	// A vanished settings entry disables the source, so no command can pass either.
	h.Reconfigure(cfg(func(c *homedash.RuntimeConfig) { delete(c.Sources, "fe") }))
	sourcetest.WaitFor(t, time.Second, func() bool { return h.Sources()[0].State == homedash.StatusDisabled }, "source disabled")
	if _, err := h.Entity(context.Background(), "fe:plug"); !errors.Is(err, homedash.ErrEntityNotFound) {
		t.Fatalf("entity of a removed source: %v", err)
	}
}

func TestHubRestartWaitsForPreviousRun(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	fa.ExitDelay = 150 * time.Millisecond
	h := newTestHub(t, nil, fa) // idle stop after 60 ms
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x"))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "first run")
	sub.Set(nil)
	time.Sleep(100 * time.Millisecond) // idle stop fired, the old run is still exiting
	if fa.Running() != 1 {
		t.Fatalf("old run already gone: running=%d", fa.Running())
	}
	sub.Set(ids("fa:x"))
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fa.Runs() == 2 && fa.Running() == 1 }, "second run")
	if m := fa.MaxRunning(); m != 1 {
		t.Fatalf("runs overlapped: max concurrent %d", m)
	}
}

// newPlainHub builds a hub with one factory per id; the factory sees the live settings.
func newPlainHub(t *testing.T, opts homedash.Options, factory func(homedash.RuntimeConfig) homedash.Source, cfg homedash.RuntimeConfig, id homedash.SourceID) *homedash.Hub {
	t.Helper()
	h := homedash.NewHub(context.Background(), opts)
	t.Cleanup(h.Close)
	h.Register(id, func(c homedash.RuntimeConfig) (homedash.Source, bool) { return factory(c), true })
	h.Reconfigure(cfg)
	sourcetest.WaitFor(t, time.Second, func() bool { return h.Sources()[0].State != homedash.StatusDisabled }, "configured")
	return h
}

func oneSource(id homedash.SourceID, set homedash.SourceSettings) homedash.RuntimeConfig {
	return homedash.RuntimeConfig{Enabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{id: set}}
}

func TestHubRebuildDropsOldCatalog(t *testing.T) {
	oldSrc := sourcetest.New("fa", sourcetest.Entity("fa:a", homedash.KindSensor))
	newSrc := sourcetest.New("fa", sourcetest.Entity("fa:b", homedash.KindSensor))
	factory := func(c homedash.RuntimeConfig) homedash.Source {
		if c.Sources["fa"].Params["v"] == "2" {
			return newSrc
		}
		return oldSrc
	}
	h := newPlainHub(t, homedash.Options{IdleStop: time.Second, CatalogTTL: time.Minute}, factory,
		oneSource("fa", homedash.SourceSettings{Enabled: true}), "fa")
	if _, err := h.Entity(context.Background(), "fa:a"); err != nil {
		t.Fatalf("old entity: %v", err)
	}
	holder, _ := h.Attach(session("holder"))
	holder.Set(ids("fa:a"))
	holder.Drain()
	h.Reconfigure(oneSource("fa", homedash.SourceSettings{Enabled: true, Params: map[string]string{"v": "2"}}))
	removed := false
	for deadline := time.Now().Add(2 * time.Second); !removed && time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		removed = slices.Contains(holder.Drain().Removed, "fa:a")
	}
	if !removed {
		t.Fatal("subscriber of the vanished entity got no removal")
	}
	sourcetest.WaitFor(t, time.Second, func() bool {
		_, err := h.Entity(context.Background(), "fa:b")
		return err == nil
	}, "rebuilt catalog")
	if _, err := h.Entity(context.Background(), "fa:a"); !errors.Is(err, homedash.ErrEntityNotFound) {
		t.Fatalf("old entity survived the rebuild: %v", err)
	}
	items, _, _ := h.Entities(context.Background(), homedash.Filter{})
	if len(items) != 1 || items[0].ID != "fa:b" {
		t.Fatalf("entities %+v", items)
	}
	late, _ := h.Attach(session("late"))
	late.Set(ids("fa:b"))
	if b := late.Drain(); len(b.Snapshot) != 1 || b.Snapshot[0].ID != "fa:b" || !b.Snapshot[0].State.Available {
		t.Fatalf("late subscriber snapshot %+v", b.Snapshot)
	}
}

// The older of two overlapping catalog fetches must not overwrite the newer result.
func TestHubCatalogAppliesInRequestOrder(t *testing.T) {
	fa := sourcetest.New("fa")
	fa.CatalogScript = []sourcetest.CatalogStep{
		{Delay: 250 * time.Millisecond, Entities: []homedash.Entity{sourcetest.Entity("fa:old", homedash.KindSensor)}},
		{Entities: []homedash.Entity{sourcetest.Entity("fa:new", homedash.KindSensor)}},
	}
	h := newPlainHub(t, homedash.Options{IdleStop: time.Second, CatalogTTL: time.Minute},
		func(homedash.RuntimeConfig) homedash.Source { return fa }, oneSource("fa", homedash.SourceSettings{Enabled: true}), "fa")
	first := make(chan struct{})
	go func() {
		h.Entities(context.Background(), homedash.Filter{})
		close(first)
	}()
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.CatalogCalls() == 1 }, "first fetch started")
	items, _, _ := h.Entities(context.Background(), homedash.Filter{})
	if len(items) != 1 || items[0].ID != "fa:new" {
		t.Fatalf("second fetch %+v", items)
	}
	<-first
	items, _, _ = h.Entities(context.Background(), homedash.Filter{})
	if len(items) != 1 || items[0].ID != "fa:new" {
		t.Fatalf("slow older fetch overwrote the newer catalog: %+v", items)
	}
}

func TestHubCatalogTimeoutDoesNotBlock(t *testing.T) {
	slow := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	slow.CatalogDelay = 1500 * time.Millisecond
	ok := sourcetest.New("fb", sourcetest.Entity("fb:y", homedash.KindSensor))
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: time.Second, CatalogTTL: time.Minute, CatalogTimeout: 80 * time.Millisecond})
	t.Cleanup(h.Close)
	for _, f := range []*sourcetest.FakeSource{slow, ok} {
		f := f
		h.Register(f.ID(), func(homedash.RuntimeConfig) (homedash.Source, bool) { return f, true })
	}
	h.Reconfigure(homedash.RuntimeConfig{Enabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{
		"fa": {Enabled: true}, "fb": {Enabled: true}}})
	sourcetest.WaitFor(t, time.Second, func() bool {
		return h.Sources()[0].State != homedash.StatusDisabled && h.Sources()[1].State != homedash.StatusDisabled
	}, "configured")
	start := time.Now()
	items, _, sources := h.Entities(context.Background(), homedash.Filter{})
	if d := time.Since(start); d > 800*time.Millisecond {
		t.Fatalf("Entities waited %v for a source ignoring its context", d)
	}
	if len(items) != 1 || items[0].ID != "fb:y" {
		t.Fatalf("items %+v", items)
	}
	if sources[0].State != homedash.StatusUnavailable || sources[0].Reason != homedash.ReasonCatalogFailed {
		t.Fatalf("slow source status %+v", sources[0])
	}
}

func TestHubStateCacheIsBounded(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	h := newPlainHub(t, homedash.Options{IdleStop: time.Second, CatalogTTL: 40 * time.Millisecond},
		func(homedash.RuntimeConfig) homedash.Source { return fa }, oneSource("fa", homedash.SourceSettings{Enabled: true}), "fa")
	future := time.Now().Add(time.Hour)
	stale := func() { time.Sleep(60 * time.Millisecond) } // let the catalog TTL expire
	value := func(id homedash.EntityID) any {
		t.Helper()
		items, _, _ := h.Entities(context.Background(), homedash.Filter{})
		for _, e := range items {
			if e.ID == id {
				return e.State.Value
			}
		}
		t.Fatalf("%s not listed", id)
		return nil
	}
	sub, _ := h.Attach(session("s1"))
	sub.Set(ids("fa:x"))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running")

	// (a) an update for an unknown native is not kept
	fa.Push(homedash.StateUpdate{Native: "ghost", State: homedash.State{Value: 99.0, Available: true, UpdatedAt: future}})
	fa.SetCatalog(sourcetest.Entity("fa:x", homedash.KindSensor), sourcetest.Entity("fa:ghost", homedash.KindSensor))
	stale()
	if v := value("fa:ghost"); v != nil {
		t.Fatalf("stale state of an unknown entity surfaced: %v", v)
	}

	// (b) an entity leaving the catalog loses its cached state
	fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: 5.0, Available: true, UpdatedAt: future}})
	sub.Set(nil)
	fa.SetCatalog(sourcetest.Entity("fa:ghost", homedash.KindSensor))
	stale()
	value("fa:ghost")
	fa.SetCatalog(sourcetest.Entity("fa:x", homedash.KindSensor), sourcetest.Entity("fa:ghost", homedash.KindSensor))
	stale()
	if v := value("fa:x"); v != nil {
		t.Fatalf("state of a removed entity survived: %v", v)
	}

	// (c) an older update does not overwrite a newer one
	sub.Set(ids("fa:x"))
	sub.Drain()
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.Running() == 1 }, "running again")
	fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: 7.0, Available: true, UpdatedAt: future.Add(time.Minute)}})
	fa.Push(homedash.StateUpdate{Native: "x", State: homedash.State{Value: 6.0, Available: true, UpdatedAt: future}})
	if b := sub.Drain(); b.States["fa:x"].Value != 7.0 {
		t.Fatalf("older update won: %+v", b.States)
	}
}

func TestHubTooManyStreams(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.Limits.MaxStreams, c.Limits.MaxKioskStreams = 2, 1 }, fa)
	kiosk := func(key string) homedash.Principal {
		return homedash.Principal{Kind: homedash.PrincipalKiosk, Key: key, DeviceID: key}
	}
	for _, tc := range []struct {
		name string
		p    func(string) homedash.Principal
		n    int
	}{{"desktop", session, 2}, {"kiosk", kiosk, 1}} {
		var subs []*homedash.Subscriber
		for i := 0; i < tc.n; i++ {
			s, err := h.Attach(tc.p(tc.name))
			if err != nil {
				t.Fatalf("%s attach %d: %v", tc.name, i, err)
			}
			subs = append(subs, s)
		}
		if _, err := h.Attach(tc.p(tc.name)); !errors.Is(err, homedash.ErrTooManyStreams) {
			t.Fatalf("%s: over the limit: %v", tc.name, err)
		}
		subs[0].Close()
		s, err := h.Attach(tc.p(tc.name))
		if err != nil {
			t.Fatalf("%s: slot not freed: %v", tc.name, err)
		}
		s.Close()
	}
}

func TestHubRevokeDeviceEndsOnlyThatDevice(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, nil, fa)
	attach := func(dev string) *homedash.Subscriber {
		s, err := h.Attach(homedash.Principal{Kind: homedash.PrincipalKiosk, Key: dev, DeviceID: dev})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	one, two := attach("kd_1"), attach("kd_2")
	h.RevokeDevice("kd_1", homedash.CloseDeviceRevoked)
	if reason := waitClosed(t, one); reason != homedash.CloseDeviceRevoked {
		t.Fatalf("reason %q", reason)
	}
	time.Sleep(30 * time.Millisecond)
	if b := two.Drain(); b.Closed != "" {
		t.Fatalf("other device closed with %q", b.Closed)
	}
	if _, ok := h.Subscriber(two.ID(), homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd_2", DeviceID: "kd_2"}); !ok {
		t.Fatal("other device lost its stream")
	}
}

// A kiosk principal without an allowed set reads nothing; agents have no stream at all.
func TestHubKioskNilAllowedGetsNothing(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor), sourcetest.Entity("fa:y", homedash.KindSensor))
	h := newTestHub(t, nil, fa)
	sub, err := h.Attach(homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd_1", DeviceID: "kd_1"})
	if err != nil {
		t.Fatal(err)
	}
	acc, rej, err := sub.Set(ids("fa:x", "fa:y"))
	if err != nil || len(acc) != 0 || len(rej) != 2 {
		t.Fatalf("accepted %v rejected %v err %v", acc, rej, err)
	}
	for _, r := range rej {
		if r.Reason != homedash.RejectForbidden {
			t.Errorf("%s: reason %q", r.ID, r.Reason)
		}
	}
	if b := sub.Drain(); len(b.Snapshot)+len(b.Entities)+len(b.States) != 0 {
		t.Fatalf("kiosk received data: %+v", b)
	}
	// Existing behaviour: Attach refuses an agent principal with forbidden_entity.
	var he *homedash.Error
	if _, err := h.Attach(homedash.Principal{Kind: homedash.PrincipalAgent, Key: "agent"}); !errors.As(err, &he) || he.Code != homedash.CodeForbiddenEntity {
		t.Fatalf("agent attach: %v", err)
	}
}
