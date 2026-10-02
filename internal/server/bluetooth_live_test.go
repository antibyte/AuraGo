package server

import (
	"log/slog"
	"testing"
	"time"

	"aurago/internal/bluetooth"
	"aurago/internal/desktop"
)

func TestDesktopCapabilitiesFollowBluetoothPresence(t *testing.T) {
	server := testBluetoothServer()
	capabilities := desktopCapabilities{s: server}
	if !capabilities.HasCapability("bluetooth") {
		t.Fatal("present adapter must grant the bluetooth capability")
	}
	if capabilities.HasCapability("wifi") {
		t.Fatal("unknown capabilities must be false")
	}
	server.Bluetooth.SeedStatus(bluetooth.Status{Supported: true})
	if capabilities.HasCapability("bluetooth") {
		t.Fatal("absent adapter must not grant the capability")
	}
}

func TestPublishBluetoothChangeBroadcastsDesktopEvents(t *testing.T) {
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	server := &Server{DesktopHub: hub, Logger: slog.Default()}
	server.publishBluetoothChange(bluetooth.Change{Revision: 7, CapabilityChanged: true, Present: true, InteractionID: "abc"})
	for _, want := range []string{"bluetooth_changed", "bluetooth_interaction", "desktop_changed"} {
		select {
		case event := <-events:
			if event.Type != want {
				t.Fatalf("event = %q, want %q", event.Type, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing %s", want)
		}
	}
}
