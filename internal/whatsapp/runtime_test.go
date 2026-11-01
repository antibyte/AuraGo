package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
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
		// No owner survives NormalizeE164: a Bot would drop every message as a stranger's.
		{testBotConfig(config.WhatsAppModeDedicated, "not a number", "   ", "0049170"), integrationstatus.WhatsAppStateNotConfigured},
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
	if n := clientCount(client, func(f *fakeClient) int { return f.disconnects }); n != 2 {
		t.Fatalf("a failed connect left the socket open: %d disconnects before the third attempt, want 2", n)
	}
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
	if b.deps.agentLoop == nil || b.filter == nil || b.pipeline == nil || cap(b.slots) != pipelineWorkers {
		t.Fatal("start left the inbound path unset")
	}
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
	if _, err := ownerRecipients(config.WhatsAppConfig{Mode: config.WhatsAppModeDedicated}, testOwn); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("dedicated without owners: %v, want ErrNotConfigured", err)
	}
	if _, err := ownerRecipients(config.WhatsAppConfig{Mode: config.WhatsAppModeDedicated, OwnerNumbers: []string{"not a number"}}, testOwn); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("dedicated with only invalid owners: %v, want ErrNotConfigured", err)
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
	if after := integrationstatus.WhatsAppStatus(); !reflect.DeepEqual(after, before) { // the status holds a map now
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

// agentBotFixture is a running Bot over the fake client, an in-memory memory store and a
// recording agent loop.
type agentBotFixture struct {
	bot    *Bot
	client *fakeClient
	store  *fakeStore
	logs   *bytes.Buffer
	mu     sync.Mutex
	calls  []agent.RunConfig
}

const fixtureAnswer = "answer from the stub"

func (f *agentBotFixture) recordLoop(_ context.Context, _ openai.ChatCompletionRequest, runCfg agent.RunConfig, _ bool, _ agent.FeedbackBroker) (openai.ChatCompletionResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, runCfg)
	f.mu.Unlock()
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{
		{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: fixtureAnswer}},
	}}, nil
}

func (f *agentBotFixture) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// startAgentBot starts a linked Bot; a nil loop records the calls and answers fixtureAnswer.
func startAgentBot(t *testing.T, cfg *config.Config, client *fakeClient, loop agentLoopFunc) *agentBotFixture {
	t.Helper()
	f := &agentBotFixture{client: client, logs: &bytes.Buffer{}}
	f.store = &fakeStore{linked: true, own: testOwn, client: client}
	logger := slog.New(slog.NewTextHandler(f.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() }) // runs after the Bot's Stop below
	hm := memory.NewEphemeralHistoryManager()
	t.Cleanup(hm.Close)
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.PromptsDir = t.TempDir()
	if loop == nil {
		loop = f.recordLoop
	}
	deps := runtimeDeps{backend: &fakeBackend{store: f.store}, sleep: (&recordedSleeps{}).sleep, now: fixedNow,
		agent: AgentDeps{ShortTermMem: stm, HistoryManager: hm}, agentLoop: loop}
	f.bot = start(context.Background(), cfg, logger, deps)
	if f.bot == nil {
		t.Fatal("start returned no Bot")
	}
	t.Cleanup(f.bot.Stop)
	waitFor(t, "connect", func() bool { return connectsOf(client) == 1 })
	return f
}

// inboundEvent is a text message from `from` in their own chat, stamped `at`.
func inboundEvent(id string, from types.JID, text string, at time.Time) *events.Message {
	info := dmInfo(id, from)
	info.Timestamp = at
	return msgEvent(info, textMsg(text))
}

func filteredDrops(reason string) int {
	return integrationstatus.WhatsAppStatus().DroppedFiltered[reason]
}

