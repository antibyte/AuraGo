package server

import (
	"context"
	"time"

	"aurago/internal/bluetooth"
	"aurago/internal/desktop"
)

// desktopCapabilities answers hardware questions for virtual-desktop app gating.
type desktopCapabilities struct{ s *Server }

func (c desktopCapabilities) HasCapability(name string) bool {
	if c.s == nil {
		return false
	}
	switch name {
	case "bluetooth":
		return c.s.Bluetooth != nil && c.s.Bluetooth.Present()
	case "flows":
		return c.s.flowsAvailable()
	default:
		return false
	}
}

// initBluetoothLive starts the live BlueZ session and mirrors its changes into
// payload-free desktop events.
func (s *Server) initBluetoothLive(ctx context.Context) {
	if s == nil || s.Bluetooth == nil {
		return
	}
	s.Bluetooth.StartLive(ctx)
	changes, cancel := s.Bluetooth.Subscribe()
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case change := <-changes:
				s.publishBluetoothChange(change)
			}
		}
	}()
}

func (s *Server) publishBluetoothChange(change bluetooth.Change) {
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	now := time.Now().UTC()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "bluetooth_changed", Payload: map[string]interface{}{"revision": change.Revision}, CreatedAt: now})
	if change.InteractionID != "" {
		broadcastDesktopEvent(s, hub, desktop.Event{Type: "bluetooth_interaction", Payload: map[string]interface{}{"id": change.InteractionID}, CreatedAt: now})
	}
	if change.CapabilityChanged {
		broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{
			"operation": "app_availability", "app_id": "bluetooth", "available": change.Present,
		}, CreatedAt: now})
	}
}
