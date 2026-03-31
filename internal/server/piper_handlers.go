package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handlePiperStatus returns the current health status of the Piper TTS container.
func handlePiperStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		enabled := s.Cfg.TTS.Piper.Enabled
		port := s.Cfg.TTS.Piper.ContainerPort
		s.CfgMu.RUnlock()

		if !enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "Piper TTS is not enabled",
			})
			return
		}

		health := tools.PiperHealth(port)
		json.NewEncoder(w).Encode(health)
	}
}

// handlePiperVoices returns the list of voices available on the Piper instance.
func handlePiperVoices(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		enabled := s.Cfg.TTS.Piper.Enabled
		port := s.Cfg.TTS.Piper.ContainerPort
		s.CfgMu.RUnlock()

		if !enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Piper TTS is not enabled",
			})
			return
		}

		voices, err := tools.PiperListVoices(port)
		if err != nil {
			s.Logger.Error("Failed to list Piper voices", "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Failed to list voices",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"voices": voices,
		})
	}
}
