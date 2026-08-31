package integrationstatus

import (
	"sync"
	"time"
)

// TelegramRuntimeStatus is a sanitized passive snapshot. It never contains the
// bot token, chat content, usernames, or raw provider errors.
type TelegramRuntimeStatus struct {
	Enabled               bool   `json:"enabled"`
	Configured            bool   `json:"configured"`
	Polling               bool   `json:"polling"`
	State                 string `json:"state"`
	ConsecutivePollErrors int    `json:"consecutive_poll_errors"`
	LastErrorCode         string `json:"last_error_code,omitempty"`
	LastSuccessfulPollAt  string `json:"last_successful_poll_at,omitempty"`
	LastPollErrorAt       string `json:"last_poll_error_at,omitempty"`
	LastTestAt            string `json:"last_test_at,omitempty"`
}

var telegramRuntime = struct {
	sync.RWMutex
	status TelegramRuntimeStatus
}{status: TelegramRuntimeStatus{State: "disabled"}}

func SetTelegramConfigured(enabled, configured bool) {
	telegramRuntime.Lock()
	defer telegramRuntime.Unlock()
	telegramRuntime.status.Enabled = enabled
	telegramRuntime.status.Configured = configured
	telegramRuntime.status.Polling = false
	telegramRuntime.status.ConsecutivePollErrors = 0
	telegramRuntime.status.LastErrorCode = ""
	if !enabled {
		telegramRuntime.status.State = "disabled"
	} else if !configured {
		telegramRuntime.status.State = "not_configured"
	} else {
		telegramRuntime.status.State = "starting"
	}
}

func MarkTelegramPolling() {
	telegramRuntime.Lock()
	defer telegramRuntime.Unlock()
	telegramRuntime.status.Polling = true
	if telegramRuntime.status.State != "degraded" {
		telegramRuntime.status.State = "starting"
	}
}

func MarkTelegramPollSuccess(now time.Time) {
	telegramRuntime.Lock()
	defer telegramRuntime.Unlock()
	telegramRuntime.status.Polling = true
	telegramRuntime.status.State = "healthy"
	telegramRuntime.status.ConsecutivePollErrors = 0
	telegramRuntime.status.LastErrorCode = ""
	telegramRuntime.status.LastSuccessfulPollAt = now.UTC().Format(time.RFC3339)
}

func MarkTelegramPollFailure(code string, now time.Time) int {
	telegramRuntime.Lock()
	defer telegramRuntime.Unlock()
	telegramRuntime.status.Polling = true
	telegramRuntime.status.State = "degraded"
	telegramRuntime.status.ConsecutivePollErrors++
	telegramRuntime.status.LastErrorCode = code
	telegramRuntime.status.LastPollErrorAt = now.UTC().Format(time.RFC3339)
	return telegramRuntime.status.ConsecutivePollErrors
}

func MarkTelegramUnavailable(code string, now time.Time) {
	telegramRuntime.Lock()
	defer telegramRuntime.Unlock()
	telegramRuntime.status.Polling = false
	telegramRuntime.status.State = "unavailable"
	telegramRuntime.status.LastErrorCode = code
	telegramRuntime.status.LastPollErrorAt = now.UTC().Format(time.RFC3339)
}

func MarkTelegramTest(now time.Time) {
	telegramRuntime.Lock()
	defer telegramRuntime.Unlock()
	telegramRuntime.status.LastTestAt = now.UTC().Format(time.RFC3339)
}

func TelegramStatus() TelegramRuntimeStatus {
	telegramRuntime.RLock()
	defer telegramRuntime.RUnlock()
	return telegramRuntime.status
}
