package tools

import (
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
