package dockerwatch

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
)

func eventLine(typ, action, id string, attrs map[string]string) string {
	line := map[string]any{
		"Type": typ, "Action": action, "scope": "local", "time": 1730102400, "timeNano": int64(1730102400123456789),
		"Actor": map[string]any{"ID": id, "Attributes": attrs},
	}
	raw, _ := json.Marshal(line)
	return string(raw)
}

const webID = "aaaaaaaaaaaa1111111111111111111111111111111111111111111111111111"

func TestWatcherForwardsAndFiltersEvents(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.attach(t, Subscription{Hosts: []string{"h1"}})
	var mu sync.Mutex
	var viaCallback []Event
	cancel := env.hub.WatchHost("h1", func(ev Event) {
		mu.Lock()
		viaCallback = append(viaCallback, ev)
		mu.Unlock()
	})
	defer cancel()

	if m := next(t, c, 2*time.Second); m.Event != MsgHostStatus || m.Data.(HostStatus).State != StateConnecting {
		t.Fatalf("first message = %+v, want host_status connecting", m)
	}
	waitFor(t, c, "host_status live", isStatus(StateLive))

	labels := map[string]string{"name": "shop-web-1", "com.docker.compose.project": "shop", "com.docker.compose.service": "web", "secret": "must-not-travel"}
	env.fe.send(t,
		eventLine("container", "exec_start: sh -c ls", webID, labels), // dropped
		eventLine("container", "attach", webID, labels),               // dropped
		eventLine("container", "start", webID, labels),
		eventLine("container", "die", webID, map[string]string{"name": "shop-web-1", "exitCode": "137"}),
		eventLine("container", "health_status: unhealthy", webID, labels),
		eventLine("image", "pull", "nginx:1.27", map[string]string{"name": "nginx"}),
		eventLine("volume", "create", "shop_data", map[string]string{"driver": "local"}),
		eventLine("network", "connect", "net1", map[string]string{"name": "shop_default"}),
		eventLine("daemon", "reload", "x", nil), // not a resource type
	)
	var got []Event
	for len(got) < 6 {
		m := next(t, c, 2*time.Second)
		if m.Event == MsgEvent {
			got = append(got, m.Data.(Event))
		}
	}
	start, die, health, image, volume, network := got[0], got[1], got[2], got[3], got[4], got[5]
	if start.Action != "start" || start.Type != "container" || start.ID != webID || start.ShortID != "aaaaaaaaaaaa" ||
		start.Name != "shop-web-1" || start.Project != "shop" || start.Service != "web" || start.Host != "h1" ||
		start.T != 1730102400123 || start.ExitCode != nil || start.Health != "" {
		t.Fatalf("start = %+v", start)
	}
	if die.Action != "die" || die.ExitCode == nil || *die.ExitCode != 137 || die.Project != "" {
		t.Fatalf("die = %+v, want exit code 137 and empty project/service", die)
	}
	if health.Action != "health_status" || health.Health != "unhealthy" {
		t.Fatalf("health = %+v, want action health_status with health unhealthy", health)
	}
	if image.Type != "image" || image.Action != "pull" || image.ID != "nginx:1.27" || image.Name != "nginx" || image.ShortID != "" {
		t.Fatalf("image = %+v", image)
	}
	if volume.Type != "volume" || volume.Action != "create" || volume.ID != "shop_data" || network.Type != "network" || network.Name != "shop_default" {
		t.Fatalf("volume/network = %+v / %+v", volume, network)
	}
	raw, _ := json.Marshal(start)
	if strings.Contains(string(raw), "must-not-travel") {
		t.Fatalf("event JSON %s leaks a label value", raw)
	}

	waitUntil(t, "the callback to see all six events", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(viaCallback) == 6
	})
}

