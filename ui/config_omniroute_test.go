package ui

import (
	"strings"
	"testing"
)

func TestConfigOmniRouteEnableToggleMarksUserEditAfterRerender(t *testing.T) {
	t.Parallel()

	omniRouteJS := normalizeAssetText(mustReadUIFile(t, "cfg/omniroute.js"))
	start := strings.Index(omniRouteJS, "function omniRouteToggleEnabled(")
	if start < 0 {
		t.Fatal("omniroute.js missing omniRouteToggleEnabled")
	}
	end := strings.Index(omniRouteJS[start:], "function omniRoutePayload(")
	if end < 0 {
		t.Fatal("omniroute.js missing omniRoutePayload after omniRouteToggleEnabled")
	}
	fn := omniRouteJS[start : start+end]

	if strings.Contains(fn, "setDirty(true)") {
		t.Fatal("omniRouteToggleEnabled must use markDirty so pending baseline refresh timers cannot clear the save state")
	}
	for _, marker := range []string{
		"renderOmniRouteSection(null);",
		"markDirty();",
	} {
		if !strings.Contains(fn, marker) {
			t.Fatalf("omniRouteToggleEnabled missing marker %q", marker)
		}
	}
	if strings.Index(fn, "renderOmniRouteSection(null);") > strings.Index(fn, "markDirty();") {
		t.Fatal("omniRouteToggleEnabled must call markDirty after rerender so the new enabled form state is snapshotted")
	}
}
