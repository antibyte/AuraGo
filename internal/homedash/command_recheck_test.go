package homedash_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

// outcome is the result of one Command call, collected from a goroutine.
type outcome struct {
	res homedash.CommandResult
	err error
}

func codeOf(err error) string {
	var he *homedash.Error
	if errors.As(err, &he) {
		return he.Code
	}
	if err != nil {
		return "untyped"
	}
	return ""
}

func background(h *homedash.Hub, ctx context.Context, c homedash.Command) chan outcome {
	out := make(chan outcome, 1)
	go func() {
		res, err := h.Command(ctx, c)
		out <- outcome{res, err}
	}()
	return out
}

func recv(t *testing.T, ch chan outcome) outcome {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(3 * time.Second):
		t.Fatal("command did not return")
		return outcome{}
	}
}

func waitReadOnly(t *testing.T, h *homedash.Hub, what string) {
	t.Helper()
	sourcetest.WaitFor(t, time.Second, func() bool {
		e, err := h.Entity(context.Background(), "fa:light")
		return err == nil && e.ReadOnly
	}, what)
}

func TestCommandParkedBehindSlowOneIsRefusedWhenReadOnlyArrives(t *testing.T) {
	fa := gateSource()
	fa.CommandGate = make(chan struct{})
	h := newTestHub(t, nil, fa)
	queued := make(chan struct{}, 4)
	homedash.SetQueuedHookForTest(h, func() { queued <- struct{}{} })
	ctx := context.Background()
	bright := func(v float64) homedash.Command {
		return cmd("fa:light", homedash.CapBrightness, map[string]any{"value": v}, session("s"))
	}
	a := background(h, ctx, bright(10))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.InCommand() == 1 }, "first command in flight")
	b := background(h, ctx, bright(20))
	select {
	case <-queued:
	case <-time.After(2 * time.Second):
		t.Fatal("command was not queued")
	}
	h.Reconfigure(homedash.RuntimeConfig{Enabled: true, ReadOnly: true, KioskEnabled: true, KioskAllowControl: true,
		Sources: map[homedash.SourceID]homedash.SourceSettings{"fa": {Enabled: true, Control: true, ConfigKey: "fa"}}})
	waitReadOnly(t, h, "read-only applied")
	close(fa.CommandGate)
	if o := recv(t, a); o.err != nil {
		t.Fatalf("first: %v", o.err)
	}
	o := recv(t, b)
	var he *homedash.Error
	if !errors.As(o.err, &he) || he.Code != homedash.CodeReadOnly || he.BlockedBy != "homedash.readonly" {
		t.Fatalf("parked command: %v", o.err)
	}
	if n := len(fa.Commands()); n != 1 {
		t.Fatalf("the source received %d commands, want 1", n)
	}
}

func TestCommandWaitingForInFlightSlotIsRefusedWhenControlTurnsOff(t *testing.T) {
	fa := gateSource()
	fa.CommandGate = make(chan struct{})
	h := newTestHub(t, nil, fa)
	waiting := make(chan struct{}, 1)
	homedash.SetSemWaitHookForTest(h, func() { waiting <- struct{}{} })
	ctx := context.Background()
	var first []chan outcome
	for i := 0; i < 4; i++ {
		first = append(first, background(h, ctx, cmd("fa:light", homedash.CapToggle, nil, session("s"))))
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.InCommand() == 4 }, "four commands in flight")
	fifth := background(h, ctx, cmd("fa:light", homedash.CapToggle, nil, session("s")))
	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("fifth command did not wait for a slot")
	}
	h.Reconfigure(homedash.RuntimeConfig{Enabled: true, KioskEnabled: true, KioskAllowControl: true,
		Sources: map[homedash.SourceID]homedash.SourceSettings{"fa": {Enabled: true, Control: false, ConfigKey: "fa"}}})
	waitReadOnly(t, h, "control off applied")
	close(fa.CommandGate)
	for _, ch := range first {
		recv(t, ch)
	}
	o := recv(t, fifth)
	var he *homedash.Error
	if !errors.As(o.err, &he) || he.Code != homedash.CodeReadOnly || he.BlockedBy != "homedash.sources.fa.control" {
		t.Fatalf("waiting command: %v", o.err)
	}
	if n := len(fa.Commands()); n != 4 {
		t.Fatalf("the source received %d commands, want 4", n)
	}
}

func TestCommandKioskLearnsNothingAboutEntities(t *testing.T) {
	fa := gateSource()
	h := newTestHub(t, nil, fa)
	kiosk := homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd", Allowed: &homedash.AllowedSet{
		Commands: map[homedash.EntityID]map[homedash.Capability]bool{"fa:light": {homedash.CapToggle: true}}}}
	var first *homedash.Error
	for _, id := range []string{"fa:nope", "zz:x", "not an id", "fa:lock"} {
		_, err := h.Command(context.Background(), cmd(id, homedash.CapToggle, nil, kiosk))
		var he *homedash.Error
		if !errors.As(err, &he) || he.Code != homedash.CodeForbiddenEntity || he.BlockedBy != "" {
			t.Fatalf("%q: %v", id, err)
		}
		if first == nil {
			first = he
		} else if he.Message != first.Message {
			t.Fatalf("%q: message %q differs from %q", id, he.Message, first.Message)
		}
	}
	if n := fa.CatalogCalls(); n != 0 {
		t.Fatalf("%d catalog calls for refused kiosk commands", n)
	}
}

