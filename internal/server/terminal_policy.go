package server

import (
	"context"
	"net/http"
)

// Terminal admission shares the config publication lock, so a concurrent
// revocation either cancels this connection or rejects it before shell creation.
func (s *Server) admitDockerTerminal(w http.ResponseWriter, r *http.Request, expectedHost ...string) (context.CancelFunc, bool) {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Docker.Enabled {
		jsonError(w, "Docker is not enabled", http.StatusServiceUnavailable)
		return nil, false
	}
	if len(expectedHost) > 0 && expectedHost[0] != cfg.Docker.Host {
		jsonError(w, "Docker host changed; retry the request", http.StatusConflict)
		return nil, false
	}
	if cfg.Docker.ReadOnly {
		jsonError(w, "Docker is in read-only mode", http.StatusForbidden)
		return nil, false
	}
	if r.Context().Err() != nil {
		jsonError(w, "terminal request was canceled", http.StatusRequestTimeout)
		return nil, false
	}
	registry := &s.dockerTerminalRuns
	registry.mu.Lock()
	if registry.runs == nil {
		registry.runs = make(map[uint64]context.CancelFunc)
	}
	registry.next++
	id := registry.next
	ctx, cancel := context.WithCancel(r.Context())
	registry.runs[id] = cancel
	registry.mu.Unlock()
	context.AfterFunc(ctx, func() { registry.mu.Lock(); delete(registry.runs, id); registry.mu.Unlock() })
	*r = *r.WithContext(ctx)
	return cancel, true
}

func (s *Server) revokeDockerTerminals() {
	registry := &s.dockerTerminalRuns
	registry.mu.Lock()
	runs := registry.runs
	registry.runs = nil
	registry.mu.Unlock()
	for _, cancel := range runs {
		cancel()
	}
}
