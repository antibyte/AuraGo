package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Connect backoff for startup failures and for disconnects whatsmeow does not
// recover by itself. It never gives up.
const (
	minBackoff = 5 * time.Second
	maxBackoff = 5 * time.Minute
)

const (
	// offlineCutoff is how far before the login a message may be stamped and still count
	// as live while the connection's offline sync runs.
	offlineCutoff = 10 * time.Minute
	// pipelineWorkers bounds the agent turns that run at once; it is Telegram's default
	// MaxConcurrentWorkers (internal/telegram/runtime.go).
	pipelineWorkers = 5
)

func nextBackoff(d time.Duration) time.Duration {
	if d*2 > maxBackoff {
		return maxBackoff
	}
	return d * 2
}

// runtimeDeps are the Bot's replaceable collaborators.
type runtimeDeps struct {
	backend backend
	sleep   func(ctx context.Context, d time.Duration) bool
	now     func() time.Time
	// agent holds the collaborators of the agent pipeline; agentLoop runs one turn and
	// defaults to agent.ExecuteAgentLoop.
	agent     AgentDeps
	agentLoop agentLoopFunc
}

func defaultRuntimeDeps(logger *slog.Logger) runtimeDeps {
	return runtimeDeps{backend: sqliteBackend{log: newWALogger(logger, "whatsmeow")}, sleep: sleepContext, now: time.Now, agentLoop: agent.ExecuteAgentLoop}
}

// sleepContext waits for d. It returns false if ctx ends first.
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// LogoutResult says whether WhatsApp also removed the linked device from the
// phone. false means only the local keys were deleted.
type LogoutResult struct {
	RemoteRemoved bool `json:"remote_removed"`
}

// ConfigState returns "disabled" or "not_configured" when the configuration
// does not allow a Bot, and "" when it does. It does not check the device
// store. Dedicated mode needs an owner number that survives config.NormalizeE164:
// a Bot without one would drop every message as a stranger's.
func ConfigState(cfg *config.Config) string {
	switch {
	case cfg == nil || cfg.EggMode.Enabled || !cfg.WhatsApp.Enabled:
		return integrationstatus.WhatsAppStateDisabled
	case strings.TrimSpace(cfg.Directories.DataDir) == "":
		return integrationstatus.WhatsAppStateNotConfigured
	case cfg.WhatsApp.Mode != config.WhatsAppModeSelfChat && !hasValidOwnerNumber(cfg.WhatsApp.OwnerNumbers):
		return integrationstatus.WhatsAppStateNotConfigured
	}
	return ""
}

// hasValidOwnerNumber reports whether at least one entry is a usable E.164 number.
func hasValidOwnerNumber(numbers []string) bool {
	for _, raw := range numbers {
		if _, err := config.NormalizeE164(raw); err == nil {
			return true
		}
	}
	return false
}

func configMode(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.WhatsApp.Mode
}

// Bot is one WhatsApp connection generation. The server owns at most one per
// process. A config change cancels it, and the next generation starts without
// relinking.
//
// Reconnect ownership: whatsmeow reconnects transient drops itself
// (Disconnected, KeepAliveTimeout) with its delay capped at five minutes. The
// Bot connects the first time, retries failed connects with backoff, and
// reconnects after ConnectFailure and after a temporary ban, where whatsmeow
// expects the disconnect and stays down. The Bot always calls Disconnect
// before it connects again; that cancels any pending whatsmeow retry, and a
// connect that still finds a live socket (ErrAlreadyConnected) counts as
// connected.
type Bot struct {
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	initial config.WhatsAppConfig
	dataDir string
	logger  *slog.Logger
	deps    runtimeDeps

	// sent holds the IDs of our own outgoing messages, the inbound filter's echo
	// guard. sender is the one serialized, owner-only outbound queue.
	sent   *idCache
	sender *sender

	// filter and pipeline are this generation's inbound path; both are built over the
	// same sent cache and the same config clone as sender. Agent turns run on workers,
	// at most pipelineWorkers at once (slots), and run() waits for them (workers).
	filter   *inboundFilter
	pipeline *pipeline
	workers  sync.WaitGroup
	slots    chan struct{}

	mu     sync.Mutex
	client waClient

	// eventMu serializes handleEvent and guards the connection details below.
	// whatsmeow dispatches from several goroutines, so Connected and
	// OfflineSyncCompleted can race.
	eventMu sync.Mutex
	// synced: the current connection finished its offline sync.
	synced bool
	// resumeSynced: a keepalive timeout hid a finished sync; KeepAliveRestored
	// on the same connection brings it back.
	resumeSynced bool
	// connectedAt is the current connection's login time, shown while healthy.
	connectedAt time.Time
	// stopped is set when the generation has torn down; handleEvent then
	// writes nothing.
	stopped bool
	// loginSeen is the login (LastSuccessfulConnect) that synced and resumeSynced
	// describe. A login the Bot did not start, such as whatsmeow's forced reconnect after
	// a failed keepalive, has no Disconnected event and skips resetConnection; the
	// first handler-queue event of the new login resets the sync flags instead.
	loginSeen time.Time
}

