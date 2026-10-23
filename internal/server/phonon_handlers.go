package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/phonon"
)

func handlePhononVoices(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		root := phonon.Root()
		language := cfg.SanoTTSLanguage("")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"voices":           phonon.Voices(root, cfg.Directories.DataDir),
			"ready":            phonon.Language(language) != "" && phonon.Ready(root) == nil,
			"language":         language,
			"default_voice_id": phonon.DefaultVoice(language, cfg.Personality.CorePersonality),
		})
	}
}

func handlePhononVoiceDownload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !checkCSRFOrigin(r) {
			jsonError(w, "Invalid request origin", http.StatusForbidden)
			return
		}
		suffix := strings.TrimPrefix(r.URL.Path, "/api/tts/phonon/voices/")
		id, found := strings.CutSuffix(suffix, "/download")
		if _, ok := phonon.Lookup(id); !found || !ok {
			jsonError(w, "Unknown Phonon voice", http.StatusBadRequest)
			return
		}
		cfg := s.ConfigSnapshot()
		if err := phonon.DownloadVoice(r.Context(), phonon.Root(), cfg.Directories.DataDir, id); err != nil {
			jsonError(w, "Phonon voice download failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "installed": true})
	}
}
