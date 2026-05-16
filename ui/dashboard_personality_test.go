package ui

import (
	"strings"
	"testing"
)

func TestDashboardPersonalityDisabledStateIsNotVisibleByDefault(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "dashboard.html")
	marker := `id="personality-disabled" class="empty-state is-hidden"`
	if !strings.Contains(html, marker) {
		t.Fatalf("personality disabled placeholder should be hidden until API confirms disabled state; missing %q", marker)
	}
}
