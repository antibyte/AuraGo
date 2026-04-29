package telnyx

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchSMSReadOnlyBlocksSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &config.Config{}
	cfg.Telnyx.ReadOnly = true
	cfg.Telnyx.APIKey = "key"
	cfg.Telnyx.PhoneNumber = "+15550001000"

	result := DispatchSMS(ctx, "send", "+15550002000", "hello", "", nil, cfg, nil)
	if !strings.Contains(result, "read-only") {
		t.Fatalf("DispatchSMS = %s, want read-only denial", result)
	}
}

func TestDispatchCallReadOnlyBlocksMutations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &config.Config{}
	cfg.Telnyx.ReadOnly = true
	cfg.Telnyx.APIKey = "key"
	cfg.Telnyx.PhoneNumber = "+15550001000"
	cfg.Telnyx.ConnectionID = "conn-1"

	result := DispatchCall(ctx, "initiate", "+15550002000", "", "", "", 0, 0, cfg, nil)
	if !strings.Contains(result, "read-only") {
		t.Fatalf("DispatchCall = %s, want read-only denial", result)
	}
}
