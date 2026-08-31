package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/telegram"
)

var (
	telegramTestMu   sync.Mutex
	telegramTestLast time.Time
	telegramTestNow  = time.Now
	telegramTestSend = func(cfg *config.Config, now time.Time) error {
		return telegram.SendTestMessage(cfg, now)
	}
)

func handleTelegramStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"telegram": integrationstatus.TelegramStatus(),
		})
	}
}

func handleTelegramTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		now := telegramTestNow()
		telegramTestMu.Lock()
		if !telegramTestLast.IsZero() && now.Sub(telegramTestLast) < time.Minute {
			telegramTestMu.Unlock()
			jsonError(w, "Telegram test rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		telegramTestLast = now
		telegramTestMu.Unlock()

		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		if err := telegramTestSend(&cfg, now); err != nil {
			s.Logger.Warn("Manual Telegram test failed", "error_code", "telegram_test_failed")
			jsonError(w, "Telegram test failed", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "sent",
			"sent_at": now.UTC().Format(time.RFC3339),
		})
	}
}
