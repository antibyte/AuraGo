package server

import (
	"encoding/json"
	"net/http"
)

// handleWarnings returns all warnings from the registry.
// GET /api/warnings
func handleWarnings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.WarningsRegistry == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"warnings":       []interface{}{},
				"total":          0,
				"unacknowledged": 0,
			})
			return
		}

		all := s.WarningsRegistry.Warnings()
		total, unack := s.WarningsRegistry.Count()

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"warnings":       all,
			"total":          total,
			"unacknowledged": unack,
		})
	}
}

// handleWarningsAcknowledge marks one or all warnings as acknowledged.
// POST /api/warnings/acknowledge  {"id": "some_id"} or {"all": true}
func handleWarningsAcknowledge(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.WarningsRegistry == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
			return
		}

		var req struct {
			ID  string `json:"id"`
			All bool   `json:"all"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.All {
			s.WarningsRegistry.AcknowledgeAll()
		} else if req.ID != "" {
			if !s.WarningsRegistry.Acknowledge(req.ID) {
				writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "warning not found"})
				return
			}
		} else {
			http.Error(w, "provide id or all:true", http.StatusBadRequest)
			return
		}

		// Broadcast updated count so all connected UIs refresh their badge.
		total, unack := s.WarningsRegistry.Count()
		if s.SSE != nil {
			s.SSE.BroadcastType(EventSystemWarning, map[string]interface{}{
				"total":          total,
				"unacknowledged": unack,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}
