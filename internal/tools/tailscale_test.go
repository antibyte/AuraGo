package tools

import (
	"strings"
	"testing"
)

func TestTailscaleSetRoutesRespectsDirectReadOnly(t *testing.T) {
	cfg := TailscaleConfig{ReadOnly: true}

	got := TailscaleSetRoutes(cfg, "node-1234567890", []string{"192.168.1.0/24"}, true)
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(strings.ToLower(got), "read-only") {
		t.Fatalf("expected read-only error, got %s", got)
	}
}
