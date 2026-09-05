package audit

import (
	"strings"
	"testing"
)

func TestMeshCoreUSBSignalsHostReady(t *testing.T) {
	// Hardware contract: TinyUSB CDC drops writes while DTR is deasserted.
	// This guards the actual serial.Open settings without adding a production
	// injection seam solely to test serial-library arguments.
	source := readRepoFile(t, "internal/meshcore/transport.go")
	start := strings.Index(source, "serial.Open(")
	if start < 0 {
		t.Fatal("missing USB serial open")
	}
	end := strings.Index(source[start:], "\n")
	if end < 0 {
		t.Fatal("missing USB serial mode")
	}
	mode := source[start : start+end]
	for _, required := range []string{"BaudRate: 115200", "DataBits: 8", "Parity: serial.NoParity", "StopBits: serial.OneStopBit", "DTR: true", "RTS: false"} {
		if !strings.Contains(mode, required) {
			t.Fatalf("Companion USB host-ready contract missing %q", required)
		}
	}
}
