package tools

import (
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestTrueNASPoolScrubRequiresAllowDestructive(t *testing.T) {
	cfg := config.TrueNASConfig{Enabled: true, ReadOnly: false, AllowDestructive: false}
	got := TrueNASPoolScrub(cfg, 1, slog.Default())
	if !strings.Contains(got, "allow_destructive: false") {
		t.Fatalf("TrueNASPoolScrub = %s, want allow_destructive denial", got)
	}
}
