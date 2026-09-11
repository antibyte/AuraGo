package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	systemWorldArtifactLimit    = 8
	systemWorldArtifactRunes    = 96
	systemWorldArtifactCooldown = 4 * time.Second
)

// handleSystemWorldMemoryArtifacts serves a small random batch of short memory excerpts for
// the hologram above the memory archive. It reuses the tower-voice sampler: the same
// read-only sources, the same secret/thinking/code scrubbing, no LLM, no SSE, no logging.
func handleSystemWorldMemoryArtifacts(s *Server) http.HandlerFunc {
	var mu sync.Mutex
	var next time.Time
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Scoped desktop readers must not gain access to the owner's global memory/chat.
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if !s.ConfigSnapshot().VirtualDesktop.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if !mu.TryLock() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		defer mu.Unlock()
		if time.Now().Before(next) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next = time.Now().Add(systemWorldArtifactCooldown)
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		artifacts := systemWorldSampleExcerpts(ctx, s, systemWorldArtifactLimit, systemWorldArtifactRunes)
		if ctx.Err() != nil {
			return
		}
		if artifacts == nil {
			artifacts = []string{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": artifacts})
	}
}
