package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

// handleListSkills returns all skills with optional filters.
// GET /api/skills?type=agent&status=clean&enabled=true&search=foo
func handleListSkills(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, "Failed to list skills: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if skills == nil {
			skills = []tools.SkillRegistryEntry{}
		}

		// Get stats
		total, agentCount, userCount, pending, _ := s.SkillManager.GetStats()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"count":  len(skills),
			"skills": skills,
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}

		// Include code if requested
		var code string
		if r.URL.Query().Get("code") == "true" {
			code, _ = s.SkillManager.GetSkillCode(id)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"skill":  skill,
			"code":   code,
		})
	}
}

// handleCreateSkill creates a new skill from JSON body with inline code.
// POST /api/skills
func handleCreateSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			Name        string `json:"name"`
			Description string `json:"description"`
			Code        string `json:"code"`
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

		skill, err := s.SkillManager.CreateSkillEntry(req.Name, req.Description, req.Code, tools.SkillTypeUser, "user")
		if err != nil {
			jsonError(w, "Failed to create skill: "+err.Error(), http.StatusInternalServerError)
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
				s.SkillManager.EnableSkill(skill.ID, true)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			Enabled     *bool  `json:"enabled"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Enabled != nil {
			// Prevent toggling built-in skills
			skillEntry, err := s.SkillManager.GetSkill(id)
			if err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
			if skillEntry.Type == tools.SkillTypeBuiltIn {
				jsonError(w, "Built-in skills cannot be toggled", http.StatusForbidden)
				return
			}
			if err := s.SkillManager.EnableSkill(id, *req.Enabled); err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
		}

		skill, err := s.SkillManager.GetSkill(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		if skill.Type == tools.SkillTypeBuiltIn {
			jsonError(w, "Built-in skills cannot be deleted", http.StatusForbidden)
			return
		}

		deleteFiles := r.URL.Query().Get("delete_files") != "false"
		if err := s.SkillManager.DeleteSkill(id, deleteFiles); err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

		fileData, err := io.ReadAll(file)
		if err != nil {
			jsonError(w, "Failed to read uploaded file", http.StatusInternalServerError)
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
			name = strings.TrimSuffix(header.Filename, ".py")
		}
		description := strings.TrimSpace(r.FormValue("description"))

		// Create entry
		skill, err := s.SkillManager.CreateSkillEntry(name, description, string(fileData), tools.SkillTypeUser, "user")
		if err != nil {
			jsonError(w, "Failed to save skill: "+err.Error(), http.StatusInternalServerError)
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
				s.SkillManager.EnableSkill(skill.ID, true)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, "Security scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.CfgMu.RLock()
		autoEnable := s.Cfg.Tools.SkillManager.AutoEnableClean
		s.CfgMu.RUnlock()

		if autoEnable && status == tools.SecurityClean {
			s.SkillManager.EnableSkill(id, true)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "scanned",
			"security_status": status,
			"report":          report,
		})
	}
}

// handleSkillTemplates returns available skill templates.
// GET /api/skills/templates
func handleSkillTemplates(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		templates := tools.AvailableSkillTemplates()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"templates": templates,
		})
	}
}

// handleCreateSkillFromTemplate creates a skill from a built-in template.
// POST /api/skills/templates
func handleCreateSkillFromTemplate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		readOnly := s.Cfg.Tools.SkillManager.ReadOnly
		allowUploads := s.Cfg.Tools.SkillManager.AllowUploads
		skillsDir := s.Cfg.Directories.SkillsDir
		s.CfgMu.RUnlock()

		if readOnly || !allowUploads {
			jsonError(w, "Skill creation is disabled", http.StatusForbidden)
			return
		}

		var req struct {
			TemplateName string   `json:"template_name"`
			SkillName    string   `json:"skill_name"`
			Description  string   `json:"description"`
			BaseURL      string   `json:"base_url"`
			Dependencies []string `json:"dependencies"`
			VaultKeys    []string `json:"vault_keys"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.TemplateName == "" || req.SkillName == "" {
			jsonError(w, "template_name and skill_name are required", http.StatusBadRequest)
			return
		}

		result, err := tools.CreateSkillFromTemplate(skillsDir, req.TemplateName, req.SkillName, req.Description, req.BaseURL, req.Dependencies, req.VaultKeys)
		if err != nil {
			jsonError(w, "Failed to create skill from template: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Sync to registry
		s.SkillManager.SyncFromDisk()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "created",
			"message": result,
		})
	}
}

// handleSkillStats returns skill statistics for the dashboard.
// GET /api/skills/stats
func handleSkillStats(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"enabled": false,
			})
			return
		}

		total, agentCount, userCount, pending, err := s.SkillManager.GetStats()
		if err != nil {
			jsonError(w, "Failed to get stats", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"enabled": true,
			"total":   total,
			"agent":   agentCount,
			"user":    userCount,
			"pending": pending,
		})
	}
}

// extractSkillPathID extracts the resource ID from a URL path after a given prefix.
func extractSkillPathID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	// Remove trailing slash
	id = strings.TrimSuffix(id, "/")
	// Stop at the next slash (sub-path like /verify)
	if idx := strings.Index(id, "/"); idx >= 0 {
		id = id[:idx]
	}
	return id
}
