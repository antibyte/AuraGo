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

// Not parallel: the owner sender is process-wide state.
func TestSendNotificationReachesWhatsAppAndJoinsAll(t *testing.T) {
	t.Cleanup(func() { SetWhatsAppOwnerSender(nil) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := whatsAppToolConfig(true, false)
	const sent = `"channel":"whatsapp","status":"sent"`

	fake := &scriptedWhatsAppSender{ready: true}
	SetWhatsAppOwnerSender(fake)
	result := SendNotification(cfg, logger, "whatsapp", "T", "m", "normal", nil)
	if fake.calls != 1 || fake.text != "**T**\nm" || fake.file != "" {
		t.Fatalf("sender got calls=%d text=%q file=%q", fake.calls, fake.text, fake.file)
	}
	if !strings.Contains(result, sent) {
		t.Fatalf("result = %s", result)
	}

	// Read-only sends nothing and says so.
	fake.calls = 0
	result = SendNotification(whatsAppToolConfig(true, true), logger, "whatsapp", "T", "m", "normal", nil)
	if fake.calls != 0 || !strings.Contains(result, `"detail":"whatsapp is in read-only mode"`) {
		t.Fatalf("read-only: calls=%d result=%s", fake.calls, result)
	}
	result = SendNotification(whatsAppToolConfig(false, false), logger, "whatsapp", "T", "m", "normal", nil)
	if fake.calls != 0 || !strings.Contains(result, "whatsapp is not enabled") {
		t.Fatalf("disabled: calls=%d result=%s", fake.calls, result)
	}

	// "all" includes WhatsApp while it is enabled and a linked device runs, and reports a
	// read-only WhatsApp as an error entry, as it does for Telegram.
	result = SendNotification(cfg, logger, "all", "T", "m", "normal", nil)
	if fake.calls != 1 || !strings.Contains(result, sent) {
		t.Fatalf("all: calls=%d result=%s", fake.calls, result)
	}
	fake.calls = 0
	result = SendNotification(whatsAppToolConfig(true, true), logger, "all", "T", "m", "normal", nil)
	if fake.calls != 0 || !strings.Contains(result, `"detail":"whatsapp is in read-only mode"`) {
		t.Fatalf("all read-only: calls=%d result=%s", fake.calls, result)
	}

	// Not available: "all" has no WhatsApp entry, and a direct send reports it is not running.
	fake.ready = false
	result = SendNotification(cfg, logger, "all", "T", "m", "normal", nil)
	if strings.Contains(result, "whatsapp") || fake.calls != 0 {
		t.Fatalf("all without a linked device: calls=%d result=%s", fake.calls, result)
	}
	result = SendNotification(cfg, logger, "whatsapp", "T", "m", "normal", nil)
	if !strings.Contains(result, "not linked or not running") {
		t.Fatalf("direct send without a linked device: %s", result)
	}
}

// Not parallel: the owner sender is process-wide state.
func TestSendNotificationRefusesAnOversizeWhatsAppText(t *testing.T) {
	t.Cleanup(func() { SetWhatsAppOwnerSender(nil) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := whatsAppToolConfig(true, false)
	fake := &scriptedWhatsAppSender{ready: true}
	SetWhatsAppOwnerSender(fake)
	const tooLong = `"detail":"whatsapp message is too long`

	// A one-rune title, the newline and the message fill exactly one message; runes count, not bytes.
	fits := strings.Repeat("ä", WhatsAppMaxMessageRunes-2)
	if result := SendNotification(cfg, logger, "whatsapp", "T", fits, "normal", nil); fake.calls != 1 || strings.Contains(result, tooLong) {
		t.Fatalf("a text that fits was refused: calls=%d result=%.200s", fake.calls, result)
	}

	fake.calls = 0
	over := fits + "ä"
	for _, channel := range []string{"whatsapp", "all"} {
		result := SendNotification(cfg, logger, channel, "T", over, "normal", nil)
		if fake.calls != 0 || !strings.Contains(result, tooLong) || !strings.Contains(result, `"channel":"whatsapp","status":"error"`) {
			t.Fatalf("%s: calls=%d result=%.300s", channel, fake.calls, result)
		}
	}
}

// Not parallel: the owner sender is process-wide state.
func TestSendNotificationKeepsTheWhatsAppTitleOnOneBoldLine(t *testing.T) {
	t.Cleanup(func() { SetWhatsAppOwnerSender(nil) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := &scriptedWhatsAppSender{ready: true}
	SetWhatsAppOwnerSender(fake)
	for title, want := range map[string]string{
		"Backup\nfinished": "**Backup finished**\nm",
		"a\r\n**b**\n\n c": "**a b c**\nm",
		"***":              "**" + defaultNotificationTitle + "**\nm",
		"  padded  ":       "**padded**\nm",
		"Ünïcode ✓ stays":  "**Ünïcode ✓ stays**\nm",
	} {
		SendNotification(whatsAppToolConfig(true, false), logger, "whatsapp", title, "m", "normal", nil)
		if fake.text != want {
			t.Errorf("title %q: sender got %q, want %q", title, fake.text, want)
		}
	}
}
