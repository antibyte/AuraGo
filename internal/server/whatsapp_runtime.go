package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/whatsapp"
)

// whatsAppRemoteLogoutTimeout bounds the remote unlink, which runs under
// whatsAppLifecycleMu and so stalls config saves while it waits.
const whatsAppRemoteLogoutTimeout = 10 * time.Second

// whatsAppLinkSession is what the server needs from a link session.
// *whatsapp.LinkSession satisfies it; tests substitute a fake.
type whatsAppLinkSession interface {
	Snapshot() whatsapp.LinkSnapshot
	Active() bool
	Cancel()
	Done() <-chan struct{}
}

// whatsAppLinkRef lets an atomic.Pointer hold the interface.
type whatsAppLinkRef struct{ whatsAppLinkSession }

// Test seams, like telegramTestSend. Production keeps the real implementations.
var (
	whatsAppDeviceLinked = whatsapp.DeviceLinked
	whatsAppStartLink    = func(parent context.Context, cfg *config.Config, req whatsapp.LinkRequest, logger *slog.Logger, onFinish func(linked bool)) (whatsAppLinkSession, error) {
		session, err := whatsapp.StartLink(parent, cfg, req, logger, onFinish)
		if err != nil {
			return nil, err
		}
		return session, nil
	}
	whatsAppBotLogout = func(ctx context.Context, bot *whatsapp.Bot) (whatsapp.LogoutResult, error) {
		return bot.Logout(ctx)
	}
)

// whatsAppController is what the WhatsApp HTTP handlers need from the
// runtime. serverWhatsApp is the production implementation. Handler tests
// set Server.whatsAppCtl to a fake.
type whatsAppController interface {
	Link(ctx context.Context, req whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error)
	LinkSnapshot() whatsapp.LinkSnapshot
	Logout(ctx context.Context) (whatsapp.LogoutResult, error)
	SendTest(ctx context.Context, now time.Time) error
}

func (s *Server) whatsApp() whatsAppController {
	if s.whatsAppCtl != nil {
		return s.whatsAppCtl
	}
	return serverWhatsApp{s: s}
}

// configureWhatsAppBot publishes the Bot generation for the current config.
func (s *Server) configureWhatsAppBot() {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	s.configureWhatsAppBotLocked(false)
}

// configureWhatsAppBotLocked replaces the generation unless it still matches.
// force replaces it anyway, because a newly linked device needs a fresh
// connection. While a link session owns the device, no Bot starts. The
// caller holds whatsAppLifecycleMu.
func (s *Server) configureWhatsAppBotLocked(force bool) {
	if s.whatsAppClosed || s.activeWhatsAppLink() != nil {
		return
	}
	s.CfgMu.RLock()
	current := s.whatsAppBot.Load()
	unchanged := !force && current != nil && current.MatchesConfig(s.ConfigSnapshot())
	s.CfgMu.RUnlock()
	if unchanged {
		return
	}
	if previous := s.whatsAppBot.Swap(nil); previous != nil {
		previous.Stop()
	}
	// Publish under CfgMu so a concurrent config change cannot miss revoking it.
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	s.whatsAppBot.Store(whatsapp.Start(parent, s.ConfigSnapshot(), s.Logger, s.whatsAppAgentDeps()))
}

// whatsAppAgentDeps hands the Bot the collaborators configureTelegramBot passes to the
// Telegram bot, as one struct. The knowledge graph is left out: Telegram runs the agent
// without it. Budget tracker and config/client are read per message, so a config reload
// reaches a running generation. The caller holds CfgMu.RLock; this reads fields only and
// takes no lock itself.
func (s *Server) whatsAppAgentDeps() whatsapp.AgentDeps {
	return whatsapp.AgentDeps{
		LLMClient:        s.LLMClient,
		ShortTermMem:     s.ShortTermMem,
		LongTermMem:      s.LongTermMem,
		Vault:            s.Vault,
		Registry:         s.Registry,
		CronManager:      s.CronManager,
		HistoryManager:   s.HistoryManager,
		InventoryDB:      s.InventoryDB,
		MissionManagerV2: s.MissionManagerV2,
		RemoteHub:        s.RemoteHub,
		Guardian:         s.Guardian,
		BudgetTracker:    s.budgetTrackerSnapshot,
		WebhookManager:   s.WebhookManager,
		Snapshot:         s.chatRuntimeSnapshot,
		RoutingDecisions: taskDecisionService{s},
	}
}