func TestBotRoutesOwnerMessagesToThePipeline(t *testing.T) {
	const ownerText, strangerText = "owner-secret-text", "stranger-secret-text"
	client := &fakeClient{own: testOwn, loggedIn: true}
	f := startAgentBot(t, testBotConfig(config.WhatsAppModeDedicated, "+4917012345678"), client, nil)

	// The echo guard is the sender's own cache, not a copy.
	if f.bot.filter.sent != f.bot.sent || f.bot.sender.sent != f.bot.sent {
		t.Fatal("the inbound filter does not share the sender's sent cache")
	}

	client.emit(inboundEvent("OWNER1", ownerJID, ownerText, fixedNow()))
	waitFor(t, "the agent call", func() bool { return f.callCount() == 1 })
	f.mu.Lock()
	source := f.calls[0].MessageSource
	f.mu.Unlock()
	if source != "whatsapp" {
		t.Fatalf("message source = %q, want whatsapp", source)
	}
	waitFor(t, "the reply", func() bool { return len(sentTexts(client)) == 1 })
	if sent := sentTexts(client)[0]; sent.to != ownerJID || sent.text != fixtureAnswer {
		t.Fatalf("reply = %+v", sent)
	}

	// A stranger is dropped by the filter on the emitting goroutine: the counters are final
	// when emit returns, and the message counts as received before it is dropped.
	s1 := integrationstatus.WhatsAppStatus()
	client.emit(inboundEvent("STRANGER1", strangerJID, strangerText, fixedNow()))
	s2 := integrationstatus.WhatsAppStatus()
	if s2.DroppedNonOwner != s1.DroppedNonOwner+1 || s2.MessagesReceived != s1.MessagesReceived+1 {
		t.Fatalf("stranger: dropped %d -> %d, received %d -> %d", s1.DroppedNonOwner, s2.DroppedNonOwner, s1.MessagesReceived, s2.MessagesReceived)
	}

	// A repeated ID is counted once as received and once as dropped.
	client.emit(inboundEvent("OWNER1", ownerJID, ownerText, fixedNow()))
	s3 := integrationstatus.WhatsAppStatus()
	if s3.DroppedFiltered[dropDuplicate] != s2.DroppedFiltered[dropDuplicate]+1 ||
		s3.MessagesReceived != s2.MessagesReceived+1 || s3.DroppedNonOwner != s2.DroppedNonOwner {
		t.Fatalf("duplicate: dropped %v -> %v, received %d -> %d", s2.DroppedFiltered, s3.DroppedFiltered, s2.MessagesReceived, s3.MessagesReceived)
	}

	// The reply the Bot sent comes back as an inbound event with the ID the fake gave it
	// ("fake-1"); only the shared sent cache can recognise it.
	client.emit(inboundEvent("fake-1", ownerJID, fixtureAnswer, fixedNow()))
	if got := filteredDrops(dropEcho); got != s3.DroppedFiltered[dropEcho]+1 {
		t.Fatalf("echo drops = %d, want %d", got, s3.DroppedFiltered[dropEcho]+1)
	}
	if n := f.callCount(); n != 1 {
		t.Fatalf("agent calls = %d, want 1: only the first owner message may start a turn", n)
	}

	// Logs carry masked JIDs and no message text.
	f.bot.Stop()
	out := f.logs.String()
	if !strings.Contains(out, "***5678") {
		t.Fatalf("no masked owner in the logs: %q", out)
	}
	for _, leak := range []string{ownerText, strangerText, "4917012345678", "4917099998888"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log leaks %q: %s", leak, out)
		}
	}
}

