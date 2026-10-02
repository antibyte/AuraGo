package bluetooth

import (
	"context"
	"time"
)

type discoverableWindow struct {
	release      func()
	prevPairable bool
	timer        *time.Timer
}

func (m *Manager) endDiscoverable(ctx context.Context) {
	m.mu.Lock()
	window := m.window
	m.window = nil
	m.mu.Unlock()
	if window == nil {
		return
	}
	if window.timer != nil {
		window.timer.Stop()
	}
	if m.live != nil {
		m.live.setDiscoverable(time.Time{})
		if bus, path, err := m.liveAdapter(); err == nil {
			_ = bus.SetProperty(ctx, path, bluezAdapterInterface, "Discoverable", false)
			_ = bus.SetProperty(ctx, path, bluezAdapterInterface, "Pairable", window.prevPairable)
		}
	}
	window.release()
}

func (m *Manager) stopDiscoveryQuietly(ctx context.Context) {
	m.mu.Lock()
	if m.discoveryTimer != nil {
		m.discoveryTimer.Stop()
		m.discoveryTimer = nil
	}
	m.mu.Unlock()
	if m.live == nil || m.live.discoveryEnd().IsZero() {
		return
	}
	m.live.setDiscovery(time.Time{})
	if bus, path, err := m.liveAdapter(); err == nil {
		if callErr := bus.Call(ctx, path, bluezAdapterInterface+".StopDiscovery"); callErr != nil {
			m.logger.Debug("[Bluetooth] StopDiscovery failed", "error", callErr)
		}
	}
}

func (m *Manager) liveAdapter() (bluezBus, string, error) {
	if !m.liveActive() {
		return nil, "", codedError(ErrorUnavailable, "The live Bluetooth session is not running.", nil)
	}
	bus, err := m.live.currentBus()
	if err != nil {
		return nil, "", err
	}
	path := m.live.adapterPath()
	if path == "" {
		return nil, "", codedError(ErrorUnavailable, "No Bluetooth adapter was found.", nil)
	}
	return bus, path, nil
}
