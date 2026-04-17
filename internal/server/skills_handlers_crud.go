package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"aurago/internal/tools"
)

// handleListSkills returns all skills with optional filters.
// GET /api/skills?type=agent&status=clean&enabled=true&search=foo
func handleListSkills(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}

		q := r.URL.Query()
		skillType := q.Get("type")
		status := q.Get("status")
		search := q.Get("search")
		var enabledFilter *bool
		if e := q.Get("enabled"); e != "" {
			val := e == "true" || e == "1"
			enabledFilter = &val
		}

		skills, err := s.SkillManager.ListSkillsFiltered(skillType, status, search, enabledFilter)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list skills", "Failed to list skills", err)
			return
		}
		if skills == nil {
			skills = []tools.SkillRegistryEntry{}
		}

		// Get stats
		total, agentCount, userCount, pending, _ := s.SkillManager.GetStats()

		daemonStateMap := map[string]interface{}{}
		if s.DaemonSupervisor != nil {
			for _, ds := range s.DaemonSupervisor.ListDaemons() {
				daemonStateMap[ds.SkillID] = ds
			}
		}

		daemonSystemEnabled := s.DaemonSupervisor != nil

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":                "ok",
			"count":                 len(skills),
			"skills":                skills,
			"daemon_states":         daemonStateMap,
			"daemon_system_enabled": daemonSystemEnabled,
			"stats": map[string]int{
				"total":   total,
				"agent":   agentCount,
				"user":    userCount,
				"pending": pending,
			},
		})
	}
}

// handleGetSkill returns a single skill with its code.
// GET /api/skills/{id}
func handleGetSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}

		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		// Reject special sub-paths handled by other handlers
		if id == "upload" || id == "templates" || id == "stats" {
			jsonError(w, "Invalid skill ID", http.StatusBadRequest)
			return
		}

		skill, err := s.SkillManager.GetSkill(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Skill lookup failed", err, "skill_id", id)
			return
		}

		// Include code if requested
		var code string
		if r.URL.Query().Get("code") == "true" {
			code, _ = s.SkillManager.GetSkillCode(id)
		}

		// Include daemon manifest config if this is a daemon skill
		// Check both skill.IsDaemon (DB) and manifest.Daemon (disk) since DB may be stale
		var daemonManifest interface{}
		if skill.FilePath != "" {
			if manifestData, err := os.ReadFile(skill.FilePath); err == nil {
				var manifest tools.SkillManifest
				if json.Unmarshal(manifestData, &manifest) == nil && manifest.Daemon != nil {
					daemonManifest = manifest.Daemon
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"skill":  skill,
			"code":   code,
			"daemon": daemonManifest,
		})
	}
}

