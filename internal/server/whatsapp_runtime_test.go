package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"
	"aurago/internal/whatsapp"
)

func waitWhatsAppState(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for integrationstatus.WhatsAppStatus().State != want {
		if time.Now().After(deadline) {
			t.Fatalf("WhatsApp state = %q, want %q", integrationstatus.WhatsAppStatus().State, want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// whatsAppRuntimeServer builds a Server on an empty data dir with the given
// WhatsApp section. The real backend opens the store there; nothing is linked
// and nothing touches the network.
func whatsAppRuntimeServer(t *testing.T, mutate func(*config.Config)) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	if mutate != nil {
		mutate(cfg)
	}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), integrationCtx: context.Background()}
	s.initConfigSnapshot()
	t.Cleanup(s.stopWhatsApp)
	return s
}

// Uses the real backend on an empty data dir. The store opens, no device is
// linked, and nothing touches the network.
func TestWhatsAppRuntimeGenerationsFollowConfigAndShutdown(t *testing.T) {
	s := whatsAppRuntimeServer(t, nil)
	cfg := s.ConfigSnapshot()
	publish := func(next *config.Config) { s.CfgMu.Lock(); s.replaceConfigSnapshot(next); s.CfgMu.Unlock() }

	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil || integrationstatus.WhatsAppStatus().State != integrationstatus.WhatsAppStateDisabled {
		t.Fatal("a disabled section started a Bot")
	}
	next := cfg.Clone()
	next.WhatsApp.Enabled = true
	next.WhatsApp.Mode = config.WhatsAppModeDedicated
	publish(next)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil || integrationstatus.WhatsAppStatus().State != integrationstatus.WhatsAppStateNotConfigured {
		t.Fatal("dedicated mode without owners started a Bot")
	}
	next = next.Clone()
	next.WhatsApp.OwnerNumbers = []string{"+491701234567"}
	publish(next)
	s.configureWhatsAppBot()
	first := s.whatsAppBot.Load()
	if first == nil {
		t.Fatal("a configured section started no Bot")
	}
	waitWhatsAppState(t, integrationstatus.WhatsAppStateNotLinked)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != first {
		t.Fatal("unchanged settings started another generation")
	}
	changed := next.Clone()
	changed.WhatsApp.SelfChatPrefix = "AI: "
	publish(changed)
	if first.MatchesConfig(changed) {
		t.Fatal("publication did not revoke the old generation")
	}
	s.configureWhatsAppBot()
	if second := s.whatsAppBot.Load(); second == nil || second == first {
		t.Fatal("a changed section did not start a new generation")
	}
	egg := changed.Clone()
	egg.EggMode.Enabled = true
	publish(egg)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil {
		t.Fatal("egg mode retained a Bot")
	}
	publish(changed)
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() == nil {
		t.Fatal("leaving egg mode started no Bot")
	}
	s.stopWhatsApp()
	s.configureWhatsAppBot()
	if s.whatsAppBot.Load() != nil {
		t.Fatal("shutdown allowed a new generation")
	}
}

func TestServerWhatsAppControllerGuards(t *testing.T) {
	ctx := context.Background()
	s := whatsAppRuntimeServer(t, func(cfg *config.Config) {
		cfg.WhatsApp.Enabled = true
		cfg.WhatsApp.Mode = config.WhatsAppModeSelfChat
	})
	ctl := serverWhatsApp{s: s}
	if _, err := ctl.Link(ctx, whatsapp.LinkRequest{Method: whatsapp.LinkMethodPhone, Phone: "0170"}, true); !errors.Is(err, whatsapp.ErrInvalidPhone) {
		t.Fatalf("invalid phone = %v, want ErrInvalidPhone before any unlink", err)
	}
	if snap := ctl.LinkSnapshot(); snap.State != whatsapp.LinkStateIdle {
		t.Fatalf("snapshot without session = %+v", snap)
	}
	result, err := ctl.Logout(ctx)
	if err != nil || result.RemoteRemoved {
		t.Fatalf("logout without device = %+v, %v", result, err)
	}
	if s.whatsAppBot.Load() == nil {
		t.Fatal("logout did not start a fresh generation")
	}
	if err := ctl.SendTest(ctx, time.Now()); !errors.Is(err, whatsapp.ErrNotConnected) {
		t.Fatalf("test send without connection = %v", err)
	}
}

// Dedicated mode sends only to the owner numbers, so linking needs at least
// one. The refusal comes before the device store is touched.
func TestServerWhatsAppLinkNeedsAnOwnerInDedicatedMode(t *testing.T) {
	s := whatsAppRuntimeServer(t, func(cfg *config.Config) {
		cfg.WhatsApp.Enabled = true
		cfg.WhatsApp.Mode = config.WhatsAppModeDedicated
	})
	_, err := serverWhatsApp{s: s}.Link(context.Background(), whatsapp.LinkRequest{Method: whatsapp.LinkMethodQR}, false)
	if !errors.Is(err, whatsapp.ErrNotConfigured) {
		t.Fatalf("link without owners = %v, want ErrNotConfigured", err)
	}
	if _, statErr := os.Stat(config.WhatsAppStoreDir(s.ConfigSnapshot().Directories.DataDir)); !os.IsNotExist(statErr) {
		t.Fatalf("the refused link touched the device store: %v", statErr)
	}
	if s.whatsAppLink.Load() != nil || s.whatsAppBot.Load() != nil {
		t.Fatal("the refused link left a session or a Bot behind")
	}
}

// After a link session ends, the server either starts the Bot or, when the
// configuration no longer allows one, sets the matching state. The status
// never stays "linking".
func TestAfterWhatsAppLinkNeverLeavesLinkingStatus(t *testing.T) {
	selfChat := func(cfg *config.Config) {
		cfg.WhatsApp.Enabled = true
		cfg.WhatsApp.Mode = config.WhatsAppModeSelfChat
	}
	for _, tc := range []struct {
		name      string
		mutate    func(*config.Config)
		closed    bool
		wantState string // "" means a Bot starts
	}{
		{"config still allows a Bot", selfChat, false, ""},
		{"disabled meanwhile", func(cfg *config.Config) { selfChat(cfg); cfg.WhatsApp.Enabled = false }, false, integrationstatus.WhatsAppStateDisabled},
		{"dedicated without owners", func(cfg *config.Config) {
			selfChat(cfg)
			cfg.WhatsApp.Mode = config.WhatsAppModeDedicated
		}, false, integrationstatus.WhatsAppStateNotConfigured},
		{"egg mode", func(cfg *config.Config) { selfChat(cfg); cfg.EggMode.Enabled = true }, false, integrationstatus.WhatsAppStateDisabled},
		{"shutdown started", selfChat, true, integrationstatus.WhatsAppStateNotLinked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := whatsAppRuntimeServer(t, tc.mutate)
			s.whatsAppClosed = tc.closed
			integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateLinking)
			s.afterWhatsAppLink(0, true)
			if tc.wantState == "" {
				if s.whatsAppBot.Load() == nil {
					t.Fatal("a linked device with a valid config started no Bot")
				}
				waitWhatsAppState(t, integrationstatus.WhatsAppStateNotLinked)
				return
			}
			if s.whatsAppBot.Load() != nil {
				t.Fatal("a Bot started although the config does not allow one")
			}
			if got := integrationstatus.WhatsAppStatus().State; got != tc.wantState {
				t.Fatalf("state = %q, want %q", got, tc.wantState)
			}
		})
	}
}

