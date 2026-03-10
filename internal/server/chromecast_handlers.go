package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleChromecastDiscover triggers an mDNS scan for Chromecast devices and
// returns the discovered devices as JSON.
// GET /api/chromecast/discover
func handleChromecastDiscover(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		raw := tools.ChromecastDiscover(s.Logger)

		// Forward the JSON string from ChromecastDiscover as-is
		w.Header().Set("Content-Type", "application/json")

		// Validate that it's proper JSON before sending
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			http.Error(w, `{"error":"discovery returned invalid data"}`, http.StatusInternalServerError)
			return
		}

		w.Write([]byte(raw))
	}
}
