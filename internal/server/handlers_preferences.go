package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/agent"
)

// SessionPreferences stores per-session UI preferences that can influence
// agent behavior (e.g. voice output mode).
type SessionPreferences struct {
	SpeakerMode bool `json:"speaker_mode"`
}

// GetSpeakerMode returns the current speaker mode preference.
func GetSpeakerMode() bool {
	return agent.GetVoiceMode()
}

func registerPreferencesHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/preferences", handlePreferences(s))
}

func handlePreferences(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			prefs := SessionPreferences{SpeakerMode: agent.GetVoiceMode()}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(prefs)

		case http.MethodPost:
			var incoming SessionPreferences
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&incoming); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			agent.SetVoiceMode(incoming.SpeakerMode)
			s.Logger.Info("Session preferences updated", "speaker_mode", incoming.SpeakerMode)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
