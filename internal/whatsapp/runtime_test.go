package whatsapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var testOwn = types.NewJID("4917099990000", types.DefaultUserServer)

func fixedNow() time.Time { return time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC) }

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testBotConfig(mode string, owners ...string) *config.Config {
	cfg := &config.Config{}
	cfg.Directories.DataDir = "data"
	cfg.WhatsApp.Enabled = true
	cfg.WhatsApp.Mode = mode
	cfg.WhatsApp.OwnerNumbers = owners
	cfg.WhatsApp.SendReadReceipts = true
	cfg.WhatsApp.SelfChatPrefix = config.DefaultWhatsAppSelfChatPrefix
	return cfg
}

// recordedSleeps replaces the Bot's sleep: it returns at once and records the
// requested delays. A sleep of an hour or more (a temporary ban) waits for
// hold when a test sets it.
type recordedSleeps struct {
	mu     sync.Mutex
	values []time.Duration
	hold   chan struct{}
}

func (r *recordedSleeps) sleep(ctx context.Context, d time.Duration) bool {
	r.mu.Lock()
	r.values = append(r.values, d)
	hold := r.hold
	r.mu.Unlock()
	if hold != nil && d >= time.Hour {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}
	return ctx.Err() == nil
}

func (r *recordedSleeps) list() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.values)
}

func startTestBot(t *testing.T, cfg *config.Config, st *fakeStore, sleeps *recordedSleeps) *Bot {
	t.Helper()
	deps := runtimeDeps{backend: &fakeBackend{store: st}, sleep: sleeps.sleep, now: fixedNow}
	b := start(context.Background(), cfg, quietLogger(), deps)
	if b == nil {
		t.Fatal("start returned no Bot")
	}
	t.Cleanup(b.Stop)
	return b
}

func clientCount(c *fakeClient, field func(*fakeClient) int) int {
	n := 0
	c.with(func(f *fakeClient) { n = field(f) })
	return n
}

func connectsOf(c *fakeClient) int {
	return clientCount(c, func(f *fakeClient) int { return f.connects })
}

func TestStartSkipsDisabledAndUnconfiguredSections(t *testing.T) {
	deps := runtimeDeps{backend: &fakeBackend{err: errors.New("must not open")}, sleep: sleepContext, now: fixedNow}
	disabled := testBotConfig(config.WhatsAppModeDedicated, "+491701234567")
	disabled.WhatsApp.Enabled = false
	egg := testBotConfig(config.WhatsAppModeSelfChat)
	egg.EggMode.Enabled = true
	noDataDir := testBotConfig(config.WhatsAppModeSelfChat)
	noDataDir.Directories.DataDir = ""
	for _, c := range []struct {
		cfg  *config.Config
		want string
	}{
		{disabled, integrationstatus.WhatsAppStateDisabled},
		{egg, integrationstatus.WhatsAppStateDisabled},
		{testBotConfig(config.WhatsAppModeDedicated), integrationstatus.WhatsAppStateNotConfigured},
		{noDataDir, integrationstatus.WhatsAppStateNotConfigured},
	} {
		if b := start(context.Background(), c.cfg, quietLogger(), deps); b != nil {
			b.Stop()
			t.Fatalf("a %s section started a Bot", c.want)
		}
		if got := integrationstatus.WhatsAppStatus().State; got != c.want {
			t.Fatalf("state = %q, want %q", got, c.want)
		}
	}
}

func TestBotWithoutLinkedDeviceReportsNotLinked(t *testing.T) {
	client := &fakeClient{}
	st := &fakeStore{client: client}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), st, &recordedSleeps{})
	select {
	case <-b.done:
	case <-time.After(5 * time.Second):
		t.Fatal("an unlinked Bot did not finish")
	}
	st.mu.Lock()
	closed := st.closed
	st.mu.Unlock()
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateNotLinked || s.Linked {
		t.Fatalf("status = %+v", s)
	}
	if connectsOf(client) != 0 || closed != 1 {
		t.Fatalf("connects=%d closed=%d, want 0/1", connectsOf(client), closed)
	}
}

