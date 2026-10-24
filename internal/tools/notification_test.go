package tools

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestSendNotificationRespectsDiscordReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	cfg.Discord.ReadOnly = true
	cfg.Discord.DefaultChannelID = "channel-1"

	called := false
	result := SendNotification(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		"discord",
		"Deploy",
		"completed",
		"normal",
		func(channelID, content string) error {
			called = true
			return nil
		},
	)

	if called {
		t.Fatal("discord send function was called in read-only mode")
	}
	if !strings.Contains(result, "read-only") {
		t.Fatalf("SendNotification = %s, want read-only denial", result)
	}
}

func TestSendNotificationRespectsTelnyxReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telnyx.Enabled = true
	cfg.Telnyx.ReadOnly = true
	cfg.Telnyx.PhoneNumber = "+15550001000"
	cfg.Telnyx.AllowedNumbers = []string{"+15550002000"}

	called := false
	result := SendNotification(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		"telnyx",
		"Deploy",
		"completed",
		"normal",
		nil,
		func(to, message string) error {
			called = true
			return nil
		},
	)

	if called {
		t.Fatal("telnyx send function was called in read-only mode")
	}
	if !strings.Contains(result, "read-only") {
		t.Fatalf("SendNotification = %s, want read-only denial", result)
	}
}

// The ntfy topic works like a password: a failed send must not put the topic URL into the
// returned error, the SendNotification result or the log (http.Client.Do and url.Parse
// return a *url.Error that prints the whole URL).
func TestNtfyFailureKeepsTheTopicOutOfErrorsAndLogs(t *testing.T) {
	const topic = "secret-topic-7c41"
	cfg := &config.Config{}
	cfg.Notifications.Ntfy.Enabled = true
	cfg.Notifications.Ntfy.Topic = topic
	for _, base := range []string{"http://127.0.0.1:1", "http://bad host"} {
		cfg.Notifications.Ntfy.URL = base
		var logs bytes.Buffer
		result := SendNotification(cfg, slog.New(slog.NewTextHandler(&logs, nil)), "ntfy", "T", "B", "high", nil)
		if !strings.Contains(result, `"status":"error"`) || !strings.Contains(logs.String(), "Notification failed") {
			t.Fatalf("%s: result %s, logs %s; want a logged failure", base, result, logs.String())
		}
		if strings.Contains(result, topic) || strings.Contains(logs.String(), topic) {
			t.Fatalf("%s: the topic leaked: result %s, logs %s", base, result, logs.String())
		}
		if err := sendNtfy(cfg, "T", "B", "high"); err == nil || strings.Contains(err.Error(), topic) {
			t.Fatalf("%s: sendNtfy error = %v", base, err)
		}
	}
}

// A flood-limited Telegram API must not block the caller for the announced
// retry window: the whole send stays inside telegramSendTotalBudget.
func TestSendTelegramNotificationStopsAtTheRetryBudget(t *testing.T) {
	// The shipped default has to bound the blocking time on its own.
	if telegramSendTotalBudget <= 0 || telegramSendTotalBudget > time.Minute {
		t.Fatalf("default send budget = %v, want at most one minute", telegramSendTotalBudget)
	}
	old := telegramSendTotalBudget
	telegramSendTotalBudget = 300 * time.Millisecond
	t.Cleanup(func() { telegramSendTotalBudget = old })

	var requests atomic.Int32
	tg04API(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"ok":false,"description":"Too Many Requests","parameters":{"retry_after":60}}`)
	})
	cfg, _ := tg04Config(t, "123:secret")

	start := time.Now()
	err := sendTelegramNotification(cfg, "t", "m")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a flood-limited send must fail")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("send error = %v, want the telegram status", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("send blocked for %v, want the retry budget to stop it", elapsed)
	}
	if got := requests.Load(); got > 2 {
		t.Fatalf("requests = %d, want at most 2", got)
	}
}
