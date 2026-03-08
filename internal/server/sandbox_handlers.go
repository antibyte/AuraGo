package server

import (
	"aurago/internal/tools"
	"encoding/json"
	"net/http"
)

// handleSandboxStatus returns the sandbox readiness status as JSON.
func handleSandboxStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := tools.SandboxGetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}
