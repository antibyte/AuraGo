package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ── Daemon Skills API Handlers ──────────────────────────────────────────────
// Provides REST endpoints for the Web UI to manage long-running daemon skills.

// handleDaemonList returns all daemon states (GET /api/daemons).
func handleDaemonList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.DaemonSupervisor == nil {
			daemonJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Daemon supervisor not initialized"})
			return
		}
		states := s.DaemonSupervisor.ListDaemons()
		daemonJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"count":   len(states),
			"daemons": states,
		})
	}
}

// handleDaemonRefresh triggers a skill rescan from disk (POST /api/daemons/refresh).
func handleDaemonRefresh(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.DaemonSupervisor == nil {
			daemonJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Daemon supervisor not initialized"})
			return
		}
		if err := s.DaemonSupervisor.RefreshSkills(); err != nil {
			daemonJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
			return
		}
		daemonJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Daemon skill list refreshed"})
	}
}

// handleDaemonAction routes /api/daemons/{id}/{action} requests.
func handleDaemonAction(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.DaemonSupervisor == nil {
			daemonJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Daemon supervisor not initialized"})
			return
		}

		// Parse path: /api/daemons/{id} or /api/daemons/{id}/{action}
		path := strings.TrimPrefix(r.URL.Path, "/api/daemons/")
		if path == "" || path == "refresh" {
			// Handled by dedicated handlers
			http.NotFound(w, r)
			return
		}

		parts := strings.SplitN(path, "/", 2)
		skillID := parts[0]
		action := ""
		if len(parts) == 2 {
			action = parts[1]
		}

		switch action {
		case "": // GET /api/daemons/{id} — get status
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			state, ok := s.DaemonSupervisor.GetDaemonState(skillID)
			if !ok {
				daemonJSON(w, http.StatusNotFound, map[string]string{"status": "error", "message": "Daemon not found"})
				return
			}
			daemonJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "daemon": state})

		case "start": // POST /api/daemons/{id}/start
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := s.DaemonSupervisor.StartDaemon(skillID); err != nil {
				daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": err.Error()})
				return
			}
			daemonJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Daemon started"})

		case "stop": // POST /api/daemons/{id}/stop
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := s.DaemonSupervisor.StopDaemon(skillID); err != nil {
				daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": err.Error()})
				return
			}
			daemonJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Daemon stopped"})

		case "reenable": // POST /api/daemons/{id}/reenable
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := s.DaemonSupervisor.ReenableDaemon(skillID); err != nil {
				daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": err.Error()})
				return
			}
			daemonJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Daemon re-enabled"})

		default:
			daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Unknown action: " + action})
		}
	}
}

// daemonJSON writes a JSON response with the given status code.
func daemonJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