func TestBotHealthyOnlyAfterOfflineSyncAndSendsUnavailable(t *testing.T) {
	client := &fakeClient{own: testOwn, loggedIn: true}
	startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: client}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(client) == 1 })
	client.emit(&events.Connected{})
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateConnecting ||
		s.ConnectedSince != "" || !s.Linked || s.LinkedJIDMasked != "***0000" {
		t.Fatalf("after Connected: %+v", s)
	}
	received := integrationstatus.WhatsAppStatus().MessagesReceived
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(&events.Message{})
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateHealthy ||
		s.ConnectedSince != "2026-10-31T12:00:00Z" || s.MessagesReceived != received+1 {
		t.Fatalf("after OfflineSyncCompleted: %+v", s)
	}
	waitFor(t, "presence unavailable", func() bool {
		var presences []types.Presence
		client.with(func(f *fakeClient) { presences = slices.Clone(f.presences) })
		return slices.Equal(presences, []types.Presence{types.PresenceUnavailable})
	})
}

func TestBotSyncStateFollowsTheConnection(t *testing.T) {
	client := &fakeClient{own: testOwn, loggedIn: true}
	startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: client}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(client) == 1 })
	expect := func(step, state, since string) {
		t.Helper()
		if s := integrationstatus.WhatsAppStatus(); s.State != state || s.ConnectedSince != since {
			t.Fatalf("%s: state=%q since=%q, want %q/%q", step, s.State, s.ConnectedSince, state, since)
		}
	}
	const since = "2026-10-31T12:00:00Z"

	// whatsmeow dispatches from goroutines: the sync can finish before Connected.
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(&events.Connected{})
	expect("sync before Connected", integrationstatus.WhatsAppStateHealthy, since)

	// A keepalive hiccup on the same connection recovers without a new sync.
	client.emit(&events.KeepAliveTimeout{})
	expect("keepalive timeout", integrationstatus.WhatsAppStateDegraded, "")
	client.emit(&events.KeepAliveRestored{})
	expect("keepalive restored", integrationstatus.WhatsAppStateHealthy, since)

	// A drop forgets the sync: the next connection is healthy only after its own.
	client.emit(&events.Disconnected{})
	expect("disconnected", integrationstatus.WhatsAppStateDegraded, "")
	client.emit(&events.Connected{})
	expect("reconnected", integrationstatus.WhatsAppStateConnecting, "")
	client.emit(&events.OfflineSyncCompleted{})
	expect("synced again", integrationstatus.WhatsAppStateHealthy, since)
}

func TestBotBackoffReconnectAndTerminalStates(t *testing.T) {
	dial := errors.New("dial tcp: connection refused")
	client := &fakeClient{connectErrs: []error{dial, dial}, own: testOwn, loggedIn: true}
	sleeps := &recordedSleeps{hold: make(chan struct{})}
	startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: client}, sleeps)
	waitFor(t, "third connect", func() bool { return connectsOf(client) == 3 })
	if got := sleeps.list(); !slices.Equal(got, []time.Duration{5 * time.Second, 10 * time.Second}) {
		t.Fatalf("backoff = %v, want [5s 10s]", got)
	}
	client.emit(&events.ConnectFailure{Reason: events.ConnectFailureReason(418)})
	waitFor(t, "reconnect after a connect failure", func() bool { return connectsOf(client) == 4 })
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(&events.ConnectFailure{Reason: events.ConnectFailureReason(418)})
	waitFor(t, "reconnect after a healthy connection", func() bool { return connectsOf(client) == 5 })
	if got := sleeps.list(); !slices.Equal(got, []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 5 * time.Second}) {
		t.Fatalf("backoff after reconnects = %v", got)
	}
	client.emit(&events.TemporaryBan{Code: events.TempBanSentToTooManyPeople, Expire: time.Hour})
	waitFor(t, "ban wait", func() bool { return len(sleeps.list()) == 5 })
	if s := integrationstatus.WhatsAppStatus(); s.State != integrationstatus.WhatsAppStateTempBanned ||
		s.TempBanExpiresAt != "2026-10-31T13:00:00Z" || s.LastError != "temp_ban_101" {
		t.Fatalf("temp ban status = %+v", s)
	}
	if n := connectsOf(client); n != 5 {
		t.Fatalf("reconnected during the ban: %d connects", n)
	}
	close(sleeps.hold)
	waitFor(t, "retry after the ban expired", func() bool { return connectsOf(client) == 6 })
	client.emit(&events.StreamReplaced{})
	waitFor(t, "conflict state", func() bool {
		return integrationstatus.WhatsAppStatus().State == integrationstatus.WhatsAppStateConflict
	})
	time.Sleep(50 * time.Millisecond)
	if n := connectsOf(client); n != 6 {
		t.Fatalf("a terminal state kept reconnecting: %d connects", n)
	}
}

