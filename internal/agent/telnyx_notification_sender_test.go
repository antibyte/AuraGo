package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

// Notification SMS go through the configured Telnyx client, so the number
// allowlist and read-only mode apply exactly as for the telnyx_sms tool. The
// context is already cancelled: a send that slipped past the policy fails
// with "context canceled" instead of reaching the provider.
func TestTelnyxNotificationSenderUsesConfiguredClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.Telnyx.APIKey = "synthetic-test-key"
	cfg.Telnyx.PhoneNumber = "+15550000001"
	cfg.Telnyx.AllowedNumbers = []string{"+15550000002"}

	if telnyxNotificationSender(ctx, cfg, logger) != nil {
		t.Fatal("disabled Telnyx produced a notification sender")
	}
	cfg.Telnyx.Enabled = true

	send := telnyxNotificationSender(ctx, cfg, logger)
	if send == nil {
		t.Fatal("enabled Telnyx produced no notification sender")
	}
	if err := send("+15550000003", "fixture"); err == nil || !strings.Contains(err.Error(), "allowed_numbers") {
		t.Fatalf("unlisted destination: error = %v, want the allowlist refusal", err)
	}

	cfg.Telnyx.ReadOnly = true
	if err := telnyxNotificationSender(ctx, cfg, logger)("+15550000002", "fixture"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("read-only mode: error = %v, want the read-only refusal", err)
	}
}
