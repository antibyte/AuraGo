package meshcore

import (
	"testing"

	"go.bug.st/serial"
)

func TestUSBTransportKeepsConfiguredAliasAndCompanionSignals(t *testing.T) {
	const configuredAlias = "/dev/serial/by-id/meshcore-companion"
	cfg := Config{Transport: "usb", Port: configuredAlias}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if cfg.Port != configuredAlias {
		t.Fatalf("configured USB alias changed to %q", cfg.Port)
	}
	mode := meshCoreSerialMode()
	if mode.BaudRate != 115200 || mode.DataBits != 8 || mode.Parity != serial.NoParity || mode.StopBits != serial.OneStopBit || mode.InitialStatusBits == nil || !mode.InitialStatusBits.DTR || mode.InitialStatusBits.RTS {
		t.Fatalf("MeshCore Companion mode = %+v", mode)
	}
}
