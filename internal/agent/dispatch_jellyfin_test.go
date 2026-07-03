package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchJellyfinReadOnlyUsesCanonicalConfigKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jellyfin.Enabled = true
	cfg.Jellyfin.ReadOnly = true

	output, ok := dispatchPlatform(context.Background(), ToolCall{
		Action:    "jellyfin",
		Operation: "playback_control",
		Command:   "pause",
		Params: map[string]interface{}{
			"session_id": "session-1",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected jellyfin operation to be handled")
	}
	if !strings.Contains(output, "jellyfin.readonly") {
		t.Fatalf("output = %s, want canonical jellyfin.readonly key", output)
	}
	if strings.Contains(output, "jellyfin.read_only") {
		t.Fatalf("output = %s, should not mention stale jellyfin.read_only key", output)
	}
}