func TestBotOfflineCutoffAppliesOnlyDuringOfflineSync(t *testing.T) {
	client := &fakeClient{own: testOwn, loggedIn: true, lastConnect: fixedNow()}
	f := startAgentBot(t, testBotConfig(config.WhatsAppModeDedicated, "+4917012345678"), client, nil)
	at := func(age time.Duration) time.Time { return fixedNow().Add(-age) }

	// During the offline sync a message stamped before the cutoff is stale, one inside the
	// window is not.
	before := filteredDrops(dropStale)
	client.emit(inboundEvent("OLD1", ownerJID, "x", at(11*time.Minute)))
	if got := filteredDrops(dropStale); got != before+1 || f.callCount() != 0 {
		t.Fatalf("stale drops = %d (want %d), agent calls = %d (want 0)", got, before+1, f.callCount())
	}
	client.emit(inboundEvent("NEW1", ownerJID, "x", at(9*time.Minute)))
	waitFor(t, "a message inside the window", func() bool { return f.callCount() == 1 })

	// After the sync the cutoff is gone: the same old stamp reaches the pipeline.
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(inboundEvent("OLD2", ownerJID, "x", at(11*time.Minute)))
	waitFor(t, "an old stamp after the sync", func() bool { return f.callCount() == 2 })

	// A keepalive timeout keeps the connection and its finished sync.
	client.emit(&events.KeepAliveTimeout{})
	client.emit(inboundEvent("OLD3", ownerJID, "x", at(11*time.Minute)))
	waitFor(t, "an old stamp after a keepalive timeout", func() bool { return f.callCount() == 3 })

	// A dropped connection forgets the sync: its successor has an offline sync of its own.
	client.emit(&events.Disconnected{})
	before = filteredDrops(dropStale)
	client.emit(inboundEvent("OLD4", ownerJID, "x", at(11*time.Minute)))
	if got := filteredDrops(dropStale); got != before+1 || f.callCount() != 3 {
		t.Fatalf("after a disconnect: stale drops = %d (want %d), agent calls = %d (want 3)", got, before+1, f.callCount())
	}
}

// whatsmeow forces a reconnect after a failed keepalive without a Disconnected event, so the
// Bot's resetConnection does not run; the new login's offline messages can come before its
// Connected. The cutoff has to follow the login, not the events the Bot happened to see.
func TestBotForcedKeepAliveReconnectKeepsTheOfflineCutoff(t *testing.T) {
	login := func(c *fakeClient, at time.Time) { c.with(func(f *fakeClient) { f.lastConnect = at }) }
	client := &fakeClient{own: testOwn, loggedIn: true, lastConnect: fixedNow().Add(-2 * time.Hour)}
	f := startAgentBot(t, testBotConfig(config.WhatsAppModeDedicated, "+4917012345678"), client, nil)
	old := fixedNow().Add(-30 * time.Minute) // queued while the connection was down

	// Control: a keepalive timeout on the same login keeps its finished sync.
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(&events.KeepAliveTimeout{})
	client.emit(inboundEvent("SAME1", ownerJID, "x", old))
	waitFor(t, "same-login message after a keepalive timeout", func() bool { return f.callCount() == 1 })

	// The forced reconnect accepts a new login. Its first offline message is stale.
	login(client, fixedNow())
	before := filteredDrops(dropStale)
	client.emit(inboundEvent("FORCED1", ownerJID, "x", old))
	if got := filteredDrops(dropStale); got != before+1 || f.callCount() != 1 {
		t.Fatalf("forced reconnect: stale drops = %d (want %d), agent calls = %d (want 1)", got, before+1, f.callCount())
	}

	// The new login's own sync ends the cutoff.
	client.emit(&events.OfflineSyncCompleted{})
	client.emit(inboundEvent("FORCED2", ownerJID, "x", old))
	waitFor(t, "message after the new login's sync", func() bool { return f.callCount() == 2 })

	// Another forced reconnect, announced by its offline preview before any message.
	client.emit(&events.KeepAliveTimeout{})
	login(client, fixedNow().Add(time.Minute))
	client.emit(&events.OfflineSyncPreview{Total: 1, Messages: 1})
	before = filteredDrops(dropStale)
	client.emit(inboundEvent("FORCED3", ownerJID, "x", old))
	if got := filteredDrops(dropStale); got != before+1 || f.callCount() != 2 {
		t.Fatalf("second forced reconnect: stale drops = %d (want %d), agent calls = %d (want 2)", got, before+1, f.callCount())
	}
}

