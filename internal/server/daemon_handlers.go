package server

import (
	"aurago/internal/tools"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// resolveToManifestName resolves a raw skill ID (may be a DB ID like
// "agent_my_daemon_1712448000000") to the manifest name used as key in
// DaemonSupervisor.runners (e.g. "my_daemon").  If the ID cannot be resolved
// (SkillManager absent, skill not found, or rawID is already a name) the
// original value is returned unchanged so that callers always get a usable key.
func resolveToManifestName(s *Server, rawID string) string {
	if s.SkillManager == nil {
		return rawID
	}
	skill, err := s.SkillManager.GetSkill(rawID)
	if err != nil || skill == nil {
		return rawID
	}
	return skill.Name
}

// ── Daemon Skills API Handlers ──────────────────────────────────────────────
// Provides REST endpoints for the Web UI to manage long-running daemon skills.

// isDaemonAuthOK returns true if auth is disabled or the request is authenticated.
func isDaemonAuthOK(s *Server, r *http.Request) bool {
	s.CfgMu.RLock()
	enabled := s.Cfg.Auth.Enabled
	secret := s.Cfg.Auth.SessionSecret
	s.CfgMu.RUnlock()
	return !enabled || IsAuthenticated(r, secret)
}

// handleDaemonList returns all daemon states (GET /api/daemons).
func handleDaemonList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isDaemonAuthOK(s, r) {
			daemonJSON(w, http.StatusUnauthorized, map[string]string{"status": "error", "message": "Unauthorized"})
			return
		}
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
		if !isDaemonAuthOK(s, r) {
			daemonJSON(w, http.StatusUnauthorized, map[string]string{"status": "error", "message": "Unauthorized"})
			return
		}
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
		if !isDaemonAuthOK(s, r) {
			daemonJSON(w, http.StatusUnauthorized, map[string]string{"status": "error", "message": "Unauthorized"})
			return
		}
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
		skillID := resolveToManifestName(s, parts[0])
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
				s.Logger.Warn("[Daemon] start failed", "skill_id", skillID, "error", err)
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
				s.Logger.Warn("[Daemon] stop failed", "skill_id", skillID, "error", err)
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
				s.Logger.Warn("[Daemon] reenable failed", "skill_id", skillID, "error", err)
				daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": err.Error()})
				return
			}
			daemonJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Daemon re-enabled"})

		default:
			daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Unknown action: " + action})
		}
	}
}

// handleDaemonSkillSettings handles GET and PUT /api/skills/{id}/daemon
// for reading and updating daemon-specific trigger_mission_id and cheatsheet_id.
func handleDaemonSkillSettings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isDaemonAuthOK(s, r) {
			daemonJSON(w, http.StatusUnauthorized, map[string]string{"status": "error", "message": "Unauthorized"})
			return
		}

		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "missing skill ID"})
			return
		}

		if s.SkillManager == nil {
			daemonJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Skill manager not initialized"})
			return
		}

		skill, err := s.SkillManager.GetSkill(id)
		if err != nil || skill == nil {
			daemonJSON(w, http.StatusNotFound, map[string]string{"status": "error", "message": "Skill not found"})
			return
		}

		manifestPath := skill.FilePath
		if manifestPath == "" {
			daemonJSON(w, http.StatusNotFound, map[string]string{"status": "error", "message": "Skill manifest path not found"})
			return
		}

		manifestData, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			daemonJSON(w, http.StatusNotFound, map[string]string{"status": "error", "message": "Skill manifest not readable"})
			return
		}

		var manifest tools.SkillManifest
		if jsonErr := json.Unmarshal(manifestData, &manifest); jsonErr != nil {
			daemonJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "Invalid manifest JSON"})
			return
		}

		if manifest.Daemon == nil {
			daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Skill is not a daemon skill"})
			return
		}

		switch r.Method {
		case http.MethodGet:
			daemonJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
				"daemon": manifest.Daemon,
			})

		case http.MethodPut:
			var req struct {
				WakeAgent          *bool  `json:"wake_agent,omitempty"`
				TriggerMissionID   string `json:"trigger_mission_id"`
				TriggerMissionName string `json:"trigger_mission_name"`
				CheatsheetID       string `json:"cheatsheet_id"`
				CheatsheetName     string `json:"cheatsheet_name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Invalid JSON"})
				return
			}

			s.Logger.Info("DAEMON SETTINGS PUT", "skill_id", id, "mission_id", req.TriggerMissionID, "cheatsheet_id", req.CheatsheetID, "cheatsheetDB_nil", s.CheatsheetDB == nil)

			if req.TriggerMissionID != "" {
				if s.MissionManagerV2 == nil {
					daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Missions not enabled"})
					return
				}
				if m, ok := s.MissionManagerV2.Get(req.TriggerMissionID); !ok {
					daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Referenced mission not found"})
					return
				} else {
					req.TriggerMissionName = m.Name
				}
			}

			if req.CheatsheetID != "" {
				if s.CheatsheetDB == nil {
					daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Cheatsheets not enabled"})
					return
				}
				cs, csErr := tools.CheatsheetGet(s.CheatsheetDB, req.CheatsheetID)
				if csErr != nil || !cs.Active {
					daemonJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Referenced cheatsheet not found or inactive"})
					return
				}
				req.CheatsheetName = cs.Name
			}

			if req.WakeAgent != nil {
				manifest.Daemon.WakeAgent = *req.WakeAgent
			}
			manifest.Daemon.TriggerMissionID = req.TriggerMissionID
			manifest.Daemon.TriggerMissionName = req.TriggerMissionName
			manifest.Daemon.CheatsheetID = req.CheatsheetID
			manifest.Daemon.CheatsheetName = req.CheatsheetName

			updated, marshalErr := json.MarshalIndent(manifest, "", "  ")
			if marshalErr != nil {
				daemonJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "Failed to serialize manifest"})
				return
			}
			if writeErr := os.WriteFile(manifestPath, updated, 0644); writeErr != nil {
				s.Logger.Error("Failed to save daemon skill settings", "skill_id", id, "error", writeErr)
				daemonJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "Failed to save settings"})
				return
			}

			tools.InvalidateSkillsCache(s.Cfg.Directories.SkillsDir)

			if s.DaemonSupervisor != nil {
				if refreshErr := s.DaemonSupervisor.RefreshSkills(); refreshErr != nil {
					s.Logger.Warn("Daemon refresh failed after settings update", "error", refreshErr)
				}
			}

			s.Logger.Info("Daemon skill settings updated", "skill_id", id, "mission_id", req.TriggerMissionID, "cheatsheet_id", req.CheatsheetID)
			daemonJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
				"daemon": manifest.Daemon,
			})

		default:
			daemonJSON(w, http.StatusMethodNotAllowed, map[string]string{"status": "error", "message": "Method not allowed"})
		}
	}
}

// daemonJSON serializes v as JSON and writes it with the given HTTP status code.
func daemonJSON(w http.ResponseWriter, code int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"status":"error","message":"internal serialization error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(data)
}
