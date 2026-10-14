package audit

import (
	"strings"
	"testing"
)

func TestMeshCoreUSBSignalsHostReady(t *testing.T) {
	// The typed transport test verifies 115200/8N1, DTR and RTS. This audit
	// keeps the production open path connected to that mode and the shared owner.
	source := readRepoFile(t, "internal/meshcore/transport.go")
	if !strings.Contains(source, "serialutil.OpenConfiguredContext(ctx, cfg.Port, serialutil.OwnerMeshCore, meshCoreSerialMode())") {
		t.Fatal("USB transport bypasses the shared port owner or configured companion mode")
	}
	tests := readRepoFile(t, "internal/meshcore/transport_test.go")
	if !strings.Contains(tests, "func TestUSBTransportKeepsConfiguredAliasAndCompanionSignals(") {
		t.Fatal("missing typed companion serial settings regression")
	}
}
