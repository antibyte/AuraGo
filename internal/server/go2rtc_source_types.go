package server

import (
	"net/http"

	"aurago/internal/config"
)

// handleGo2RTCSourceTypes serves the camera source catalog for the guided
// forms. Static, non-secret data; validation stays server-side.
func handleGo2RTCSourceTypes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=300")
		writeGo2RTCJSON(w, map[string]interface{}{"types": config.Go2RTCSourceTypes()})
	}
}
