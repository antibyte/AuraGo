package audit

import (
	"slices"
	"strings"
	"testing"
)

// The ecalc tool computes on an embedded catalog (read: search, describe, calculate, table, pinout, decode, encode) and, with
// its open operation, puts a window on the desktop (change). It has no config switch of its own, so the matrix entry
// names the desktop gates of open and the operations an A2A peer may use.
func TestToolPermissionMatrixDescribesECalc(t *testing.T) {
	t.Parallel()

	found := false
	for _, entry := range ToolPermissionMatrix() {
		if entry.Name != "ecalc" {
			continue
		}
		found = true
		if !slices.Equal(entry.Capabilities, []Capability{CapabilityRead, CapabilityChange}) {
			t.Fatalf("ecalc capabilities = %v, want read and change only", entry.Capabilities)
		}
		for _, want := range []string{"always on", "virtual_desktop.allow_agent_control", "tools.virtual_desktop.enabled", "A2A", "calculate, table, pinout, decode and encode only"} {
			if !strings.Contains(entry.ConfigGate, want) {
				t.Fatalf("ecalc gate %q lacks %q", entry.ConfigGate, want)
			}
		}
	}
	if !found {
		t.Fatal("ecalc is missing from ToolPermissionMatrix")
	}
}