// handleCreateSkill creates a new skill from JSON body with inline code.
// POST /api/skills
func handleCreateSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		readOnly := s.Cfg.Tools.SkillManager.ReadOnly
		allowUploads := s.Cfg.Tools.SkillManager.AllowUploads
		s.CfgMu.RUnlock()

		if readOnly || !allowUploads {
			jsonError(w, "Skill creation is disabled (read-only mode or uploads not allowed)", http.StatusForbidden)
			return
		}

		var req struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Category    string   `json:"category"`
			Tags        []string `json:"tags"`
			Code        string   `json:"code"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.Code == "" {
			jsonError(w, "Name and code are required", http.StatusBadRequest)
			return
		}

		// Validate the code
		s.CfgMu.RLock()
		maxSize := s.Cfg.Tools.SkillManager.MaxUploadSizeMB
		s.CfgMu.RUnlock()
		validation := tools.ValidateSkillUpload([]byte(req.Code), req.Name+".py", maxSize)

		skill, err := s.SkillManager.CreateSkillEntry(req.Name, req.Description, req.Code, tools.SkillTypeUser, "user", req.Category, req.Tags)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create skill", "Failed to create skill", err, "skill_name", req.Name)
			return
		}

		// Run security scan
		s.CfgMu.RLock()
		requireScan := s.Cfg.Tools.SkillManager.RequireScan
		autoEnable := s.Cfg.Tools.SkillManager.AutoEnableClean
		useGuardian := s.Cfg.Tools.SkillManager.ScanWithGuardian
		vtEnabled := s.Cfg.VirusTotal.Enabled
		vtKey := s.Cfg.VirusTotal.APIKey
		s.CfgMu.RUnlock()

		var scanReport *tools.SecurityReport
		var secStatus tools.SecurityStatus

		if requireScan {
			scanReport, secStatus, err = s.SkillManager.ScanSkill(r.Context(), skill.ID, vtKey, s.LLMGuardian, vtEnabled, useGuardian)
			if err != nil {
				s.Logger.Warn("Skill security scan failed", "id", skill.ID, "error", err)
			}
			skill.SecurityStatus = secStatus
			skill.SecurityReport = scanReport

			if autoEnable && secStatus == tools.SecurityClean {
				s.SkillManager.EnableSkill(skill.ID, true, "system:auto_enable")
				skill.Enabled = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "created",
			"skill":      skill,
			"validation": validation,
		})
	}
}

// handleUpdateSkill updates a skill (enable/disable, description).
// PUT /api/skills/{id}
func handleUpdateSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		readOnly := s.Cfg.Tools.SkillManager.ReadOnly
		s.CfgMu.RUnlock()
		if readOnly {
			jsonError(w, "Skill Manager is in read-only mode", http.StatusForbidden)
			return
		}

		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}

		var req struct {
			Enabled        *bool     `json:"enabled"`
			Description    *string   `json:"description"`
			Category       *string   `json:"category"`
			Tags           *[]string `json:"tags"`
			Code           *string   `json:"code"`
			RestoreVersion *int      `json:"restore_version"`
			VaultKeys      []string  `json:"vault_keys"`
			InternalTools  []string  `json:"internal_tools"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Enabled != nil {
			// Prevent toggling built-in skills
			skillEntry, err := s.SkillManager.GetSkill(id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Skill lookup failed", err, "skill_id", id)
				return
			}
			if skillEntry.Type == tools.SkillTypeBuiltIn {
				jsonError(w, "Built-in skills cannot be toggled", http.StatusForbidden)
				return
			}
			if err := s.SkillManager.EnableSkill(id, *req.Enabled, "user"); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Failed to toggle skill", err, "skill_id", id)
				return
			}
		}

		if req.Description != nil || req.Category != nil || req.Tags != nil {
			currentSkill, err := s.SkillManager.GetSkill(id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Skill lookup failed", err, "skill_id", id)
				return
			}
			description := currentSkill.Description
			category := currentSkill.Category
			tags := currentSkill.Tags
			if req.Description != nil {
				description = *req.Description
			}
			if req.Category != nil {
				category = *req.Category
			}
			if req.Tags != nil {
				tags = *req.Tags
			}
			if err := s.SkillManager.UpdateSkillMetadata(id, description, category, tags, "user"); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update skill metadata", "Failed to update skill metadata", err, "skill_id", id)
				return
			}
		}

		if req.Code != nil {
			if err := s.SkillManager.UpdateSkillCode(id, *req.Code, "user"); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update skill code", "Failed to update skill code", err, "skill_id", id)
				return
			}
		}
		if req.RestoreVersion != nil {
			code, err := s.SkillManager.GetSkillVersionCode(id, *req.RestoreVersion)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to load skill version", "Failed to load skill version", err, "skill_id", id, "version", *req.RestoreVersion)
				return
			}
			if err := s.SkillManager.UpdateSkillCode(id, code, "user:restore"); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to restore skill version", "Failed to restore skill version", err, "skill_id", id, "version", *req.RestoreVersion)
				return
			}
		}

		if req.VaultKeys != nil {
			if err := s.SkillManager.UpdateVaultKeys(id, req.VaultKeys); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update skill vault keys", "Failed to update skill vault keys", err, "skill_id", id)
				return
			}
		}

		if req.InternalTools != nil {
			if err := s.SkillManager.UpdateInternalTools(id, req.InternalTools); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update skill internal tools", "Failed to update skill internal tools", err, "skill_id", id)
				return
			}
		}

		skill, err := s.SkillManager.GetSkill(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Skill lookup failed", err, "skill_id", id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"skill":  skill,
		})
	}
}

// handleDeleteSkill deletes a skill from the registry and disk.
// DELETE /api/skills/{id}
func handleDeleteSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		readOnly := s.Cfg.Tools.SkillManager.ReadOnly
		s.CfgMu.RUnlock()
		if readOnly {
			jsonError(w, "Skill Manager is in read-only mode", http.StatusForbidden)
			return
		}

		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}

		// Prevent deleting built-in skills
		skill, err := s.SkillManager.GetSkill(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Skill lookup failed", err, "skill_id", id)
			return
		}
		if skill.Type == tools.SkillTypeBuiltIn {
			jsonError(w, "Built-in skills cannot be deleted", http.StatusForbidden)
			return
		}

		deleteFiles := r.URL.Query().Get("delete_files") != "false"
		if err := s.SkillManager.DeleteSkill(id, deleteFiles, "user"); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Failed to delete skill", err, "skill_id", id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "deleted",
			"id":     id,
		})
	}
}