// The session goroutine calls onFinish before it closes Done, while Cancel and
// Done are awaited under whatsAppLifecycleMu. The callback must therefore
// return without taking that lock; the hand-off runs once the lock is free.
func TestWhatsAppLinkFinishedCallbackDoesNotWaitForTheLifecycleLock(t *testing.T) {
	s := whatsAppRuntimeServer(t, func(cfg *config.Config) {
		cfg.WhatsApp.Enabled = true
		cfg.WhatsApp.Mode = config.WhatsAppModeSelfChat
	})
	integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateLinking)
	s.whatsAppLifecycleMu.Lock()
	returned := make(chan struct{})
	go func() { s.onWhatsAppLinkFinished(0, true); close(returned) }()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		s.whatsAppLifecycleMu.Unlock()
		t.Fatal("the link callback blocked on the lifecycle lock")
	}
	if s.whatsAppBot.Load() != nil {
		s.whatsAppLifecycleMu.Unlock()
		t.Fatal("the hand-off ran while the lifecycle lock was held")
	}
	s.whatsAppLifecycleMu.Unlock()
	waitWhatsAppState(t, integrationstatus.WhatsAppStateNotLinked)
	// afterWhatsAppLink publishes the Bot under the lock; wait for it to finish.
	s.whatsAppLifecycleMu.Lock()
	started := s.whatsAppBot.Load() != nil
	s.whatsAppLifecycleMu.Unlock()
	if !started {
		t.Fatal("the hand-off started no Bot")
	}
}

func TestHandleUpdateConfigWhatsAppValidatesAndHotReloads(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	body := fmt.Sprintf("directories:\n  data_dir: %q\nwhatsapp:\n  enabled: false\n", filepath.ToSlash(filepath.Join(root, "data")))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("01", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Vault: vault}
	defer s.stopWhatsApp()
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"whatsapp":{"owner_numbers":["01701234567"]}}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "whatsapp.owner_numbers[0]") {
		t.Fatalf("invalid number: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"whatsapp":{"enabled":true,"mode":"self_chat"}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["needs_restart"] == true {
		t.Fatalf("WhatsApp required a restart: %s", rec.Body.String())
	}
	if s.ConfigSnapshot().WhatsApp.Mode != config.WhatsAppModeSelfChat || s.whatsAppBot.Load() == nil {
		t.Fatal("the WhatsApp change was not hot-reloaded")
	}
}