func TestWatcherReconnectsAndSendsResync(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.attach(t, Subscription{Hosts: []string{"h1"}})
	waitFor(t, c, "host_status live", isStatus(StateLive))

	env.fe.dropFeed(t)
	waitFor(t, c, "host_status connecting after the drop", isStatus(StateConnecting))
	waitFor(t, c, "host_status live again", isStatus(StateLive))
	m := waitFor(t, c, "resync", func(m Message) bool { return m.Event == MsgResync })
	if got := m.Data.(Resync); got != (Resync{Host: "h1", Reason: "reconnect"}) {
		t.Fatalf("resync = %+v, want reconnect on h1", got)
	}
	if env.fe.conns() != 2 {
		t.Fatalf("events connections = %d, want 2", env.fe.conns())
	}
	env.fe.send(t, eventLine("container", "start", webID, map[string]string{"name": "web"}))
	waitFor(t, c, "an event on the new feed", func(m Message) bool { return m.Event == MsgEvent })
}

func TestWatcherReportsAnUnreachableHostAndRecovers(t *testing.T) {
	env := newTestEnv(t, nil)
	env.hosts.setDown("h1", true)
	c := env.attach(t, Subscription{Hosts: []string{"h1"}})

	m := waitFor(t, c, "host_status unreachable", isStatus(StateUnreachable))
	if got := m.Data.(HostStatus); got.Reason != dockerhost.CodeRefused || got.Host != "h1" {
		t.Fatalf("status = %+v, want unreachable with reason refused", got)
	}
	env.hosts.setDown("h1", false)
	waitFor(t, c, "host_status live", isStatus(StateLive))
	m = waitFor(t, c, "resync", func(m Message) bool { return m.Event == MsgResync })
	if m.Data.(Resync).Reason != "reconnect" {
		t.Fatalf("resync = %+v", m.Data)
	}

	// A host the registry no longer knows says so.
	ghost := env.attach(t, Subscription{Hosts: []string{"ghost"}})
	m = waitFor(t, ghost, "host_status unreachable for a deleted host", isStatus(StateUnreachable))
	if got := m.Data.(HostStatus); got.Reason != ReasonHostNotFound {
		t.Fatalf("status = %+v, want reason %q", got, ReasonHostNotFound)
	}
}

func TestWatcherFallsBackToPollingWhenEventsAreRefused(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) {
		fe.eventsStatus = 503
		fe.containers = `[{"Id":"aaaaaaaaaaaa1111","Names":["/web"],"State":"running","Status":"Up 1 minute","Labels":{}}]`
	})
	c := env.attach(t, Subscription{Hosts: []string{"h1"}})

	waitFor(t, c, "host_status paused", isStatus(StatePaused))
	if env.fe.conns() != 3 {
		t.Fatalf("events attempts before pausing = %d, want 3", env.fe.conns())
	}
	m := waitFor(t, c, "the first poll resync", func(m Message) bool { return m.Event == MsgResync })
	if m.Data.(Resync) != (Resync{Host: "h1", Reason: "poll"}) {
		t.Fatalf("resync = %+v, want a poll resync", m.Data)
	}
	// An unchanged list sends nothing; a changed one sends a resync.
	time.Sleep(80 * time.Millisecond)
	select {
	case m := <-c.Messages():
		t.Fatalf("message %+v although the container list did not change", m)
	default:
	}
	env.fe.set(func(fe *fakeEngine) {
		fe.containers = `[{"Id":"aaaaaaaaaaaa1111","Names":["/web"],"State":"exited","Status":"Exited (1) 1 second ago","Labels":{}}]`
	})
	waitFor(t, c, "a resync after the list changed", func(m Message) bool { return m.Event == MsgResync })

	// The feed is retried while paused and takes over when the Engine allows it again.
	env.fe.set(func(fe *fakeEngine) { fe.eventsStatus = 0 })
	waitFor(t, c, "host_status live after the retry", isStatus(StateLive))
	m = waitFor(t, c, "the reconnect resync", func(m Message) bool { return m.Event == MsgResync })
	if m.Data.(Resync).Reason != "reconnect" {
		t.Fatalf("resync = %+v, want reconnect", m.Data)
	}
}

func TestWatcherPausesWhenTheStreamKeepsEnding(t *testing.T) {
	env := newTestEnv(t, nil)
	env.fe.set(func(fe *fakeEngine) { fe.eventsEnd = true })
	c := env.attach(t, Subscription{Hosts: []string{"h1"}})
	waitFor(t, c, "host_status paused after three stream ends", isStatus(StatePaused))
	if n := env.fe.conns(); n != 3 {
		t.Fatalf("events connections = %d, want 3 stream ends before pausing", n)
	}
}