// handleUploadSkill handles multipart file upload of a Python skill.
// POST /api/skills/upload
func handleUploadSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		readOnly := s.Cfg.Tools.SkillManager.ReadOnly
		allowUploads := s.Cfg.Tools.SkillManager.AllowUploads
		maxSizeMB := s.Cfg.Tools.SkillManager.MaxUploadSizeMB
		requireScan := s.Cfg.Tools.SkillManager.RequireScan
		autoEnable := s.Cfg.Tools.SkillManager.AutoEnableClean
		useGuardian := s.Cfg.Tools.SkillManager.ScanWithGuardian
		vtEnabled := s.Cfg.VirusTotal.Enabled
		vtKey := s.Cfg.VirusTotal.APIKey
		s.CfgMu.RUnlock()

		if readOnly || !allowUploads {
			jsonError(w, "File upload is disabled", http.StatusForbidden)
			return
		}

		if maxSizeMB <= 0 {
			maxSizeMB = 1
		}

		// Limit upload size
		r.Body = http.MaxBytesReader(w, r.Body, int64(maxSizeMB)<<20)
		if err := r.ParseMultipartForm(int64(maxSizeMB) << 20); err != nil {
			jsonError(w, "File too large or invalid form data", http.StatusRequestEntityTooLarge)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "File is required", http.StatusBadRequest)
			return
		}
		defer file.Close()
		if !isAllowedSkillUploadFilename(header.Filename) {
			jsonError(w, "Only Python skill files (.py) are allowed", http.StatusBadRequest)
			return
		}

		fileData, err := io.ReadAll(file)
		if err != nil {
			s.Logger.Error("Failed to read uploaded skill", "filename", header.Filename, "error", err)
			jsonError(w, "Failed to process uploaded file", http.StatusInternalServerError)
			return
		}

		// Validate
		validation := tools.ValidateSkillUpload(fileData, header.Filename, maxSizeMB)
		if !validation.Passed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     "rejected",
				"validation": validation,
				"message":    "File validation failed",
			})
			return
		}

		// Determine name
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			name = strings.TrimSuffix(sanitizeFilename(header.Filename), ".py")
		}
		description := strings.TrimSpace(r.FormValue("description"))
		category := strings.TrimSpace(r.FormValue("category"))
		tags := splitCommaSeparated(r.FormValue("tags"))

		// Create entry
		skill, err := s.SkillManager.CreateSkillEntry(name, description, string(fileData), tools.SkillTypeUser, "user", category, tags)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				jsonError(w, "A skill with that name already exists", http.StatusConflict)
				return
			}
			s.Logger.Error("Failed to save uploaded skill", "name", name, "error", err)
			jsonError(w, "Failed to save skill", http.StatusInternalServerError)
			return
		}

		// Run security scan
		if requireScan {
			scanReport, secStatus, scanErr := s.SkillManager.ScanSkill(r.Context(), skill.ID, vtKey, s.LLMGuardian, vtEnabled, useGuardian)
			if scanErr != nil {
				s.Logger.Warn("Skill security scan failed", "id", skill.ID, "error", scanErr)
			}
			skill.SecurityStatus = secStatus
			skill.SecurityReport = scanReport

			if autoEnable && secStatus == tools.SecurityClean {
				s.SkillManager.EnableSkill(skill.ID, true, "system:auto_enable")
				skill.Enabled = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "uploaded",
			"skill":      skill,
			"validation": validation,
		})
	}
}

// handleVerifySkill triggers a security re-scan for a skill.
// POST /api/skills/{id}/verify
func handleVerifySkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}

		// Extract ID from path: /api/skills/{id}/verify
		path := strings.TrimPrefix(r.URL.Path, "/api/skills/")
		path = strings.TrimSuffix(path, "/verify")
		id := path
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}

		s.CfgMu.RLock()
		useGuardian := s.Cfg.Tools.SkillManager.ScanWithGuardian
		vtEnabled := s.Cfg.VirusTotal.Enabled
		vtKey := s.Cfg.VirusTotal.APIKey
		s.CfgMu.RUnlock()

		// Check for overrides in request body
		var req struct {
			ScanVirusTotal *bool `json:"scan_virustotal"`
			ScanGuardian   *bool `json:"scan_guardian"`
		}
		if r.Body != nil {
			json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
		}
		if req.ScanVirusTotal != nil {
			vtEnabled = vtEnabled && *req.ScanVirusTotal
		}
		if req.ScanGuardian != nil {
			useGuardian = *req.ScanGuardian
		}

		report, status, err := s.SkillManager.ScanSkill(r.Context(), id, vtKey, s.LLMGuardian, vtEnabled, useGuardian)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Security scan failed", "Skill security scan failed", err, "skill_id", id)
			return
		}

		s.CfgMu.RLock()
		autoEnable := s.Cfg.Tools.SkillManager.AutoEnableClean
		s.CfgMu.RUnlock()

		if autoEnable && status == tools.SecurityClean {
			s.SkillManager.EnableSkill(id, true, "system:auto_enable")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "scanned",
			"security_status": status,
			"report":          report,
		})
	}
}

// handleGetSkillVersions returns the stored version history for a skill.
// GET /api/skills/{id}/versions
func handleGetSkillVersions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		versions, err := s.SkillManager.ListSkillVersions(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to load skill versions", "Failed to load skill versions", err, "skill_id", id)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":   "ok",
			"versions": versions,
		})
	}
}

// handleGetSkillAudit returns the audit trail for a skill.
// GET /api/skills/{id}/audit
func handleGetSkillAudit(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			fmt.Sscanf(raw, "%d", &limit)
		}
		entries, err := s.SkillManager.ListSkillAudit(id, limit)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to load skill audit", "Failed to load skill audit", err, "skill_id", id)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"audit":  entries,
		})
	}
}

func isAllowedSkillUploadFilename(filename string) bool {
	name := strings.TrimSpace(filename)
	return name != "" && strings.HasSuffix(strings.ToLower(name), ".py")
}

func splitCommaSeparated(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