// stopWhatsApp ends the link session and the Bot at shutdown. Nothing
// restarts afterwards.
func (s *Server) stopWhatsApp() {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	s.whatsAppClosed = true
	s.cancelWhatsAppLinkLocked()
	if previous := s.whatsAppBot.Swap(nil); previous != nil {
		previous.Stop()
	}
}

// cancelWhatsAppLinkLocked cancels a running link session and waits for its
// cleanup. The session calls onWhatsAppLinkFinished before it closes Done, so
// that callback must never wait for whatsAppLifecycleMu, which the caller
// holds here. The caller holds whatsAppLifecycleMu.
func (s *Server) cancelWhatsAppLinkLocked() {
	if link := s.activeWhatsAppLink(); link != nil {
		link.Cancel()
		<-link.Done()
	}
}

// activeWhatsAppLink returns the running link session, or nil. It reads
// lock-free; only writes need whatsAppLifecycleMu.
func (s *Server) activeWhatsAppLink() whatsAppLinkSession {
	if ref := s.whatsAppLink.Load(); ref != nil && ref.Active() {
		return ref.whatsAppLinkSession
	}
	return nil
}

// onWhatsAppLinkFinished is the onFinish callback of the link session with the
// given generation. It runs on the session goroutine before Done closes, so it
// only hands off to a new goroutine and never takes whatsAppLifecycleMu itself.
func (s *Server) onWhatsAppLinkFinished(gen uint64, linked bool) {
	go s.afterWhatsAppLink(gen, linked)
}

// afterWhatsAppLink runs on its own goroutine when the link session of
// generation gen ends. A finish from a session that a forced link has since
// replaced is ignored: it must neither restart the Bot nor change the status of
// the current session. Otherwise a linked device always gets a fresh Bot
// generation. When the configuration no longer allows one (disabled, not
// configured, egg mode), Start publishes the matching state instead, so the
// status never stays "linking". After shutdown nothing starts, and the status
// still leaves "linking".
func (s *Server) afterWhatsAppLink(gen uint64, linked bool) {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	if gen != s.whatsAppLinkGen {
		return
	}
	s.configureWhatsAppBotLocked(linked)
	if s.whatsAppClosed && integrationstatus.WhatsAppStatus().State == integrationstatus.WhatsAppStateLinking {
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
	}
}

type serverWhatsApp struct{ s *Server }

