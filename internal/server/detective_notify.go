package server

import (
	"sync"
	"time"

	"aurago/internal/desktop"
)

// detectiveNotifyInterval bounds how often one case reaches the notification center: a
// scheduled case can publish, report no changes and ask a question within minutes.
const detectiveNotifyInterval = time.Minute

type detectiveNotifyLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

// allow reports whether the case may notify now and records the attempt.
func (l *detectiveNotifyLimiter) allow(caseID string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last == nil {
		l.last = map[string]time.Time{}
	}
	if at, ok := l.last[caseID]; ok && now.Sub(at) < detectiveNotifyInterval {
		return false
	}
	l.last[caseID] = now
	// Deleted cases would otherwise keep their slot forever; prune once the map is large.
	if len(l.last) > 256 {
		for id, at := range l.last {
			if now.Sub(at) >= detectiveNotifyInterval {
				delete(l.last, id)
			}
		}
	}
	return true
}

// notifyDetective is the Desktop sink for a research case event, shaped like the person
// alert and flow failure notifications: a notification for the notification center whose
// context lets a click open the case. It stays silent without Virtual Desktop and never
// fails the research path.
func (s *Server) notifyDetective(kind, caseID, title, message string) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled || caseID == "" {
		return
	}
	now := time.Now().UTC()
	if !s.detectiveNotify.allow(caseID, now) {
		return
	}
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "notification", Payload: map[string]any{
		"title":   title,
		"message": message,
		"type":    kind,
		"appId":   "detective",
		"context": map[string]any{"case_id": caseID},
	}, CreatedAt: now})
}
