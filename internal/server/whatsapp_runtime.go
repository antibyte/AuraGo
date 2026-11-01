package server

import (
	"context"
	"strings"
	"time"

	"aurago/internal/integrationstatus"
	"aurago/internal/whatsapp"
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
	if s.whatsAppClosed || (s.whatsAppLink != nil && s.whatsAppLink.Active()) {
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
	s.whatsAppBot.Store(whatsapp.Start(parent, s.ConfigSnapshot(), s.Logger))
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
	if link := s.whatsAppLink; link != nil && link.Active() {
		link.Cancel()
		<-link.Done()
	}
}

// onWhatsAppLinkFinished is the link session's onFinish callback. It runs on
// the session goroutine before Done closes, so it only hands off to a new
// goroutine and never takes whatsAppLifecycleMu itself.
func (s *Server) onWhatsAppLinkFinished(linked bool) {
	go s.afterWhatsAppLink(linked)
}

// afterWhatsAppLink runs on its own goroutine when a link session ends. A
// linked device always gets a fresh Bot generation. When the configuration no
// longer allows one (disabled, not configured, egg mode), Start publishes the
// matching state instead, so the status never stays "linking". After
// shutdown nothing starts, and the status still leaves "linking".
func (s *Server) afterWhatsAppLink(linked bool) {
	s.whatsAppLifecycleMu.Lock()
	defer s.whatsAppLifecycleMu.Unlock()
	s.configureWhatsAppBotLocked(linked)
	if s.whatsAppClosed && integrationstatus.WhatsAppStatus().State == integrationstatus.WhatsAppStateLinking {
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked)
	}
}

type serverWhatsApp struct{ s *Server }

// Link starts one link session. An existing device is refused unless force is
// set; then it is unlinked first, remotely when connected. The request is
// validated before anything is unlinked or stopped.
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
	if s.whatsAppLink != nil && s.whatsAppLink.Active() {
		return whatsapp.LinkSnapshot{}, whatsapp.ErrLinkInProgress
	}
	linked, err := whatsapp.DeviceLinked(ctx, cfg.Directories.DataDir)
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
	session, err := whatsapp.StartLink(parent, cfg, req, s.Logger, s.onWhatsAppLinkFinished)
	if err != nil {
		s.configureWhatsAppBotLocked(false)
		return whatsapp.LinkSnapshot{}, err
	}
	s.whatsAppLink = session
	return session.Snapshot(), nil
}

func (w serverWhatsApp) LinkSnapshot() whatsapp.LinkSnapshot {
	w.s.whatsAppLifecycleMu.Lock()
	defer w.s.whatsAppLifecycleMu.Unlock()
	if w.s.whatsAppLink == nil {
		return whatsapp.LinkSnapshot{State: whatsapp.LinkStateIdle}
	}
	return w.s.whatsAppLink.Snapshot()
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
	if bot := s.whatsAppBot.Swap(nil); bot != nil {
		return bot.Logout(ctx)
	}
	if err := whatsapp.RemoveLocalDevice(ctx, s.ConfigSnapshot().Directories.DataDir); err != nil {
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
