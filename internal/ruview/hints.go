package ruview

import "aurago/internal/config"

const (
	hintSensingStale    = "sensing_stale"
	hintStorageLimit    = "storage_limit"
	hintStopUnconfirmed = "stop_unconfirmed"
)

// SetOperationalHint registers an edge-triggered callback. The same state does
// not call it again. The callback runs outside the manager lock and must not
// call the manager. A nil callback clears it. Disabled configuration does not
// resolve an open hint; only the matching success transition does.
func (m *Manager) SetOperationalHint(fn func(event string, open bool)) {
	m.mu.Lock()
	m.hintFn = fn
	m.mu.Unlock()
}

func (m *Manager) emitHint(event string, open bool) {
	m.mu.Lock()
	if m.hintOpen[event] == open {
		m.mu.Unlock()
		return
	}
	if m.hintOpen == nil {
		m.hintOpen = map[string]bool{}
	}
	m.hintOpen[event] = open
	fn := m.hintFn
	m.mu.Unlock()
	if fn != nil {
		fn(event, open)
	}
}

func (m *Manager) noteSensing(previous, next Status, cfg config.RuViewConfig) {
	if !cfg.Enabled {
		return
	}
	source := next.Source
	if source == "" {
		source = cfg.Source
	}
	if next.Fresh && !next.Simulated && source == "esp32" {
		m.emitHint(hintSensingStale, false)
		return
	}
	if next.Simulated || source != "esp32" {
		return
	}
	if !probeFreshOrLive(previous) || !probeStaleOrDisconnected(next) {
		return
	}
	m.emitHint(hintSensingStale, true)
}

func probeFreshOrLive(s Status) bool {
	if s.Simulated || s.Source != "esp32" {
		return false
	}
	if s.Fresh {
		return true
	}
	return s.Ready && s.Error == "" && (s.DataState == "live_unverified" || s.DataState == "live")
}

func probeStaleOrDisconnected(s Status) bool {
	switch s.DataState {
	case "stale", "disconnected":
		return true
	case "live_unverified":
		return !s.Fresh && s.FrameAgeMS != nil && *s.FrameAgeMS > 10000
	default:
		return false
	}
}

func (m *Manager) noteStorage(mode string, limited bool) {
	if mode == "external" {
		return
	}
	m.mu.Lock()
	was := m.budgetLimited
	m.budgetLimited = limited
	m.mu.Unlock()
	if was == limited {
		return
	}
	m.emitHint(hintStorageLimit, limited)
}

func (m *Manager) noteStop(confirmed bool, mode string) {
	if !confirmed {
		if mode == "external" {
			return
		}
		m.emitHint(hintStopUnconfirmed, true)
		return
	}
	m.emitHint(hintStopUnconfirmed, false)
}
