package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchWebDAVReadOnlyUsesCanonicalConfigKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.WebDAV.Enabled = true
	cfg.WebDAV.ReadOnly = true

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "webdav",
		Operation: "write",
		Path:      "notes.txt",
		Content:   "hello",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected webdav operation to be handled")
	}
	if !strings.Contains(output, "webdav.readonly") {
		t.Fatalf("output = %s, want canonical webdav.readonly key", output)
	}
	if strings.Contains(output, "webdav.read_only") {
		t.Fatalf("output = %s, should not mention stale webdav.read_only key", output)
	}
}