func TestBotMatchesConfigAndCancelsOnChange(t *testing.T) {
	cfg := testBotConfig(config.WhatsAppModeDedicated, "+491701234567")
	b := startTestBot(t, cfg, &fakeStore{client: &fakeClient{}}, &recordedSleeps{})
	if !b.MatchesConfig(cfg.Clone()) {
		t.Fatal("the starting config does not match")
	}
	b.CancelIfConfigChanged(cfg.Clone())
	if b.ctx.Err() != nil {
		t.Fatal("an unchanged config cancelled the generation")
	}
	changed := cfg.Clone()
	changed.WhatsApp.SelfChatPrefix = "AI: "
	b.CancelIfConfigChanged(changed)
	if b.ctx.Err() == nil || b.MatchesConfig(cfg) {
		t.Fatal("a changed config did not revoke the generation")
	}
}

func TestBotSendTestMessageReachesOnlyOwners(t *testing.T) {
	ctx := context.Background()
	if _, err := ownerRecipients(config.WhatsAppConfig{Mode: config.WhatsAppModeSelfChat}, types.EmptyJID); !errors.Is(err, ErrNoRecipient) {
		t.Fatalf("self chat without own JID: %v", err)
	}
	if _, err := ownerRecipients(config.WhatsAppConfig{Mode: config.WhatsAppModeDedicated}, testOwn); !errors.Is(err, ErrNoRecipient) {
		t.Fatalf("dedicated without owners: %v", err)
	}
	offline := &fakeClient{own: testOwn}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: offline}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(offline) == 1 })
	if err := b.SendTestMessage(ctx, fixedNow()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("logged-out send = %v, want ErrNotConnected", err)
	}
	want := "AuraGo WhatsApp test (manual) — 2026-10-31T12:00:00Z"
	dedicated := &fakeClient{own: testOwn, loggedIn: true}
	b = startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567", "+436601234567"), &fakeStore{linked: true, own: testOwn, client: dedicated}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(dedicated) == 1 })
	if err := b.SendTestMessage(ctx, fixedNow()); err != nil {
		t.Fatalf("dedicated send: %v", err)
	}
	var sent []sentMessage
	dedicated.with(func(f *fakeClient) { sent = slices.Clone(f.sent) })
	if len(sent) != 2 || sent[0].to.String() != "491701234567@s.whatsapp.net" || sent[1].to.String() != "436601234567@s.whatsapp.net" || sent[0].text != want {
		t.Fatalf("dedicated sends = %+v", sent)
	}
	self := &fakeClient{own: testOwn, loggedIn: true}
	b = startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), &fakeStore{linked: true, own: testOwn, client: self}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(self) == 1 })
	if err := b.SendTestMessage(ctx, fixedNow()); err != nil {
		t.Fatalf("self-chat send: %v", err)
	}
	self.with(func(f *fakeClient) { sent = slices.Clone(f.sent) })
	if len(sent) != 1 || sent[0].to != testOwn || sent[0].text != config.DefaultWhatsAppSelfChatPrefix+want {
		t.Fatalf("self-chat sends = %+v", sent)
	}
	if integrationstatus.WhatsAppStatus().LastTestAt != "2026-10-31T12:00:00Z" {
		t.Fatal("the test send was not recorded")
	}
}

