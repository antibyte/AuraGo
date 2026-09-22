package server

import (
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/config"
	"aurago/internal/prompts"
)

// Configuration publication invalidates outstanding helper requests before
// exposing a new persona or disabling one of the analysis features.
func (s *Server) syncPersonalityConfig(next *config.Config) {
	if s.ShortTermMem == nil || next == nil {
		return
	}
	invalidate := false
	if old := s.Cfg; old != nil {
		invalidate = old.Personality.Engine != next.Personality.Engine || old.Personality.EngineV2 != next.Personality.EngineV2 || old.Personality.EmotionSynthesizer.Enabled != next.Personality.EmotionSynthesizer.Enabled || old.Personality.InnerVoice.Enabled != next.Personality.InnerVoice.Enabled
	}
	id, _ := prompts.ResolvePersonalityID(next.Personality.CorePersonality)
	meta := prompts.GetCorePersonalityMeta(next.Directories.PromptsDir, id)
	if err := s.ShortTermMem.SetPersonalityContext(id, meta, invalidate); err != nil && s.Logger != nil {
		s.Logger.Warn("Personality context could not be refreshed", "error", err)
	}
}

func handlePersonalityDynamicsReset(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.Personality.Engine {
			jsonError(w, "Personality engine is disabled", http.StatusBadRequest)
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Personality state unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, err := s.ShortTermMem.ResetPersonalityDynamics(time.Now()); err != nil {
			jsonError(w, "Failed to reset personality dynamics", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(s.buildPersonalityStatePayload())
	}
}
