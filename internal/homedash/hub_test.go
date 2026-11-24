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

// A source whose control switch is off (or missing) shows its command capabilities as blocked; the
// blocked_by value names the config key when there is one and the source id otherwise.
func TestHubControlGateFailsClosed(t *testing.T) {
	toggle := homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}
	fc := sourcetest.New("fc", sourcetest.Entity("fc:lamp", homedash.KindLight, toggle))
	fd := sourcetest.New("fd", sourcetest.Entity("fd:box", homedash.KindSwitch, toggle))
	fe := sourcetest.New("fe", sourcetest.Entity("fe:plug", homedash.KindSwitch, toggle))
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
		c := e.Capabilities[0]
		w := want[e.ID]
		if c.BlockedBy != w || c.Allowed != (w == "") || e.ReadOnly != (w != "") {
			t.Errorf("%s: allowed=%v blocked_by=%q readonly=%v", e.ID, c.Allowed, c.BlockedBy, e.ReadOnly)
		}
	}
}
