package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

// handleSupertonicStatus returns the current health status of the Supertonic TTS sidecar.
func handleSupertonicStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		provider := s.Cfg.TTS.Provider
		autoStart := s.Cfg.TTS.Supertonic.AutoStart
		baseURL := s.Cfg.TTS.Supertonic.URL
		containerName := s.Cfg.TTS.Supertonic.ContainerName
		image := s.Cfg.TTS.Supertonic.Image
		s.CfgMu.RUnlock()

		if !strings.EqualFold(strings.TrimSpace(provider), "supertonic") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "Supertonic TTS is not the active TTS provider",
			})
			return
		}

		health := tools.SupertonicHealth(baseURL)
		health["auto_start"] = autoStart
		health["container_name"] = containerName
		health["image"] = image
		json.NewEncoder(w).Encode(health)
	}
}

// handleSupertonicStyles returns available built-in and imported Supertonic voice styles.
func handleSupertonicStyles(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		baseURL := s.Cfg.TTS.Supertonic.URL
		s.CfgMu.RUnlock()

		styles, err := tools.SupertonicListStyles(baseURL)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to list Supertonic styles", "error", err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"styles": styles,
		})
	}
}
