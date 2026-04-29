package tools

import (
	"strings"
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

func TestJellyfinReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := config.JellyfinConfig{ReadOnly: true, AllowDestructive: true}

	for name, got := range map[string]string{
		"playback_control": JellyfinPlaybackControl(cfg, nil, "session-1", "pause", nil),
		"library_refresh":  JellyfinLibraryRefresh(cfg, nil, "library-1", nil),
		"delete_item":      JellyfinDeleteItem(cfg, nil, "item-1", nil),
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, "read-only mode") {
				t.Fatalf("response = %s, want read-only denial", got)
			}
		})
	}
}

func TestJellyfinDeleteItemRequiresAllowDestructive(t *testing.T) {
	cfg := config.JellyfinConfig{ReadOnly: false, AllowDestructive: false}

	got := JellyfinDeleteItem(cfg, nil, "item-1", nil)
	if !strings.Contains(got, "allow_destructive") {
		t.Fatalf("response = %s, want allow_destructive denial", got)
	}
}