// Start returns a running generation, or nil when the configuration does not
// allow one; the status then says why. The device store opens asynchronously.
// agentDeps are the collaborators the agent turns of owner messages run with.
func Start(parent context.Context, cfg *config.Config, logger *slog.Logger, agentDeps AgentDeps) *Bot {
	if logger == nil {
		logger = slog.Default()
	}
	deps := defaultRuntimeDeps(logger)
	deps.agent = agentDeps
	return start(parent, cfg, logger, deps)
}

func start(parent context.Context, cfg *config.Config, logger *slog.Logger, deps runtimeDeps) *Bot {
	if state := ConfigState(cfg); state != "" {
		integrationstatus.SetWhatsAppConfigState(state, configMode(cfg))
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil
	}
	if deps.agentLoop == nil {
		deps.agentLoop = agent.ExecuteAgentLoop
	}
	cfg = cfg.Clone()
	ctx, cancel := context.WithCancel(parent)
	b := &Bot{ctx: ctx, cancel: cancel, done: make(chan struct{}), initial: cfg.WhatsApp,
		dataDir: cfg.Directories.DataDir, logger: logger, deps: deps}
	b.sent = newIDCache(sentIDCacheSize, sentIDCacheTTL, deps.now)
	b.sender = newSender(cfg.WhatsApp, b.currentClient, b.sent, deps, logger)
	// The filter shares the sender's sent cache (its echo guard); the pipeline gets this
	// generation's config clone and sender, so a stale generation never replies with
	// another generation's owners.
	b.filter = newInboundFilter(cfg.WhatsApp, b.sent, logger)
	b.pipeline = &pipeline{base: cfg, deps: deps.agent, sender: b.sender, client: b.currentClient,
		agentLoop: deps.agentLoop, now: deps.now, logger: logger}
	b.slots = make(chan struct{}, pipelineWorkers)
	integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateConnecting, cfg.WhatsApp.Mode)
	go b.run()
	return b
}

// MatchesConfig reports whether this live generation still serves cfg.
func (b *Bot) MatchesConfig(cfg *config.Config) bool {
	return b != nil && b.ctx.Err() == nil && cfg != nil && !cfg.EggMode.Enabled &&
		b.dataDir == cfg.Directories.DataDir && b.initial.Equal(cfg.WhatsApp)
}

// CancelIfConfigChanged revokes the generation when a config is published.
// Stop drains it after the server releases its config lock.
func (b *Bot) CancelIfConfigChanged(cfg *config.Config) {
	if b != nil && !b.MatchesConfig(cfg) {
		b.cancel()
	}
}

// Stop cancels the generation and waits until it has disconnected and closed
// the store. It is idempotent.
func (b *Bot) Stop() {
	if b != nil {
		b.cancel()
		<-b.done
	}
}

func (b *Bot) setClient(c waClient) {
	b.mu.Lock()
	b.client = c
	b.mu.Unlock()
}

func (b *Bot) currentClient() waClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.client
}