func TestBotLogoutPrefersRemoteAndFallsBackToLocal(t *testing.T) {
	ctx := context.Background()
	remote := &fakeClient{own: testOwn, loggedIn: true}
	remoteStore := &fakeStore{linked: true, own: testOwn, client: remote}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), remoteStore, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(remote) == 1 })
	result, err := b.Logout(ctx)
	remoteStore.mu.Lock()
	deleted := remoteStore.deleted
	remoteStore.mu.Unlock()
	if err != nil || !result.RemoteRemoved || clientCount(remote, func(f *fakeClient) int { return f.logouts }) != 1 || deleted != 0 || b.ctx.Err() == nil {
		t.Fatalf("remote logout: result=%+v err=%v deleted=%d", result, err, deleted)
	}
	local := &fakeClient{own: testOwn, loggedIn: true, logoutErr: errors.New("websocket not connected")}
	localStore := &fakeStore{linked: true, own: testOwn, client: local}
	b = startTestBot(t, testBotConfig(config.WhatsAppModeSelfChat), localStore, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(local) == 1 })
	result, err = b.Logout(ctx)
	localStore.mu.Lock()
	deleted = localStore.deleted
	localStore.mu.Unlock()
	if err != nil || result.RemoteRemoved || deleted != 1 {
		t.Fatalf("local fallback: result=%+v err=%v deleted=%d", result, err, deleted)
	}
	if s := integrationstatus.WhatsAppStatus(); s.Linked || s.State != integrationstatus.WhatsAppStateNotLinked {
		t.Fatalf("status after logout = %+v", s)
	}
}

func TestBotRetriesAnUnavailableDeviceStore(t *testing.T) {
	client := &fakeClient{own: testOwn, loggedIn: true}
	st := &fakeStore{linked: true, own: testOwn, client: client}
	disk := errors.New("disk i/o error")
	backend := &fakeBackend{store: st, openErrs: []error{disk, disk}}
	sleeps := &recordedSleeps{}
	b := start(context.Background(), testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), quietLogger(),
		runtimeDeps{backend: backend, sleep: sleeps.sleep, now: fixedNow})
	if b == nil {
		t.Fatal("start returned no Bot")
	}
	t.Cleanup(b.Stop)
	waitFor(t, "connect after the store opened", func() bool { return connectsOf(client) == 1 })
	if got := sleeps.list(); !slices.Equal(got, []time.Duration{5 * time.Second, 10 * time.Second}) {
		t.Fatalf("store retry backoff = %v, want [5s 10s]", got)
	}
}

func TestStoppedBotNeverWritesStatus(t *testing.T) {
	client := &fakeClient{own: testOwn, loggedIn: true}
	b := startTestBot(t, testBotConfig(config.WhatsAppModeDedicated, "+491701234567"), &fakeStore{linked: true, own: testOwn, client: client}, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(client) == 1 })
	b.Stop()
	b.eventMu.Lock()
	stopped := b.stopped
	b.eventMu.Unlock()
	if !stopped {
		t.Fatal("Stop returned before the generation was marked stopped")
	}
	// A successor generation owns the status now. Drop the cancelled context
	// so only the stopped flag can keep a late dispatch goroutine out.
	integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateConnecting, config.WhatsAppModeDedicated)
	before := integrationstatus.WhatsAppStatus()
	b.ctx = context.Background()
	client.emit(&events.LoggedOut{})
	client.emit(&events.Disconnected{})
	client.emit(&events.Message{})
	if after := integrationstatus.WhatsAppStatus(); after != before {
		t.Fatalf("a stopped Bot changed the status: %+v -> %+v", before, after)
	}
}

func TestNextBackoffDoublesUpToTheCap(t *testing.T) {
	d := minBackoff
	for i, want := range []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second,
		80 * time.Second, 160 * time.Second, 300 * time.Second, 300 * time.Second, 300 * time.Second} {
		if d != want {
			t.Fatalf("step %d: backoff = %v, want %v", i, d, want)
		}
		d = nextBackoff(d)
	}
}

func TestSleepContextStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	began := time.Now()
	if sleepContext(ctx, time.Hour) {
		t.Fatal("a cancelled context finished the sleep")
	}
	if time.Since(began) > time.Second {
		t.Fatalf("a cancelled sleep took %v", time.Since(began))
	}
	if !sleepContext(context.Background(), time.Millisecond) {
		t.Fatal("an uncancelled sleep reported cancellation")
	}
}
