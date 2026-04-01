package tools

import (
	"testing"
	"time"

	"aurago/internal/config"
)

func TestJellyfinRequestContextUsesConfiguredTimeout(t *testing.T) {
	ctx, cancel := jellyfinRequestContext(config.JellyfinConfig{RequestTimeout: 9})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on Jellyfin request context")
	}
	remaining := time.Until(deadline)
	if remaining < 8*time.Second || remaining > 10*time.Second {
		t.Fatalf("remaining timeout = %v, want about 9s", remaining)
	}
}

func TestJellyfinRequestContextUsesDefaultTimeout(t *testing.T) {
	ctx, cancel := jellyfinRequestContext(config.JellyfinConfig{})
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on Jellyfin request context")
	}
	remaining := time.Until(deadline)
	if remaining < defaultJellyfinRequestTimeout-time.Second || remaining > defaultJellyfinRequestTimeout+time.Second {
		t.Fatalf("remaining timeout = %v, want about %v", remaining, defaultJellyfinRequestTimeout)
	}
}