// finish ends the generation. Once stopped is set, a dispatch goroutine that
// is still inside handleEvent can no longer write status, so it cannot
// overwrite the successor generation.
func (b *Bot) finish() {
	b.eventMu.Lock()
	b.stopped = true
	b.eventMu.Unlock()
	close(b.done)
}

// openStore opens the device store. A failure leaves the status degraded and
// retries with backoff until the Bot stops. It returns nil when the Bot
// stopped first.
func (b *Bot) openStore() deviceStore {
	backoff := minBackoff
	for b.ctx.Err() == nil {
		store, err := b.deps.backend.open(b.ctx, b.dataDir)
		if err == nil {
			return store
		}
		if b.ctx.Err() != nil {
			return nil
		}
		integrationstatus.MarkWhatsAppError(integrationstatus.WhatsAppStateDegraded, "store_unavailable")
		b.logger.Error("WhatsApp device store unavailable", "retry_in", backoff.String(), "error", maskDigits(security.Scrub(err.Error())))
		if !b.deps.sleep(b.ctx, backoff) {
			return nil
		}
		backoff = nextBackoff(backoff)
	}
	return nil
}

func (b *Bot) run() {
	defer b.finish()
	store := b.openStore()
	if store == nil {
		return
	}
	defer store.Close()
	if b.ctx.Err() != nil {
		return
	}
	integrationstatus.SetWhatsAppLinked(store.Linked(), maskJID(store.OwnJID()))
	if !store.Linked() {
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
		return
	}
	client := store.NewClient()
	signals := make(chan eventEffect, 16)
	client.AddEventHandler(func(evt any) {
		b.handleEvent(client, evt, signals) // counts the message before the filter sees it
		if m, ok := evt.(*events.Message); ok {
			b.handleMessage(client, m)
		}
	})
	b.setClient(client)
	defer func() {
		b.eventMu.Lock()
		b.stopped = true // no new workers, no status writes (finish sets it again; idempotent)
		b.eventMu.Unlock()
		b.cancel() // the pipeline and broker contexts end with the generation
		b.setClient(nil)
		client.Disconnect()
		b.workers.Wait() // workers end on b.ctx; the store closes only after them
	}()
	b.connectLoop(client, signals)
}

// resetConnection forgets what the previous connection reached. It runs
// before every connect.
func (b *Bot) resetConnection() {
	b.eventMu.Lock()
	b.synced, b.resumeSynced, b.connectedAt = false, false, time.Time{}
	b.eventMu.Unlock()
}