func TestCommandUnknownEntityWithFreshCatalogMakesNoUpstreamCall(t *testing.T) {
	fa := gateSource()
	h := newTestHub(t, nil, fa)
	ctx := context.Background()
	h.Entities(ctx, homedash.Filter{}) // fetches the catalog; it stays fresh for CatalogTTL
	before := fa.CatalogCalls()
	for i := 0; i < 20; i++ {
		_, err := h.Command(ctx, cmd("fa:nope", homedash.CapToggle, nil, session("s")))
		if codeOf(err) != homedash.CodeEntityNotFound {
			t.Fatalf("command %d: %v", i, err)
		}
	}
	if n := fa.CatalogCalls(); n != before {
		t.Fatalf("catalog calls grew from %d to %d", before, n)
	}
}

func TestCommandEntityWindowIsPerPrincipalKind(t *testing.T) {
	fa := gateSource()
	h := newTestHub(t, nil, fa)
	ctx := context.Background()
	kiosk := homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd", Allowed: &homedash.AllowedSet{
		Commands: map[homedash.EntityID]map[homedash.Capability]bool{"fa:light": {homedash.CapToggle: true}}}}
	for i := 0; i < 5; i++ {
		if _, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, kiosk)); err != nil {
			t.Fatalf("kiosk command %d: %v", i, err)
		}
	}
	if _, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, kiosk)); codeOf(err) != homedash.CodeRateLimited {
		t.Fatalf("kiosk sixth: %v", err)
	}
	if _, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, session("s"))); err != nil {
		t.Fatalf("session command after the kiosk used its window: %v", err)
	}
}

// The caller leaves after the drain took its parked command: it still gets the real outcome.
func TestCommandParkedCallerWaitsForCommandTheDrainTook(t *testing.T) {
	fa := gateSource()
	fa.CommandGate = make(chan struct{})
	started, release := make(chan struct{}, 1), make(chan struct{})
	fa.OnCommand = func(c homedash.Command) {
		if c.Args["value"] == 20.0 {
			started <- struct{}{}
			<-release
		}
	}
	h := newTestHub(t, nil, fa)
	queued := make(chan struct{}, 4)
	homedash.SetQueuedHookForTest(h, func() { queued <- struct{}{} })
	bright := func(v float64) homedash.Command {
		return cmd("fa:light", homedash.CapBrightness, map[string]any{"value": v}, session("s"))
	}
	a := background(h, context.Background(), bright(10))
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.InCommand() == 1 }, "first command in flight")
	bctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := background(h, bctx, bright(20))
	<-queued
	close(fa.CommandGate)
	recv(t, a)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the drain did not send the parked command")
	}
	cancel()
	select {
	case o := <-b:
		t.Fatalf("returned before the command finished: %v", o.err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if o := recv(t, b); o.err != nil {
		t.Fatalf("caller got %v although the command ran", o.err)
	}
}

// A first caller that leaves before the command ran must not hand its cancellation to waiters.
func TestCommandDedupeWaitersTakeOverAbandonedRequest(t *testing.T) {
	fa := gateSource()
	fa.CommandGate = make(chan struct{})
	h := newTestHub(t, nil, fa)
	waiting := make(chan struct{}, 2)
	homedash.SetDedupeWaitHookForTest(h, func() { waiting <- struct{}{} })
	mk := func() homedash.Command {
		c := cmd("fa:light", homedash.CapToggle, nil, session("s"))
		c.RequestID = "req-1"
		return c
	}
	fctx, cancel := context.WithCancel(context.Background())
	first := background(h, fctx, mk())
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.InCommand() == 1 }, "first command in flight")
	w1 := background(h, context.Background(), mk())
	w2 := background(h, context.Background(), mk())
	for i := 0; i < 2; i++ {
		select {
		case <-waiting:
		case <-time.After(2 * time.Second):
			t.Fatal("waiter did not start waiting")
		}
	}
	cancel()
	if o := recv(t, first); !errors.Is(o.err, context.Canceled) {
		t.Fatalf("first caller: %v", o.err)
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return len(fa.Commands()) == 2 }, "one waiter ran the command")
	close(fa.CommandGate)
	for i, ch := range []chan outcome{w1, w2} {
		if o := recv(t, ch); o.err != nil {
			t.Fatalf("waiter %d: %v", i, o.err)
		}
	}
	if n := len(fa.Commands()); n != 2 {
		t.Fatalf("the source received %d commands, want 2 (aborted first plus one retry)", n)
	}
}