// fakeWhatsAppLinkSession ends like whatsapp.LinkSession: inactive first, then
// onFinish on the session goroutine, then Done closes.
type fakeWhatsAppLinkSession struct {
	mu        sync.Mutex
	active    bool
	cancelled bool
	onFinish  func(linked bool)
	once      sync.Once
	done      chan struct{}
}

func newFakeWhatsAppLinkSession(onFinish func(bool)) *fakeWhatsAppLinkSession {
	return &fakeWhatsAppLinkSession{active: true, onFinish: onFinish, done: make(chan struct{})}
}

func (f *fakeWhatsAppLinkSession) Snapshot() whatsapp.LinkSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active {
		return whatsapp.LinkSnapshot{State: whatsapp.LinkStateWaiting}
	}
	return whatsapp.LinkSnapshot{State: whatsapp.LinkStateSuccess}
}

func (f *fakeWhatsAppLinkSession) Active() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

func (f *fakeWhatsAppLinkSession) Cancel() {
	f.mu.Lock()
	f.cancelled = true
	f.mu.Unlock()
	go f.finish(false)
}

func (f *fakeWhatsAppLinkSession) Done() <-chan struct{} { return f.done }

func (f *fakeWhatsAppLinkSession) finish(linked bool) {
	f.once.Do(func() {
		f.mu.Lock()
		f.active = false
		f.mu.Unlock()
		if !linked {
			integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateNotLinked) // like LinkSession.run
		}
		f.onFinish(linked)
		close(f.done)
	})
}

func (f *fakeWhatsAppLinkSession) wasCancelled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelled
}

