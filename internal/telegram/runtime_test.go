package telegram

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func awaitTelegram[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("Telegram lifecycle did not complete")
		var zero T
		return zero
	}
}

func TestTelegramReloadDrainsWorkersAndPreservesVerifiedBotCursor(t *testing.T) {
	type poll struct {
		offset    int
		reply     chan []tgbotapi.Update
		cancelled chan struct{}
	}
	polls := make(chan poll, 10)
	var active, peak atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			id := 11
			if strings.Contains(r.URL.Path, "other") {
				id = 22
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": id, "is_bot": true, "username": "fixture"}})
		case strings.HasSuffix(r.URL.Path, "/deleteWebhook"):
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			offset, _ := strconv.Atoi(r.Form.Get("offset"))
			p := poll{offset, make(chan []tgbotapi.Update, 1), make(chan struct{})}
			polls <- p
			select {
			case updates := <-p.reply:
				active.Add(-1)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": updates})
			case <-r.Context().Done():
				active.Add(-1)
				close(p.cancelled)
			}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(remote.Close)
	cfg := &config.Config{}
	cfg.Telegram.BotToken, cfg.Telegram.UserID, cfg.Telegram.MaxConcurrentWorkers = "first", 7, 1
	offsets := &UpdateOffsets{}
	started, cancelled, release := make(chan int, 4), make(chan struct{}, 4), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	start := func(token string) *Bot {
		current := cfg.Clone()
		current.Telegram.BotToken = token
		b := startLongPolling(context.Background(), current, logger, nil, offsets, remote.URL+"/bot%s/%s", remote.Client(), func(ctx context.Context, _ *tgbotapi.BotAPI, u tgbotapi.Update) {
			started <- u.UpdateID
			<-ctx.Done()
			cancelled <- struct{}{}
			<-release
		})
		t.Cleanup(b.Stop)
		return b
	}
	update := func(id int) tgbotapi.Update {
		return tgbotapi.Update{UpdateID: id, Message: &tgbotapi.Message{From: &tgbotapi.User{ID: 7}}}
	}
	b := start("first")
	p := awaitTelegram(t, polls)
	if p.offset != 0 {
		t.Fatal(p.offset)
	}
	p.reply <- []tgbotapi.Update{update(40), update(41)}
	if id := awaitTelegram(t, started); id != 40 {
		t.Fatal(id)
	}
	changed := cfg.Clone()
	changed.Telegram.BotToken = "rotated"
	b.CancelIfConfigChanged(changed)
	awaitTelegram(t, cancelled)
	stopped := make(chan struct{})
	go func() { b.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned before its worker")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	awaitTelegram(t, stopped)
	b.Stop()
	if got := offsets.get(11); got != 41 {
		t.Fatalf("cursor = %d, want accepted update 40 only", got)
	}

	b = start("rotated")
	p = awaitTelegram(t, polls)
	if p.offset != 41 {
		t.Fatalf("rotated token cursor = %d", p.offset)
	}
	p.reply <- []tgbotapi.Update{update(40), update(41)}
	if id := awaitTelegram(t, started); id != 41 {
		t.Fatalf("replayed update %d", id)
	}
	pending := awaitTelegram(t, polls)
	b.Stop()
	awaitTelegram(t, cancelled)
	awaitTelegram(t, pending.cancelled)
	if got := offsets.get(11); got != 42 {
		t.Fatal(got)
	}

	b = start("other")
	p = awaitTelegram(t, polls)
	if p.offset != 0 {
		t.Fatalf("different bot inherited cursor %d", p.offset)
	}
	b.Stop()
	awaitTelegram(t, p.cancelled)
	if peak.Load() != 1 {
		t.Fatalf("overlapping pollers: %d", peak.Load())
	}
	if status := integrationstatus.TelegramStatus(); status.ConsecutivePollErrors != 0 {
		t.Fatalf("reload counted as failure: %+v", status)
	}
}

func TestTelegramConfigRevocationCancelsInitialization(t *testing.T) {
	for _, change := range []string{"token", "remove", "user", "workers", "egg"} {
		t.Run(change, func(t *testing.T) {
			started, cancelled := make(chan struct{}), make(chan struct{})
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
				close(cancelled)
			}))
			defer remote.Close()
			cfg := &config.Config{}
			cfg.Telegram.BotToken = "fixture"
			cfg.Telegram.UserID = 7
			b := startLongPolling(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil)
			defer b.Stop()
			awaitTelegram(t, started)
			b.CancelIfConfigChanged(cfg.Clone())
			if !b.MatchesConfig(cfg) {
				t.Fatal("unchanged settings revoked bot")
			}
			next := cfg.Clone()
			switch change {
			case "token":
				next.Telegram.BotToken = "rotated"
			case "remove":
				next.Telegram.BotToken = ""
			case "user":
				next.Telegram.UserID++
			case "workers":
				next.Telegram.MaxConcurrentWorkers++
			case "egg":
				next.EggMode.Enabled = true
			}
			b.CancelIfConfigChanged(next)
			awaitTelegram(t, cancelled)
			b.Stop()
			if status := integrationstatus.TelegramStatus(); status.LastErrorCode != "" {
				t.Fatalf("cancellation became a fault: %+v", status)
			}
		})
	}
}

func TestTelegramInvalidTokenIsUnavailableAndDisabledConfigStartsNothing(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
	}))
	defer remote.Close()
	cfg := &config.Config{}
	cfg.Telegram.BotToken = "invalid"
	cfg.Telegram.UserID = 7
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := startLongPolling(context.Background(), cfg, logger, nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil)
	awaitTelegram(t, b.done)
	b.Stop()
	if status := integrationstatus.TelegramStatus(); status.State != "unavailable" || status.LastErrorCode != "telegram_init_failed" || status.ConsecutivePollErrors != 0 {
		t.Fatalf("invalid token status: %+v", status)
	}
	for _, egg := range []bool{false, true} {
		cfg.EggMode.Enabled = egg
		if !egg {
			cfg.Telegram.BotToken = ""
		} else {
			cfg.Telegram.BotToken = "fixture"
		}
		if b := startLongPolling(context.Background(), cfg, logger, nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil); b != nil {
			b.Stop()
			t.Fatal("disabled config started bot")
		}
	}
}
