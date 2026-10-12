package tools

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

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
