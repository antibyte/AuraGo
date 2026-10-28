package dockerwatch

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
)

func TestAttachGivesRandomIDsAndStopsAtTheClientLimit(t *testing.T) {
	env := newTestEnv(t, nil)
	reID := regexp.MustCompile(`^[0-9a-f]{32}$`)
	var clients []*Client
	for i := 0; i < MaxClients; i++ {
		c, err := env.hub.Attach()
		if err != nil {
			t.Fatalf("Attach %d: %v", i, err)
		}
		if !reID.MatchString(c.ID()) {
			t.Fatalf("ID = %q, want 32 lowercase hex characters", c.ID())
		}
		if found, ok := env.hub.Client(c.ID()); !ok || found != c {
			t.Fatalf("Client(%q) = %v, %v", c.ID(), found, ok)
		}
		clients = append(clients, c)
	}
	if _, err := env.hub.Attach(); !errors.Is(err, ErrTooManyClients) {
		t.Fatalf("Attach #%d = %v, want ErrTooManyClients", MaxClients+1, err)
	}
	clients[0].Close()
	clients[0].Close() // idempotent
	if _, ok := env.hub.Client(clients[0].ID()); ok {
		t.Fatal("a closed client is still found")
	}
	if _, open := <-clients[0].Messages(); open {
		t.Fatal("a closed client's stream is still open")
	}
	if _, err := env.hub.Attach(); err != nil {
		t.Fatalf("Attach after a Close: %v", err)
	}
}

func TestSetChecksTheLimitsAndNormalisesIDs(t *testing.T) {
	env := newTestEnv(t, nil)
	c, err := env.hub.Attach()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	long := strings.Repeat("AbC123", 10) + "abcd" // 64 characters, mixed case
	accepted, err := c.Set(Subscription{
		Hosts:     []string{"h1", "h1"},
		Stats:     map[string][]string{"h1": {long, long[:12], "0123456789ab"}},
		HostStats: []string{"h1"},
	})
	if err != nil || accepted != (Accepted{Hosts: 1, Stats: 2, HostStats: 1}) {
		t.Fatalf("Set = %+v, %v, want 1 host, 2 distinct 12-char ids, 1 gauge host", accepted, err)
	}
	env.hub.mu.Lock()
	got := c.sub.Stats["h1"]
	env.hub.mu.Unlock()
	if fmt.Sprint(got) != "[0123456789ab abc123abc123]" {
		t.Fatalf("stored ids = %v, want the lowercase 12-character prefixes, sorted", got)
	}

	ids := make([]string, MaxStatsIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("%012x", i+1)
	}
	hosts := make([]string, MaxSubscribedHosts+1)
	for i := range hosts {
		hosts[i] = fmt.Sprintf("h_%d", i)
	}
	for name, tc := range map[string]struct {
		sub  Subscription
		want error
	}{
		"too many ids":    {Subscription{Stats: map[string][]string{"h1": ids}}, ErrTooManyIDs},
		"too many hosts":  {Subscription{Hosts: hosts}, ErrTooManyHosts},
		"too many gauges": {Subscription{HostStats: hosts}, ErrTooManyHosts},
		"bad id":          {Subscription{Stats: map[string][]string{"h1": {"web"}}}, ErrInvalidSubscription},
		"bad host":        {Subscription{Hosts: []string{"../x"}}, ErrInvalidSubscription},
	} {
		if _, err := c.Set(tc.sub); !errors.Is(err, tc.want) || !errors.Is(err, ErrInvalidSubscription) {
			t.Errorf("%s: err = %v, want %v (and ErrInvalidSubscription)", name, err, tc.want)
		}
	}
	if _, err := c.Set(Subscription{Stats: map[string][]string{"h1": ids[:MaxStatsIDs]}}); err != nil {
		t.Fatalf("exactly %d ids per host must be accepted: %v", MaxStatsIDs, err)
	}
	// A refused Set changes nothing: the last good subscription is still in force.
	env.hub.mu.Lock()
	kept := len(c.sub.Stats["h1"])
	env.hub.mu.Unlock()
	if kept != MaxStatsIDs {
		t.Fatalf("the subscription holds %d ids after refused calls, want %d", kept, MaxStatsIDs)
	}
}