func TestWatcherEndsARequestTheEngineNeverAnswers(t *testing.T) {
	env := newTestEnv(t, func(o *Options) { o.ConnectTimeout = 60 * time.Millisecond })
	env.fe.set(func(fe *fakeEngine) { fe.eventsHang = true })
	c := env.attach(t, Subscription{Hosts: []string{"h1"}})
	m := waitFor(t, c, "host_status unreachable", isStatus(StateUnreachable))
	if got := m.Data.(HostStatus); got.Reason != dockerhost.CodeTimeout {
		t.Fatalf("status = %+v, want reason timeout", got)
	}
}

func TestWatcherStopsAfterTheLastUserLeaves(t *testing.T) {
	env := newTestEnv(t, nil)
	a := env.attach(t, Subscription{Hosts: []string{"h1"}})
	waitFor(t, a, "host_status live", isStatus(StateLive))
	streams := func() int {
		env.hub.budgetMu.Lock()
		defer env.hub.budgetMu.Unlock()
		return env.hub.streams["h1"]
	}
	if streams() != 1 {
		t.Fatalf("stream slots held = %d, want 1 for the events watcher", streams())
	}

	// The next client within IdleStop shares the running watcher and is told its state.
	a.Close()
	b := env.attach(t, Subscription{Hosts: []string{"h1"}})
	if m := next(t, b, 2*time.Second); m.Event != MsgHostStatus || m.Data.(HostStatus).State != StateLive {
		t.Fatalf("first message of the second client = %+v, want the current status live", m)
	}
	time.Sleep(2 * env.hub.opts.IdleStop)
	if env.fe.conns() != 1 {
		t.Fatalf("events connections = %d, want the watcher kept for the second client", env.fe.conns())
	}

	b.Close()
	waitUntil(t, "the watcher to stop after IdleStop", func() bool { return env.hub.status("h1").State == "" })
	waitUntil(t, "the stream slot to be released", func() bool { return streams() == 0 })
	env.hub.mu.Lock()
	left := len(env.hub.rts)
	env.hub.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d host runtimes left after the last user left", left)
	}

	// A later subscriber starts a new watcher.
	d := env.attach(t, Subscription{Hosts: []string{"h1"}})
	waitFor(t, d, "host_status live", isStatus(StateLive))
	if env.fe.conns() != 2 {
		t.Fatalf("events connections = %d, want a second one", env.fe.conns())
	}
}

func TestWatchHostDeliversEventsAndResyncWithoutAClient(t *testing.T) {
	env := newTestEnv(t, nil)
	got := make(chan Event, 16)
	cancel := env.hub.WatchHost("h1", func(ev Event) { got <- ev })
	waitUntil(t, "the watcher to be live", func() bool { return env.hub.status("h1").State == StateLive })

	env.fe.send(t, eventLine("container", "start", webID, map[string]string{"name": "web", "com.docker.compose.project": "shop"}))
	select {
	case ev := <-got:
		if ev.Type != "container" || ev.Action != "start" || ev.Project != "shop" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WatchHost callback saw no event")
	}

	env.fe.dropFeed(t)
	select {
	case ev := <-got:
		if ev.Type != "resync" || ev.Action != "reconnect" || ev.Host != "h1" {
			t.Fatalf("resync event = %+v, want Type resync, Action reconnect", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WatchHost callback saw no resync after a reconnect")
	}

	cancel()
	cancel() // idempotent
	waitUntil(t, "the watcher to stop after the last callback left", func() bool { return env.hub.status("h1").State == "" })
}

func TestHubCloseStopsEverythingAndClosesClients(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.attach(t, Subscription{Hosts: []string{"h1"}, Stats: map[string][]string{"h1": {"aaaaaaaaaaaa"}}, HostStats: []string{"h1"}})
	waitFor(t, c, "host_status live", isStatus(StateLive))
	done := make(chan struct{})
	go func() { env.hub.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}
	if _, err := env.hub.Attach(); err == nil {
		t.Fatal("Attach after Close succeeded")
	}
	if _, err := c.Set(Subscription{}); err == nil {
		t.Fatal("Set after Close succeeded")
	}
}
