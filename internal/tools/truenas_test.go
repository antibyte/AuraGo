package tools

import (
	"testing"
	"time"

	"aurago/internal/config"
)

func TestTrueNASRequestContextUsesConfiguredTimeout(t *testing.T) {
	ctx, cancel := truenasRequestContext(config.TrueNASConfig{RequestTimeout: 7})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on TrueNAS request context")
	}
	remaining := time.Until(deadline)
	if remaining < 6*time.Second || remaining > 8*time.Second {
		t.Fatalf("remaining timeout = %v, want about 7s", remaining)
	}
}

func TestTrueNASRequestContextUsesDefaultTimeout(t *testing.T) {
	ctx, cancel := truenasRequestContext(config.TrueNASConfig{})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on TrueNAS request context")
	}
	remaining := time.Until(deadline)
	if remaining < defaultTrueNASRequestTimeout-time.Second || remaining > defaultTrueNASRequestTimeout+time.Second {
		t.Fatalf("remaining timeout = %v, want about %v", remaining, defaultTrueNASRequestTimeout)
	}
}
