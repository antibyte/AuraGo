package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) registerToolAPIRoutes(mux *http.ServeMux) {
	if s.Cfg.WebConfig.Enabled {
		// ── Skills Manager API ──
		mux.HandleFunc("/api/skills/import", handleImportSkill(s))
		mux.HandleFunc("/api/skills/generate", handleGenerateSkillDraft(s))
		mux.HandleFunc("/api/skills/upload", handleUploadSkill(s))
		mux.HandleFunc("/api/skills/templates", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleSkillTemplates(s)(w, r)
			case http.MethodPost:
				handleCreateSkillFromTemplate(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/skills/stats", handleSkillStats(s))
		mux.HandleFunc("/api/skills/available-tools", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			tools := []string{}
			if s.Cfg != nil && s.Cfg.Tools.PythonToolBridge.Enabled {
				tools = s.Cfg.Tools.PythonToolBridge.AllowedTools
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"tools":  tools,
			})
		})
		mux.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleListSkills(s)(w, r)
			case http.MethodPost:
				handleCreateSkill(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/skills/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/verify") {
				handleVerifySkill(s)(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/versions") {
				handleGetSkillVersions(s)(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/audit") {
				handleGetSkillAudit(s)(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/export") {
				handleExportSkill(s)(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/test") {
				handleTestSkill(s)(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/daemon") {
				handleDaemonSkillSettings(s)(w, r)
				return
			}
			switch r.Method {
			case http.MethodGet:
				handleGetSkill(s)(w, r)
			case http.MethodPut:
				handleUpdateSkill(s)(w, r)
			case http.MethodDelete:
				handleDeleteSkill(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

		// ── Containers API ──
		mux.HandleFunc("/api/containers", handleContainersList(s))
		mux.HandleFunc("/api/containers/", handleContainerAction(s))

		// ── Daemon Skills API ──
		// Always register daemon routes so the frontend gets proper JSON errors
		// instead of a 404 when daemon_skills.enabled = false (the default).
		mux.HandleFunc("/api/daemons", handleDaemonList(s))
		mux.HandleFunc("/api/daemons/refresh", handleDaemonRefresh(s))
		mux.HandleFunc("/api/daemons/", handleDaemonAction(s))

		// ── Cheat Sheets API ──
		mux.HandleFunc("/api/cheatsheets", handleCheatSheets(s))
		mux.HandleFunc("/api/cheatsheets/", handleCheatSheetRouter(s))

		// ── Contacts (Address Book) API ──
		mux.HandleFunc("/api/contacts", handleContacts(s))
		mux.HandleFunc("/api/contacts/", handleContactByID(s))

		// ── Planner (Appointments & Todos) API ──
		mux.HandleFunc("/api/appointments", handleAppointments(s))
		mux.HandleFunc("/api/appointments/", handleAppointmentByID(s))
		mux.HandleFunc("/api/todos", handleTodos(s))
		mux.HandleFunc("/api/todos/", handleTodoByID(s))

		// ── SQL Connections API ──
		mux.HandleFunc("/api/sql-connections", handleSQLConnections(s))
		mux.HandleFunc("/api/sql-connections/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
			if strings.HasSuffix(path, "/test") {
				handleSQLConnectionTest(s)(w, r)
			} else {
				handleSQLConnectionByID(s)(w, r)
			}
		})

		// ── Knowledge Files API ──
		mux.HandleFunc("/api/knowledge", handleKnowledgeFiles(s))
		mux.HandleFunc("/api/knowledge/upload", handleKnowledgeUpload(s))
		mux.HandleFunc("/api/knowledge/", handleKnowledgeFile(s))
		// Inline preview endpoint (allows iframe embedding for PDFs/images)
		mux.HandleFunc("/api/knowledge-inline/", handleKnowledgeFileInline(s))
	}

	// ── Remote Control API (handlers guard themselves with s.RemoteHub == nil check) ──
	if s.RemoteHub != nil {
		mux.HandleFunc("/api/remote/devices", handleRemoteDevices(s))
		mux.HandleFunc("/api/remote/devices/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/remote/devices/")
			if strings.HasSuffix(path, "/approve") {
				handleRemoteDeviceApprove(s)(w, r)
			} else if strings.HasSuffix(path, "/reject") {
				handleRemoteDeviceReject(s)(w, r)
			} else if strings.HasSuffix(path, "/revoke") {
				handleRemoteDeviceRevoke(s)(w, r)
			} else {
				handleRemoteDevice(s)(w, r)
			}
		})
		mux.HandleFunc("/api/remote/enroll", handleRemoteEnrollmentCreate(s))
		mux.HandleFunc("/api/remote/audit", handleRemoteAuditLog(s))
		mux.HandleFunc("/api/remote/platforms", handleRemotePlatforms(s))
		mux.HandleFunc("/api/remote/download/", handleRemoteDownload(s))
		mux.HandleFunc("/api/remote/ws", handleRemoteWebSocket(s))
		s.Logger.Info("Remote Control API registered at /api/remote/...")
	}
}
