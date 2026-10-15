package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"
)

type telegramRuntimeTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (tr telegramRuntimeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "api.telegram.org" {
		return tr.base.RoundTrip(r)
	}
	r = r.Clone(r.Context())
	r.URL.Scheme = tr.target.Scheme
	r.URL.Host = tr.target.Host
	return tr.base.RoundTrip(r)
}

func waitTelegramRuntime(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("Telegram runtime did not finish")
	}
}

func TestTelegramConfigPublicationReloadAndConcurrentShutdown(t *testing.T) {
	polls := make(chan chan struct{}, 32)
	var active, peak, initializations atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			initializations.Add(1)
			if strings.Contains(r.URL.Path, "invalid") {
				w.WriteHeader(401)
				_, _ = io.WriteString(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
				return
			}
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":11,"is_bot":true,"username":"fixture"}}`)
		case strings.HasSuffix(r.URL.Path, "/deleteWebhook"):
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			cancelled := make(chan struct{})
			polls <- cancelled
			<-r.Context().Done()
			active.Add(-1)
			close(cancelled)
		default:
			t.Errorf("unexpected Telegram method")
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(remote.Close)
	previousTransport := http.DefaultTransport
	target, _ := url.Parse(remote.URL)
	base := http.DefaultTransport.(*http.Transport).Clone()
	http.DefaultTransport = telegramRuntimeTransport{target, base}
	t.Cleanup(func() { http.DefaultTransport = previousTransport; base.CloseIdleConnections() })
	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), integrationCtx: context.Background()}
	s.initConfigSnapshot()
	t.Cleanup(s.stopTelegramBot)
	publish := func(next *config.Config) { s.CfgMu.Lock(); s.replaceConfigSnapshot(next); s.CfgMu.Unlock() }
	nextPoll := func() chan struct{} {
		select {
		case p := <-polls:
			return p
		case <-time.After(5 * time.Second):
			t.Fatal("poll did not start")
			return nil
		}
	}
	s.configureTelegramBot()
	if s.telegramBot.Load() != nil {
		t.Fatal("empty settings started Telegram")
	}
	cfg = cfg.Clone()
	cfg.Telegram.BotToken = "first"
	cfg.Telegram.UserID = 7
	publish(cfg)
	s.configureTelegramBot()
	p := nextPoll()
	first := s.telegramBot.Load()
	s.configureTelegramBot()
	if s.telegramBot.Load() != first {
		t.Fatal("unchanged settings started another generation")
	}
	for _, change := range []string{"user", "workers", "token", "egg", "resume", "remove", "invalid", "valid"} {
		next := cfg.Clone()
		switch change {
		case "user":
			next.Telegram.UserID++
		case "workers":
			next.Telegram.MaxConcurrentWorkers = 2
		case "token":
			next.Telegram.BotToken = "rotated"
		case "egg":
			next.EggMode.Enabled = true
		case "resume":
			next.EggMode.Enabled = false
		case "remove":
			next.Telegram.BotToken = ""
		case "invalid":
			next.Telegram.BotToken = "invalid"
		case "valid":
			next.Telegram.BotToken = "valid"
		}
		publish(next)
		if p != nil {
			waitTelegramRuntime(t, p)
			p = nil
		}
		s.configureTelegramBot()
		cfg = next
		if change == "egg" || change == "remove" {
			if s.telegramBot.Load() != nil {
				t.Fatalf("%s retained a bot", change)
			}
		} else if change == "invalid" {
			// Initialization failure must not reactivate the previous token.
			deadline := time.Now().Add(5 * time.Second)
			for integrationstatus.TelegramStatus().State != "unavailable" && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if status := integrationstatus.TelegramStatus(); status.LastErrorCode != "telegram_init_failed" {
				t.Fatalf("invalid token status: %+v", status)
			}
			select {
			case <-polls:
				t.Fatal("invalid token started a poller")
			default:
			}
		} else {
			p = nextPoll()
		}
	}
	if initializations.Load() != 7 {
		t.Fatalf("unexpected generations: %d", initializations.Load())
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				s.stopTelegramBot()
			} else {
				next := cfg.Clone()
				next.Telegram.MaxConcurrentWorkers = i + 3
				publish(next)
				s.configureTelegramBot()
			}
		}(i)
	}
	wg.Wait()
	waitTelegramRuntime(t, p)
	for len(polls) > 0 {
		waitTelegramRuntime(t, <-polls)
	}
	initializationsAfterShutdown := initializations.Load()
	publish(cfg)
	s.configureTelegramBot()
	if s.telegramBot.Load() != nil || active.Load() != 0 || peak.Load() != 1 {
		t.Fatalf("shutdown/overlap: active=%d peak=%d", active.Load(), peak.Load())
	}
	if initializations.Load() != initializationsAfterShutdown {
		t.Fatal("shutdown allowed initialization")
	}
}

func TestHandleUpdateConfigTelegramChangeDoesNotRequireFullRestart(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte("telegram:\n  telegram_user_id: 0\n"), 0600); err != nil {
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
	defer s.stopTelegramBot()
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"telegram":{"telegram_user_id":7,"max_concurrent_workers":2}}`)))
	var result map[string]any
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["needs_restart"] == true {
		t.Fatalf("Telegram required restart: %s", rec.Body.String())
	}
	if s.ConfigSnapshot().Telegram.UserID != 7 || s.ConfigSnapshot().Telegram.MaxConcurrentWorkers != 2 {
		t.Fatal("Telegram settings were not published")
	}
}