func TestPublishReachesOnlyClientsOfTheHost(t *testing.T) {
	env := newTestEnv(t, nil)
	a := env.attach(t, Subscription{Hosts: []string{"h1"}})
	b := env.attach(t, Subscription{})
	env.hub.Publish("h1", Message{Event: MsgOperation, Data: "op-1"})
	env.hub.Publish("other", Message{Event: MsgOperation, Data: "op-2"})

	m := waitFor(t, a, "the operation message", func(m Message) bool { return m.Event == MsgOperation })
	if m.Data != "op-1" {
		t.Fatalf("operation = %v, want op-1", m.Data)
	}
	select {
	case m := <-b.Messages():
		t.Fatalf("a client without the host received %+v", m)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestSlowClientIsSentDroppedAndClosed(t *testing.T) {
	env := newTestEnv(t, nil)
	slow := env.attach(t, Subscription{Hosts: []string{"h1"}})
	for i := 0; i < ClientBuffer+10; i++ {
		env.hub.Publish("h1", Message{Event: MsgOperation, Data: i})
	}
	var last Message
	n := 0
	for m := range slow.Messages() { // the channel is closed after the notice
		last = m
		n++
	}
	if last.Event != MsgDropped || last.Data != (Dropped{Reason: "slow_client"}) {
		t.Fatalf("last message = %+v, want dropped/slow_client", last)
	}
	if n > ClientBuffer {
		t.Fatalf("read %d messages, want at most the buffer of %d", n, ClientBuffer)
	}
	waitUntil(t, "the dropped client to be removed", func() bool {
		_, ok := env.hub.Client(slow.ID())
		return !ok
	})
	if _, err := slow.Set(Subscription{}); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("Set on a dropped client = %v, want ErrClientClosed", err)
	}
}

func TestAcquireStopsAtTheStreamBudgetPerHost(t *testing.T) {
	env := newTestEnv(t, nil)
	var releases []func()
	for i := 0; i < MaxStreamsPerHost; i++ {
		release, err := env.hub.Acquire("h1")
		if err != nil {
			t.Fatalf("Acquire %d: %v", i, err)
		}
		releases = append(releases, release)
	}
	if _, err := env.hub.Acquire("h1"); !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("Acquire #%d = %v, want ErrTooManyStreams", MaxStreamsPerHost+1, err)
	}
	if release, err := env.hub.Acquire("h2"); err != nil {
		t.Fatalf("another host has its own budget: %v", err)
	} else {
		release()
	}
	releases[0]()
	releases[0]() // a second call must not free a second slot
	again, err := env.hub.Acquire("h1")
	if err != nil {
		t.Fatalf("Acquire after a release: %v", err)
	}
	if _, err := env.hub.Acquire("h1"); !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("a double release freed two slots: %v", err)
	}
	again()
}

func TestDiskUsageIsCachedForFiveMinutesAndRefreshedAfterOneMinute(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) {
		fe.df = `{"LayersSize":1000,"Images":[{"Size":1000,"SharedSize":0,"Containers":1}],"Containers":null,"Volumes":null,"BuildCache":null}`
	})
	ctx := context.Background()
	read := func(refresh bool) (dockerhost.DiskUsage, time.Time, bool) {
		t.Helper()
		usage, at, cached, err := env.hub.DiskUsage(ctx, "h1", refresh)
		if err != nil {
			t.Fatalf("DiskUsage: %v", err)
		}
		return usage, at, cached
	}
	calls := func() int {
		env.fe.mu.Lock()
		defer env.fe.mu.Unlock()
		return env.fe.dfCalls
	}

	usage, firstAt, cached := read(false)
	if cached || usage.Total != 1000 || usage.Images.Size != 1000 || calls() != 1 {
		t.Fatalf("first read: usage %+v cached %v calls %d, want a fresh reading", usage, cached, calls())
	}
	env.clock.Advance(30 * time.Second)
	if _, at, cached := read(false); !cached || !at.Equal(firstAt) || calls() != 1 {
		t.Fatalf("second read: cached %v at %v calls %d, want the cached reading", cached, at, calls())
	}
	if _, _, cached := read(true); !cached || calls() != 1 {
		t.Fatalf("refresh after 30 s: cached %v calls %d, want the cache (younger than 60 s)", cached, calls())
	}
	env.clock.Advance(40 * time.Second)
	if _, at, cached := read(true); cached || !at.After(firstAt) || calls() != 2 {
		t.Fatalf("refresh after 70 s: cached %v calls %d, want a new reading", cached, calls())
	}
	env.clock.Advance(4 * time.Minute)
	if _, _, cached := read(false); !cached || calls() != 2 {
		t.Fatalf("read after 4 min: cached %v calls %d, want the cache", cached, calls())
	}
	env.clock.Advance(2 * time.Minute)
	if _, _, cached := read(false); cached || calls() != 3 {
		t.Fatalf("read after 6 min: cached %v calls %d, want a new reading (cache older than 5 min)", cached, calls())
	}

	env.fe.set(func(fe *fakeEngine) { fe.df = `not json` })
	env.clock.Advance(2 * time.Minute)
	if _, _, _, err := env.hub.DiskUsage(ctx, "h1", true); err == nil {
		t.Fatal("DiskUsage hid an Engine failure")
	}
	if _, _, _, err := env.hub.DiskUsage(ctx, "nope", false); !errors.Is(err, dockerhost.ErrHostNotFound) {
		t.Fatalf("unknown host: err = %v, want ErrHostNotFound", err)
	}
}
