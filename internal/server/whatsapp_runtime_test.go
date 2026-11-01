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
	if s.whatsAppLink != nil || s.whatsAppBot.Load() != nil {
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
			s.afterWhatsAppLink(true)
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
	go func() { s.onWhatsAppLinkFinished(true); close(returned) }()
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