// connectLoop keeps one connection open until the Bot stops. Connect errors,
// and disconnects that whatsmeow does not recover, retry with backoff.
// Terminal states wait for the user; a temporary ban waits until it expires.
func (b *Bot) connectLoop(client waClient, signals <-chan eventEffect) {
	backoff := minBackoff
	for b.ctx.Err() == nil {
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateConnecting)
		b.resetConnection()
		if err := client.ConnectContext(b.ctx); err != nil && !errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			if b.ctx.Err() != nil {
				return
			}
			integrationstatus.MarkWhatsAppError(integrationstatus.WhatsAppStateDegraded, "connect_failed")
			b.logger.Warn("WhatsApp connect failed", "retry_in", backoff.String(), "error", maskDigits(security.Scrub(err.Error())))
			// A failed dial can leave a half-open socket; drop it before the next attempt.
			client.Disconnect()
			if !b.deps.sleep(b.ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		eff, ok := b.awaitInterruption(signals, &backoff)
		if !ok {
			return
		}
		client.Disconnect()
		switch {
		case eff.Terminal && eff.RetryAfter > 0:
			if !b.deps.sleep(b.ctx, eff.RetryAfter) {
				return
			}
			backoff = minBackoff
		case eff.Terminal:
			<-b.ctx.Done()
			return
		default:
			if !b.deps.sleep(b.ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
		}
	}
}

// awaitInterruption waits on an open connection until the Bot stops (false)
// or an event needs the loop. A healthy connection resets the backoff.
func (b *Bot) awaitInterruption(signals <-chan eventEffect, backoff *time.Duration) (eventEffect, bool) {
	for {
		select {
		case <-b.ctx.Done():
			return eventEffect{}, false
		case eff := <-signals:
			if eff.Healthy {
				*backoff = minBackoff
			}
			if eff.Terminal || eff.Reconnect {
				return eff, true
			}
		}
	}
}

// observeLogin forgets the sync flags when whatsmeow accepted a new login that the Bot has not
// seen. Only the events that arrive on the handler queue read LastSuccessfulConnect: a message
// and the two offline-sync events. The queue orders them after the login node, which sets the
// field, so the read is race-free. Every offline message of a new login comes after its login,
// so by then the flags are reset and the cutoff applies. The caller holds eventMu.
func (b *Bot) observeLogin(client waClient, evt any) {
	switch evt.(type) {
	case *events.Message, *events.OfflineSyncPreview, *events.OfflineSyncCompleted:
	default:
		return
	}
	login := client.LastSuccessfulConnect()
	if login.IsZero() || login.Equal(b.loginSeen) {
		return
	}
	b.loginSeen = login
	b.synced, b.resumeSynced = false, false
}

// trackSync keeps the offline-sync flag in step with the connection. A drop or
// terminal event forgets it, so the next Connected is not mistaken for a
// finished sync. A keepalive timeout keeps the connection, so its sync is
// remembered for KeepAliveRestored. The caller holds eventMu.
func (b *Bot) trackSync(evt any) {
	switch evt.(type) {
	case *events.KeepAliveTimeout:
		b.resumeSynced = b.resumeSynced || b.synced
		b.synced = false
	case *events.KeepAliveRestored:
		if b.resumeSynced {
			b.synced, b.resumeSynced = true, false
		}
	case *events.Disconnected, *events.LoggedOut, *events.StreamReplaced, *events.TemporaryBan,
		*events.ClientOutdated, *events.ConnectFailure:
		b.synced, b.resumeSynced, b.connectedAt = false, false, time.Time{}
	}
}

// handleEvent runs on whatsmeow's dispatcher and must not block. A stopped
// generation never overwrites its successor's status: a handler that passed
// the context check before teardown finds stopped set under eventMu.
func (b *Bot) handleEvent(client waClient, evt any, signals chan<- eventEffect) {
	if b.ctx.Err() != nil {
		return
	}
	b.eventMu.Lock()
	defer b.eventMu.Unlock()
	if b.stopped {
		return
	}
	b.observeLogin(client, evt)
	b.trackSync(evt)
	eff := classifyEvent(evt, b.synced)
	now := b.deps.now()
	if eff.Connected {
		b.connectedAt, b.resumeSynced = now, false
	}
	if eff.Synced {
		b.synced, b.resumeSynced = true, false
	}
	switch {
	case eff.State == integrationstatus.WhatsAppStateTempBanned:
		var expires time.Time
		if eff.RetryAfter > 0 {
			expires = now.Add(eff.RetryAfter)
		}
		integrationstatus.MarkWhatsAppTempBan(eff.ErrorCode, expires)
	case eff.ErrorCode != "":
		integrationstatus.MarkWhatsAppError(eff.State, eff.ErrorCode)
	case eff.State != "":
		integrationstatus.SetWhatsAppState(eff.State)
	}
	if eff.State == integrationstatus.WhatsAppStateHealthy && !b.connectedAt.IsZero() {
		// The status shows the login time only while healthy.
		integrationstatus.MarkWhatsAppConnected(b.connectedAt)
	}
	if eff.State == integrationstatus.WhatsAppStateLoggedOut {
		integrationstatus.SetWhatsAppLinked(false, "")
	}
	if eff.Message {
		integrationstatus.CountWhatsAppMessage(now)
	}
	if eff.SendPresence {
		go b.sendUnavailable(client)
	}
	if eff.Healthy || eff.Terminal || eff.Reconnect {
		select {
		case signals <- eff:
		default:
			b.logger.Warn("WhatsApp event signal dropped", "state", eff.State)
		}
	}
}

// handleMessage runs on whatsmeow's handler queue, after handleEvent has counted the message.
// The filter reads the client's own JID, LID and login time without a lock, so it must run
// here, inline, and it is cheap (map lookups, at most one LID-store read). It runs under
// eventMu like handleEvent, so a stopped generation neither counts a drop nor starts work.
// Only the agent turn goes to a worker.
func (b *Bot) handleMessage(client waClient, evt *events.Message) {
	if b.ctx.Err() != nil {
		return
	}
	b.eventMu.Lock()
	defer b.eventMu.Unlock()
	if b.stopped {
		return
	}
	var cutoff time.Time
	if !b.synced && !b.resumeSynced {
		// Missed messages arrive after the login and before OfflineSyncCompleted on the
		// same queue, and whatsmeow sets LastSuccessfulConnect before any of them. Live
		// messages (after the sync) never meet the cutoff, so local clock skew cannot
		// drop them. A keepalive timeout on the same login keeps its finished sync;
		// handleEvent (observeLogin) has already cleared the flags of an older login.
		ref := client.LastSuccessfulConnect()
		if ref.IsZero() {
			ref = b.deps.now()
		}
		cutoff = ref.Add(-offlineCutoff)
	}
	in, reason := b.filter.accept(b.ctx, client, evt, cutoff)
	if reason != "" {
		integrationstatus.CountWhatsAppDrop(reason)
		return
	}
	b.workers.Add(1)
	go func() {
		defer b.workers.Done()
		select {
		case b.slots <- struct{}{}:
		case <-b.ctx.Done():
			return
		}
		defer func() { <-b.slots }()
		b.pipeline.handle(b.ctx, in)
	}()
}

// sendUnavailable keeps AuraGo "offline" so the phone still shows
// notifications. If no push name is known yet, PushNameSetting retries it.
func (b *Bot) sendUnavailable(client waClient) {
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()
	err := client.SendPresence(ctx, types.PresenceUnavailable)
	if err != nil && !errors.Is(err, whatsmeow.ErrNoPushName) && ctx.Err() == nil {
		b.logger.Warn("WhatsApp presence update failed", "error", maskDigits(security.Scrub(err.Error())))
	}
}

// SendTestMessage sends the manual test message to the owners only, through the
// Bot's send queue.
func (b *Bot) SendTestMessage(ctx context.Context, now time.Time) error {
	if err := b.sender.deliverToOwners(ctx, testMessageText(now), "test message"); err != nil {
		return err
	}
	integrationstatus.MarkWhatsAppTest(now)
	return nil
}

// Logout unlinks the device. With a live connection WhatsApp also removes it
// from the phone, and whatsmeow deletes the local keys. Otherwise only the
// local keys are deleted, and the result says so. The Bot is stopped either
// way.
func (b *Bot) Logout(ctx context.Context) (LogoutResult, error) {
	remote := false
	if client := b.currentClient(); client != nil && client.IsLoggedIn() {
		if err := client.Logout(ctx); err != nil {
			b.logger.Warn("WhatsApp remote unlink failed; removing the local device only", "error", maskDigits(security.Scrub(err.Error())))
		} else {
			remote = true
		}
	}
	b.Stop()
	if !remote {
		if err := removeLocalDevice(ctx, b.deps.backend, b.dataDir); err != nil {
			return LogoutResult{}, err
		}
	}
	integrationstatus.SetWhatsAppLinked(false, "")
	integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
	return LogoutResult{RemoteRemoved: remote}, nil
}

// removeLocalDevice deletes the linked device's keys without contacting
// WhatsApp. Once the row is gone and the store closed, it also deletes
// device.db and its sidecars, so no key bytes stay on disk. The next Bot or
// link session creates a fresh store.
func removeLocalDevice(ctx context.Context, b backend, dataDir string) error {
	store, err := b.open(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("open whatsapp device store: %w", err)
	}
	deleteErr := store.DeleteLocal(ctx)
	_ = store.Close()
	if deleteErr != nil {
		return deleteErr
	}
	return removeDeviceFiles(dataDir)
}
