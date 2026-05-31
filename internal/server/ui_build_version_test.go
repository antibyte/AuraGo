package server

import (
	"strings"
	"testing"
	"time"
)

func TestFormatUIBuildVersionIncludesStartupTime(t *testing.T) {
	t.Parallel()

	first := formatUIBuildVersion(time.Date(2026, 5, 31, 10, 0, 1, 0, time.UTC))
	second := formatUIBuildVersion(time.Date(2026, 5, 31, 10, 0, 2, 0, time.UTC))

	if first == "20260531a" {
		t.Fatalf("BuildVersion must include startup time, got legacy date-only value %q", first)
	}
	if first == second {
		t.Fatalf("same-day restarts must produce different cache-busting versions, got %q", first)
	}
	if !strings.HasPrefix(first, "20260531T100001") || !strings.HasSuffix(first, "a") {
		t.Fatalf("BuildVersion = %q, want compact date/time with suffix", first)
	}
}