// One walk through the session rules of the server: a linked device needs
// force, one session at a time, a forced relink replaces the finished session,
// and shutdown cancels a running session and waits for it.
func TestServerWhatsAppLinkSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := whatsAppRuntimeServer(t, func(cfg *config.Config) {
		cfg.WhatsApp.Enabled = true
		cfg.WhatsApp.Mode = config.WhatsAppModeSelfChat
	})
	ctl := serverWhatsApp{s: s}
	linked := false
	var started []*fakeWhatsAppLinkSession
	originalLinked, originalStart := whatsAppDeviceLinked, whatsAppStartLink
	t.Cleanup(func() { whatsAppDeviceLinked, whatsAppStartLink = originalLinked, originalStart })
	whatsAppDeviceLinked = func(context.Context, string) (bool, error) { return linked, nil }
	whatsAppStartLink = func(_ context.Context, _ *config.Config, _ whatsapp.LinkRequest, _ *slog.Logger, onFinish func(bool)) (whatsAppLinkSession, error) {
		session := newFakeWhatsAppLinkSession(onFinish)
		started = append(started, session)
		integrationstatus.SetWhatsAppState(integrationstatus.WhatsAppStateLinking) // like StartLink
		return session, nil
	}
	qr := whatsapp.LinkRequest{Method: whatsapp.LinkMethodQR}

	// A linked device is refused without force: 409 already_linked, no session.
	linked = true
	_, err := ctl.Link(ctx, qr, false)
	if code, reason := whatsAppErrorStatus(err); !errors.Is(err, whatsapp.ErrAlreadyLinked) || code != http.StatusConflict || reason != "already_linked" {
		t.Fatalf("linked without force = %v (%d %s)", err, code, reason)
	}
	if len(started) != 0 {
		t.Fatal("a refused link started a session")
	}

	// An unlinked device starts one session; without force a second request is
	// refused.
	linked = false
	snap, err := ctl.Link(ctx, qr, false)
	if err != nil || snap.State != whatsapp.LinkStateWaiting || len(started) != 1 {
		t.Fatalf("first link = %+v, %v, sessions=%d", snap, err, len(started))
	}
	if _, err := ctl.Link(ctx, qr, false); !errors.Is(err, whatsapp.ErrLinkInProgress) || len(started) != 1 {
		t.Fatalf("second link without force = %v, sessions=%d, want ErrLinkInProgress", err, len(started))
	}

	// Polling the session never waits for the lifecycle lock.
	s.whatsAppLifecycleMu.Lock()
	polled := make(chan whatsapp.LinkSnapshot, 1)
	go func() { polled <- ctl.LinkSnapshot() }()
	select {
	case got := <-polled:
		if got.State != whatsapp.LinkStateWaiting {
			t.Errorf("polled snapshot = %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Error("LinkSnapshot waited for the lifecycle lock")
	}
	s.whatsAppLifecycleMu.Unlock()

	// Force while a session runs (QR code to phone code) cancels it, waits for
	// its cleanup and starts a new one. The status stays "linking".
	if _, err := ctl.Link(ctx, qr, true); err != nil || len(started) != 2 {
		t.Fatalf("forced link over a running session = %v, sessions=%d", err, len(started))
	}
	select {
	case <-started[0].Done():
	default:
		t.Fatal("the forced link did not wait for the old session")
	}
	if !started[0].wasCancelled() || started[0].Active() || !started[1].Active() {
		t.Fatal("the forced link did not cancel the old session and start the new one")
	}
	if current := s.whatsAppLink.Load(); current == nil || current.whatsAppLinkSession != whatsAppLinkSession(started[1]) {
		t.Fatal("the forced link did not replace the session")
	}
	if state := integrationstatus.WhatsAppStatus().State; state != integrationstatus.WhatsAppStateLinking {
		t.Fatalf("state after the forced link = %q, want linking", state)
	}
	// The old session's own hand-off (asynchronous, from the cancel) is stale. It
	// must not touch the new session or the status, however late it runs.
	s.afterWhatsAppLink(1, false)
	if state := integrationstatus.WhatsAppStatus().State; state != integrationstatus.WhatsAppStateLinking || s.whatsAppBot.Load() != nil || s.activeWhatsAppLink() == nil {
		t.Fatalf("a stale finish disturbed the new session: state=%q bot=%v", state, s.whatsAppBot.Load())
	}

	// The second session links the device; the server then runs a Bot for it.
	linked = true
	started[1].finish(true)
	deadline := time.Now().Add(5 * time.Second)
	for s.whatsAppBot.Load() == nil {
		if time.Now().After(deadline) {
			t.Fatal("a finished link started no Bot")
		}
		time.Sleep(2 * time.Millisecond)
	}
	// A late linked finish of the replaced session would force a new Bot
	// generation. It is ignored.
	first := s.whatsAppBot.Load()
	s.afterWhatsAppLink(1, true)
	if s.whatsAppBot.Load() != first {
		t.Fatal("a stale finish replaced the Bot")
	}

	// Force unlinks the device and replaces the finished session with a new one.
	if _, err := ctl.Link(ctx, qr, true); err != nil || len(started) != 3 {
		t.Fatalf("forced relink = %v, sessions=%d", err, len(started))
	}
	if current := s.whatsAppLink.Load(); current == nil || current.whatsAppLinkSession != whatsAppLinkSession(started[2]) {
		t.Fatal("the forced relink did not replace the session")
	}

	// Shutdown cancels the running session and waits until it is done.
	stopped := make(chan struct{})
	go func() { s.stopWhatsApp(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stopWhatsApp did not return")
	}
	select {
	case <-started[2].Done():
	default:
		t.Fatal("stopWhatsApp returned before the session was done")
	}
	if !started[2].wasCancelled() || s.whatsAppBot.Load() != nil {
		t.Fatal("shutdown left the session or the Bot running")
	}
	if _, err := ctl.Link(ctx, qr, false); !errors.Is(err, whatsapp.ErrNotConfigured) {
		t.Fatalf("link after shutdown = %v", err)
	}
}

// The remote unlink runs under the lifecycle lock, so it gets a short deadline.
// When it fails, the keys are still removed with the request's own context.
func TestServerWhatsAppLogoutBoundsTheRemoteUnlink(t *testing.T) {
	s := whatsAppRuntimeServer(t, func(cfg *config.Config) {
		cfg.WhatsApp.Enabled = true
		cfg.WhatsApp.Mode = config.WhatsAppModeSelfChat
	})
	s.configureWhatsAppBot()
	var deadline time.Time
	var bounded bool
	original := whatsAppBotLogout
	t.Cleanup(func() { whatsAppBotLogout = original })
	whatsAppBotLogout = func(ctx context.Context, bot *whatsapp.Bot) (whatsapp.LogoutResult, error) {
		deadline, bounded = ctx.Deadline()
		bot.Stop() // the real Logout stops the Bot before it removes the keys
		return whatsapp.LogoutResult{}, errors.New("open whatsapp device store: context deadline exceeded")
	}
	begin := time.Now()
	result, err := serverWhatsApp{s: s}.Logout(context.Background())
	if err != nil || result.RemoteRemoved {
		t.Fatalf("logout after a failed remote unlink = %+v, %v", result, err)
	}
	if !bounded || deadline.After(begin.Add(whatsAppRemoteLogoutTimeout+time.Second)) {
		t.Fatalf("remote unlink deadline = %v (bounded=%v), want within %v", deadline, bounded, whatsAppRemoteLogoutTimeout)
	}
	if s.whatsAppBot.Load() == nil {
		t.Fatal("logout did not start a fresh generation")
	}
}