func TestBotStopWaitsForPipelineWorkers(t *testing.T) {
	var f *agentBotFixture
	started := make(chan struct{})
	var once sync.Once
	var finished atomic.Bool
	var closedAtEnd atomic.Int32
	loop := func(ctx context.Context, _ openai.ChatCompletionRequest, _ agent.RunConfig, _ bool, _ agent.FeedbackBroker) (openai.ChatCompletionResponse, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond) // the teardown has to wait for this
		f.store.mu.Lock()
		closedAtEnd.Store(int32(f.store.closed))
		f.store.mu.Unlock()
		finished.Store(true)
		return openai.ChatCompletionResponse{}, ctx.Err()
	}
	client := &fakeClient{own: testOwn, loggedIn: true}
	f = startAgentBot(t, testBotConfig(config.WhatsAppModeDedicated, "+4917012345678"), client, loop)
	client.emit(inboundEvent("BLOCK1", ownerJID, "x", fixedNow()))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the agent turn never started")
	}
	f.bot.Stop()
	if !finished.Load() {
		t.Fatal("Stop returned while an agent turn was still running")
	}
	f.store.mu.Lock()
	closed := f.store.closed
	f.store.mu.Unlock()
	if closedAtEnd.Load() != 0 || closed != 1 {
		t.Fatalf("device store closed %d times before the worker ended, %d after Stop; want 0 and 1", closedAtEnd.Load(), closed)
	}
	if got := sentTexts(client); len(got) != 0 {
		t.Fatalf("a cancelled turn still replied: %+v", got)
	}
}

func TestBotStaleGenerationRoutesNothing(t *testing.T) {
	cfg := testBotConfig(config.WhatsAppModeDedicated, "+4917012345678")
	client := &fakeClient{own: testOwn, loggedIn: true}
	f := startAgentBot(t, cfg, client, nil)
	changed := cfg.Clone()
	changed.WhatsApp.OwnerNumbers = []string{"+4917099998888"}
	f.bot.CancelIfConfigChanged(changed)

	before := integrationstatus.WhatsAppStatus()
	client.emit(inboundEvent("STALE1", ownerJID, "x", fixedNow()))
	after := integrationstatus.WhatsAppStatus()
	if f.callCount() != 0 || after.MessagesReceived != before.MessagesReceived || !reflect.DeepEqual(after.DroppedFiltered, before.DroppedFiltered) {
		t.Fatalf("a revoked generation took a message: calls=%d status %+v -> %+v", f.callCount(), before, after)
	}
	f.bot.Stop()
	if got := sentTexts(client); len(got) != 0 {
		t.Fatalf("a revoked generation replied: %+v", got)
	}
}

// A panic in a pipeline worker (a media path, a malformed proto) must not end the process:
// the worker logs one fixed line without the panic value or the message, gives its slot
// back, and later messages are still handled.
func TestBotSurvivesAPipelinePanicAndReleasesTheSlot(t *testing.T) {
	const secret = "panic-secret-text"
	const panics = pipelineWorkers + 1 // more than the slots: a leaked slot would block the last one
	var started, panicked atomic.Int32
	var ok atomic.Int32
	loop := func(context.Context, openai.ChatCompletionRequest, agent.RunConfig, bool, agent.FeedbackBroker) (openai.ChatCompletionResponse, error) {
		if started.Add(1) <= panics {
			panicked.Add(1)
			panic("media path failed on " + secret)
		}
		ok.Add(1)
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: fixtureAnswer}},
		}}, nil
	}
	client := &fakeClient{own: testOwn, loggedIn: true}
	f := startAgentBot(t, testBotConfig(config.WhatsAppModeDedicated, "+4917012345678"), client, loop)

	for i := 0; i < panics; i++ {
		client.emit(inboundEvent(fmt.Sprintf("PANIC%d", i), ownerJID, secret, fixedNow()))
	}
	waitFor(t, "every panicking turn", func() bool { return panicked.Load() == panics })

	client.emit(inboundEvent("AFTER1", ownerJID, secret, fixedNow()))
	waitFor(t, "a message after the panics", func() bool { return ok.Load() == 1 })
	waitFor(t, "the reply", func() bool { return len(sentTexts(client)) == 1 })

	stopped := make(chan struct{})
	go func() { f.bot.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after worker panics")
	}

	out := f.logs.String()
	if n := strings.Count(out, "WhatsApp pipeline worker recovered from a panic"); n != panics {
		t.Fatalf("recovered-panic log lines = %d, want %d: %s", n, panics, out)
	}
	if !strings.Contains(out, "panic_type=string") || !strings.Contains(out, "sender=***5678") {
		t.Fatalf("log lacks the panic type or the masked sender: %s", out)
	}
	for _, leak := range []string{secret, "media path failed", "4917012345678"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log leaks %q: %s", leak, out)
		}
	}
}
