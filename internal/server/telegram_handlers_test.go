package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
)

func TestTelegramStatusReturnsSanitizedRuntimeSnapshot(t *testing.T) {
	integrationstatus.SetTelegramConfigured(true, true)
	integrationstatus.MarkTelegramPollFailure("telegram_poll_timeout", time.Now())
	rec := httptest.NewRecorder()
	handleTelegramStatus(&Server{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/telegram/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status response = %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Telegram integrationstatus.TelegramRuntimeStatus `json:"telegram"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if response.Telegram.LastErrorCode != "telegram_poll_timeout" || response.Telegram.ConsecutivePollErrors != 1 {
		t.Fatalf("telegram status = %#v", response.Telegram)
	}
}

func TestTelegramTestRateLimitAllowsExactlyOneInvocation(t *testing.T) {
	originalNow := telegramTestNow
	originalSend := telegramTestSend
	originalLast := telegramTestLast
	t.Cleanup(func() {
		telegramTestNow = originalNow
		telegramTestSend = originalSend
		telegramTestLast = originalLast
	})
	fixed := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	telegramTestNow = func() time.Time { return fixed }
	telegramTestLast = time.Time{}
	var calls int
	telegramTestSend = func(*config.Config, time.Time) error {
		calls++
		return nil
	}
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	first := httptest.NewRecorder()
	handleTelegramTest(s).ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/telegram/test", nil))
	second := httptest.NewRecorder()
	handleTelegramTest(s).ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/telegram/test", nil))
	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests {
		t.Fatalf("responses = %d/%d, want 200/429", first.Code, second.Code)
	}
	if calls != 1 {
		t.Fatalf("send invocations = %d, want 1", calls)
	}
}