// Link starts one link session. A running session and an existing device are
// refused unless force is set; then the session is cancelled and the device is
// unlinked first, remotely when connected. The request is validated before
// anything is unlinked or stopped.
func (w serverWhatsApp) Link(ctx context.Context, req whatsapp.LinkRequest, force bool) (whatsapp.LinkSnapshot, error) {
	if err := req.Validate(); err != nil {
		return whatsapp.LinkSnapshot{}, err
	}
	s := w.s
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	cfg := s.ConfigSnapshot()
	// Also refuses dedicated mode without an owner number: ConfigState reports
	// not_configured there, and nothing is unlinked or opened.
	if s.whatsAppClosed || whatsapp.ConfigState(cfg) != "" {
		return whatsapp.LinkSnapshot{}, whatsapp.ErrNotConfigured
	}
	if s.activeWhatsAppLink() != nil {
		if !force {
			return whatsapp.LinkSnapshot{}, whatsapp.ErrLinkInProgress
		}
		// A forced link replaces the running session (say, QR code to phone
		// code). The session's onFinish never takes the lifecycle lock, so
		// waiting for Done here is safe; its hand-off turns stale below.
		s.cancelWhatsAppLinkLocked()
	}
	linked, err := whatsAppDeviceLinked(ctx, cfg.Directories.DataDir)
	if err != nil {
		return whatsapp.LinkSnapshot{}, err
	}
	if linked && !force {
		return whatsapp.LinkSnapshot{}, whatsapp.ErrAlreadyLinked
	}
	if linked {
		if _, err := w.logoutLocked(ctx); err != nil {
			// The Bot is gone either way; start a fresh generation.
			s.configureWhatsAppBotLocked(false)
			return whatsapp.LinkSnapshot{}, err
		}
	} else if previous := s.whatsAppBot.Swap(nil); previous != nil {
		previous.Stop()
	}
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	s.whatsAppLinkGen++
	gen := s.whatsAppLinkGen
	session, err := whatsAppStartLink(parent, cfg, req, s.Logger, func(linked bool) { s.onWhatsAppLinkFinished(gen, linked) })
	if err != nil {
		s.configureWhatsAppBotLocked(false)
		return whatsapp.LinkSnapshot{}, err
	}
	s.whatsAppLink.Store(&whatsAppLinkRef{session})
	return session.Snapshot(), nil
}

// LinkSnapshot is polled by the Config UI. It reads the session lock-free, so
// a slow unlink or config save never delays it.
func (w serverWhatsApp) LinkSnapshot() whatsapp.LinkSnapshot {
	ref := w.s.whatsAppLink.Load()
	if ref == nil {
		return whatsapp.LinkSnapshot{State: whatsapp.LinkStateIdle}
	}
	return ref.Snapshot()
}

// Logout unlinks the device and starts a fresh generation, which reports
// not_linked or disabled.
func (w serverWhatsApp) Logout(ctx context.Context) (whatsapp.LogoutResult, error) {
	s := w.s
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	if s.whatsAppClosed || strings.TrimSpace(s.ConfigSnapshot().Directories.DataDir) == "" {
		return whatsapp.LogoutResult{}, whatsapp.ErrNotConfigured
	}
	result, err := w.logoutLocked(ctx)
	s.configureWhatsAppBotLocked(true)
	return result, err
}

// logoutLocked uses the running Bot's connection when there is one.
// Otherwise it deletes the local keys only. The caller holds
// whatsAppLifecycleMu.
func (w serverWhatsApp) logoutLocked(ctx context.Context) (whatsapp.LogoutResult, error) {
	s := w.s
	s.cancelWhatsAppLinkLocked()
	dataDir := s.ConfigSnapshot().Directories.DataDir
	if bot := s.whatsAppBot.Swap(nil); bot != nil {
		// The remote unlink holds the lifecycle lock, so it gets a short
		// deadline. Bot.Logout uses the same context for its local fallback,
		// which a hung remote call would leave expired; the Bot is stopped by
		// then, so the keys are removed here with the request's own context.
		remoteCtx, cancel := context.WithTimeout(ctx, whatsAppRemoteLogoutTimeout)
		defer cancel()
		result, err := whatsAppBotLogout(remoteCtx, bot)
		if err == nil {
			return result, nil
		}
	}
	if err := whatsapp.RemoveLocalDevice(ctx, dataDir); err != nil {
		return whatsapp.LogoutResult{}, err
	}
	integrationstatus.SetWhatsAppLinked(false, "")
	return whatsapp.LogoutResult{}, nil
}

func (w serverWhatsApp) SendTest(ctx context.Context, now time.Time) error {
	bot := w.s.whatsAppBot.Load()
	if bot == nil {
		return whatsapp.ErrNotConnected
	}
	return bot.SendTestMessage(ctx, now)
}
