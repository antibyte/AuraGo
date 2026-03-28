package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/llm"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
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
			Enabled        *bool     `json:"enabled"`
			Description    *string   `json:"description"`
			Category       *string   `json:"category"`
			Tags           *[]string `json:"tags"`
			Code           *string   `json:"code"`
			RestoreVersion *int      `json:"restore_version"`
			VaultKeys      []string  `json:"vault_keys"`
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
			if err := s.SkillManager.EnableSkill(id, *req.Enabled, "user"); err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
		}

		if req.Description != nil || req.Category != nil || req.Tags != nil {
			currentSkill, err := s.SkillManager.GetSkill(id)
			if err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
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
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if req.Code != nil {
			if err := s.SkillManager.UpdateSkillCode(id, *req.Code, "user"); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if req.RestoreVersion != nil {
			code, err := s.SkillManager.GetSkillVersionCode(id, *req.RestoreVersion)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := s.SkillManager.UpdateSkillCode(id, code, "user:restore"); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if req.VaultKeys != nil {
			if err := s.SkillManager.UpdateVaultKeys(id, req.VaultKeys); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
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
		if err := s.SkillManager.DeleteSkill(id, deleteFiles, "user"); err != nil {
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
		category := strings.TrimSpace(r.FormValue("category"))
		tags := splitCommaSeparated(r.FormValue("tags"))

		// Create entry
		skill, err := s.SkillManager.CreateSkillEntry(name, description, string(fileData), tools.SkillTypeUser, "user", category, tags)
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
			Category     string   `json:"category"`
			Tags         []string `json:"tags"`
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

		// Look up the newly created skill to return its ID
		var skillID string
		skills, _ := s.SkillManager.ListSkillsFiltered("", "", req.SkillName, nil)
		for _, sk := range skills {
			if sk.Name == req.SkillName {
				skillID = sk.ID
				break
			}
		}
		if skillID != "" {
			_ = s.SkillManager.EnsureInitialVersion(skillID, "system", "template creation")
			if req.Description != "" || req.Category != "" || len(req.Tags) > 0 {
				if currentSkill, metaErr := s.SkillManager.GetSkill(skillID); metaErr == nil {
					description := currentSkill.Description
					if req.Description != "" {
						description = req.Description
					}
					_ = s.SkillManager.UpdateSkillMetadata(skillID, description, req.Category, req.Tags, "user")
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "created",
			"message":  result,
			"skill_id": skillID,
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

// handleGetSkillVersions returns the stored version history for a skill.
// GET /api/skills/{id}/versions
func handleGetSkillVersions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		versions, err := s.SkillManager.ListSkillVersions(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"audit":  entries,
		})
	}
}

// handleExportSkill exports a skill as an AuraGo skill bundle.
// GET /api/skills/{id}/export
func handleExportSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		bundle, err := s.SkillManager.ExportSkillBundle(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		filename := bundle.Skill.Name + ".aurago-skill.json"
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		json.NewEncoder(w).Encode(bundle)
	}
}

// handleImportSkill imports an exported AuraGo skill bundle.
// POST /api/skills/import
func handleImportSkill(s *Server) http.HandlerFunc {
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
			jsonError(w, "Skill import is disabled", http.StatusForbidden)
			return
		}

		var bundle tools.SkillExportBundle
		if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&bundle); err != nil {
			jsonError(w, "Invalid skill bundle", http.StatusBadRequest)
			return
		}
		entry, err := s.SkillManager.ImportSkillBundle(&bundle, "user")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "imported",
			"skill":  entry,
		})
	}
}

// handleTestSkill executes a skill with a JSON payload and returns the raw output.
// POST /api/skills/{id}/test
func handleTestSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/skills/")
		if id == "" {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}
		skill, err := s.SkillManager.GetSkill(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}

		var req struct {
			Args map[string]interface{} `json:"args"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && err != io.EOF {
				jsonError(w, "Invalid request body", http.StatusBadRequest)
				return
			}
		}
		if req.Args == nil {
			req.Args = map[string]interface{}{}
		}

		secrets := loadPlainSkillSecrets(s, skill)
		var output string
		if len(secrets) > 0 {
			output, err = tools.ExecuteSkillWithSecrets(s.Cfg.Directories.SkillsDir, s.Cfg.Directories.WorkspaceDir, skill.Name, req.Args, secrets, nil)
		} else {
			output, err = tools.ExecuteSkill(s.Cfg.Directories.SkillsDir, s.Cfg.Directories.WorkspaceDir, skill.Name, req.Args)
		}
		status := "ok"
		message := ""
		if err != nil {
			status = "error"
			message = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  status,
			"output":  output,
			"message": message,
		})
	}
}

// handleGenerateSkillDraft asks the configured LLM for a draft AuraGo skill.
// POST /api/skills/generate
func handleGenerateSkillDraft(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		if s.LLMClient == nil {
			jsonError(w, "LLM is not available", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Prompt       string   `json:"prompt"`
			SkillName    string   `json:"skill_name"`
			TemplateName string   `json:"template_name"`
			Category     string   `json:"category"`
			Dependencies []string `json:"dependencies"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req.Prompt = strings.TrimSpace(req.Prompt)
		if req.Prompt == "" {
			jsonError(w, "prompt is required", http.StatusBadRequest)
			return
		}
		s.Logger.Info("[Skills] Generating AI draft",
			"prompt_len", len(req.Prompt),
			"skill_name", req.SkillName,
			"template", req.TemplateName,
			"category", req.Category,
			"dependency_count", len(req.Dependencies),
		)

		templateHint := ""
		if req.TemplateName != "" {
			for _, tmpl := range tools.AvailableSkillTemplates() {
				if strings.EqualFold(tmpl.Name, req.TemplateName) {
					templateHint = fmt.Sprintf("Prefer the built-in template '%s'. Description: %s. Default deps: %s.",
						tmpl.Name, tmpl.Description, strings.Join(tmpl.Dependencies, ", "))
					break
				}
			}
		}
		userNameHint := ""
		if req.SkillName != "" {
			userNameHint = fmt.Sprintf("Use the exact skill name '%s'.", req.SkillName)
		}
		categoryHint := ""
		if req.Category != "" {
			categoryHint = fmt.Sprintf("Preferred category: %s.", req.Category)
		}
		depHint := ""
		if len(req.Dependencies) > 0 {
			depHint = fmt.Sprintf("Requested dependencies: %s.", strings.Join(req.Dependencies, ", "))
		}
		systemPrompt := "You generate AuraGo Python skills. Return JSON only, no markdown fences. " +
			"Schema: {\"name\":\"...\",\"description\":\"...\",\"category\":\"...\",\"tags\":[...],\"dependencies\":[...],\"code\":\"...\"}. " +
			"Code rules: read JSON from stdin, write exactly one JSON object to stdout, do not use destructive operations, keep code compact and production-ready. " +
			"The skill runs inside AuraGo's execute_skill Python sandbox. Never use subprocess, os.system, os.popen, shell commands, ping, curl, nmap, sudo, or external CLIs. " +
			"Prefer Python stdlib or direct libraries/APIs instead of shelling out. Handle errors inside the JSON response instead of crashing."
		userPrompt := strings.TrimSpace(strings.Join([]string{
			req.Prompt,
			userNameHint,
			templateHint,
			categoryHint,
			depHint,
		}, "\n"))
		llmReq := openai.ChatCompletionRequest{
			Model: s.Cfg.LLM.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
			Temperature: 0.2,
		}
		ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
		defer cancel()
		resp, err := llm.ExecuteWithRetry(ctx, s.LLMClient, llmReq, s.Logger, nil)
		if err != nil {
			s.Logger.Warn("[Skills] AI draft generation failed", "error", err)
			jsonError(w, "LLM generation failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if len(resp.Choices) == 0 {
			s.Logger.Warn("[Skills] AI draft generation returned no choices")
			jsonError(w, "LLM generation returned no response", http.StatusBadGateway)
			return
		}
		draft, err := decodeSkillDraft(resp.Choices[0].Message.Content)
		if err != nil {
			s.Logger.Warn("[Skills] Failed to decode generated skill draft", "error", err)
			jsonError(w, "Failed to parse generated skill draft: "+err.Error(), http.StatusBadGateway)
			return
		}
		if repaired, repairApplied, repairReason, repairErr := maybeRepairGeneratedSkillDraft(ctx, s, llmReq.Model, draft); repairErr != nil {
			s.Logger.Warn("[Skills] Failed to repair generated skill draft", "error", repairErr)
		} else if repairApplied {
			draft = repaired
			s.Logger.Info("[Skills] Repaired generated skill draft", "name", draft.Name, "reason", repairReason)
		}
		if req.SkillName != "" {
			draft.Name = req.SkillName
		}
		if req.Category != "" {
			draft.Category = req.Category
		}
		if len(req.Dependencies) > 0 {
			draft.Dependencies = req.Dependencies
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"draft":  draft,
		})
		s.Logger.Info("[Skills] AI draft generated successfully", "name", draft.Name, "category", draft.Category)
	}
}

type generatedSkillDraft struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Dependencies []string `json:"dependencies"`
	Code         string   `json:"code"`
}

func decodeSkillDraft(raw string) (*generatedSkillDraft, error) {
	candidates, err := extractJSONObjectCandidates(raw)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, obj := range candidates {
		draft, parseErr := parseGeneratedSkillDraft([]byte(obj))
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		if strings.TrimSpace(draft.Name) == "" {
			lastErr = fmt.Errorf("draft name is missing")
			continue
		}
		if strings.TrimSpace(draft.Code) == "" {
			lastErr = fmt.Errorf("draft code is missing")
			continue
		}
		return &draft, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no valid skill draft JSON object found")
}

func parseGeneratedSkillDraft(raw []byte) (generatedSkillDraft, error) {
	draft, err := parseGeneratedSkillDraftStrict(raw)
	if err == nil {
		return draft, nil
	}
	lastErr := err

	normalized := normalizeLooseJSON(raw)
	if !bytes.Equal(normalized, raw) {
		draft, err = parseGeneratedSkillDraftStrict(normalized)
		if err == nil {
			return draft, nil
		}
		lastErr = err
	}

	return generatedSkillDraft{}, lastErr
}

func parseGeneratedSkillDraftStrict(raw []byte) (generatedSkillDraft, error) {
	var direct generatedSkillDraft
	if err := json.Unmarshal(raw, &direct); err == nil {
		direct.Name = strings.TrimSpace(direct.Name)
		direct.Description = strings.TrimSpace(direct.Description)
		direct.Category = strings.TrimSpace(direct.Category)
		direct.Tags = normalizeStringList(direct.Tags)
		direct.Dependencies = normalizeStringList(direct.Dependencies)
		direct.Code = strings.TrimSpace(direct.Code)
		if direct.Name != "" || direct.Code != "" || direct.Description != "" || direct.Category != "" || len(direct.Tags) > 0 || len(direct.Dependencies) > 0 {
			return direct, nil
		}
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return generatedSkillDraft{}, err
	}
	if nested, ok := generic["draft"].(map[string]any); ok {
		generic = nested
	}

	draft := generatedSkillDraft{
		Name:         firstNonEmptyString(generic, "name", "skill_name", "skill"),
		Description:  firstNonEmptyString(generic, "description", "summary"),
		Category:     firstNonEmptyString(generic, "category"),
		Tags:         coerceStringList(generic["tags"]),
		Dependencies: coerceStringList(generic["dependencies"]),
		Code:         firstNonEmptyString(generic, "code", "python_code", "script"),
	}
	draft.Name = strings.TrimSpace(draft.Name)
	draft.Description = strings.TrimSpace(draft.Description)
	draft.Category = strings.TrimSpace(draft.Category)
	draft.Tags = normalizeStringList(draft.Tags)
	draft.Dependencies = normalizeStringList(draft.Dependencies)
	draft.Code = strings.TrimSpace(draft.Code)
	return draft, nil
}

func normalizeLooseJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}

	var out strings.Builder
	out.Grow(len(raw) + 16)

	inDouble := false
	inSingle := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inSingle {
			if escaped {
				switch ch {
				case '"':
					out.WriteByte('\\')
					out.WriteByte('"')
				case '\'':
					out.WriteByte('\'')
				default:
					out.WriteByte(ch)
				}
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '\'':
				inSingle = false
				out.WriteByte('"')
			case '"':
				out.WriteByte('\\')
				out.WriteByte('"')
			case '\n', '\r', '\t':
				out.WriteByte(' ')
			default:
				out.WriteByte(ch)
			}
			continue
		}
		if inDouble {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
			out.WriteByte('"')
		case '"':
			inDouble = true
			out.WriteByte(ch)
		default:
			out.WriteByte(ch)
		}
	}

	normalized := stripTrailingCommas(out.String())
	normalized = replaceBareJSONLiteral(normalized, "None", "null")
	normalized = replaceBareJSONLiteral(normalized, "True", "true")
	normalized = replaceBareJSONLiteral(normalized, "False", "false")
	return []byte(normalized)
}

func stripTrailingCommas(raw string) string {
	var out strings.Builder
	out.Grow(len(raw))
	inString := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}
		if ch == ',' {
			j := i + 1
			for j < len(raw) {
				switch raw[j] {
				case ' ', '\n', '\r', '\t':
					j++
					continue
				case '}', ']':
					goto skipComma
				}
				break
			}
		}
		out.WriteByte(ch)
		continue
	skipComma:
	}
	return out.String()
}

func replaceBareJSONLiteral(raw, from, to string) string {
	if raw == "" || !strings.Contains(raw, from) {
		return raw
	}
	var out strings.Builder
	out.Grow(len(raw))
	inString := false
	escaped := false

	for i := 0; i < len(raw); {
		ch := raw[i]
		if inString {
			out.WriteByte(ch)
			i++
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			i++
			continue
		}
		if strings.HasPrefix(raw[i:], from) && isJSONLiteralBoundary(raw, i-1) && isJSONLiteralBoundary(raw, i+len(from)) {
			out.WriteString(to)
			i += len(from)
			continue
		}
		out.WriteByte(ch)
		i++
	}
	return out.String()
}

func isJSONLiteralBoundary(raw string, idx int) bool {
	if idx < 0 || idx >= len(raw) {
		return true
	}
	ch := raw[idx]
	return !(ch == '_' || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z'))
}

func firstNonEmptyString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			switch v := value.(type) {
			case string:
				if s := strings.TrimSpace(v); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func coerceStringList(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		return splitCSVLike(v)
	case []string:
		return normalizeStringList(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
		return normalizeStringList(out)
	default:
		return nil
	}
}

func splitCSVLike(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return normalizeStringList(out)
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func maybeRepairGeneratedSkillDraft(ctx context.Context, s *Server, model string, draft *generatedSkillDraft) (*generatedSkillDraft, bool, string, error) {
	if draft == nil || s == nil || s.LLMClient == nil {
		return draft, false, "", nil
	}
	needsRepair, reason := generatedSkillNeedsRepair(draft)
	if !needsRepair {
		return draft, false, "", nil
	}

	repairPrompt := "You are repairing an AuraGo Python skill draft so it works inside AuraGo's execute_skill sandbox. " +
		"Return JSON only, no markdown fences. Use the same schema: " +
		"{\"name\":\"...\",\"description\":\"...\",\"category\":\"...\",\"tags\":[...],\"dependencies\":[...],\"code\":\"...\"}. " +
		"Keep the functionality, but remove sandbox-incompatible behavior. " +
		"Never use subprocess, os.system, os.popen, shell commands, ping, curl, nmap, sudo, or external CLIs. " +
		"Use Python stdlib or direct libraries/APIs, read JSON from stdin, and write exactly one JSON object to stdout."
	rawDraft, err := json.Marshal(draft)
	if err != nil {
		return draft, false, reason, fmt.Errorf("marshal draft for repair: %w", err)
	}
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: repairPrompt},
			{
				Role: openai.ChatMessageRoleUser,
				Content: "Repair this generated skill draft. " +
					"Problem: " + reason + "\n\nDraft JSON:\n" + string(rawDraft),
			},
		},
		Temperature: 0.1,
	}
	resp, err := llm.ExecuteWithRetry(ctx, s.LLMClient, req, s.Logger, nil)
	if err != nil {
		return draft, false, reason, fmt.Errorf("repair LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return draft, false, reason, fmt.Errorf("repair LLM returned no response")
	}
	repaired, err := decodeSkillDraft(resp.Choices[0].Message.Content)
	if err != nil {
		return draft, false, reason, fmt.Errorf("repair draft parse failed: %w", err)
	}
	return repaired, true, reason, nil
}

func generatedSkillNeedsRepair(draft *generatedSkillDraft) (bool, string) {
	if draft == nil {
		return false, ""
	}
	code := strings.TrimSpace(draft.Code)
	if code == "" {
		return true, "generated draft has no code"
	}

	validation := tools.ValidateSkillUpload([]byte(code), safeSkillFilename(draft.Name), 1)
	if validation != nil {
		var reasons []string
		for _, finding := range validation.Findings {
			category := strings.ToLower(strings.TrimSpace(finding.Category))
			pattern := strings.ToLower(strings.TrimSpace(finding.Pattern))
			message := strings.TrimSpace(finding.Message)
			if category == "exec" || pattern == "import_subprocess" || strings.Contains(strings.ToLower(message), "shell command") || strings.Contains(strings.ToLower(message), "subprocess") {
				reasons = append(reasons, message)
			}
		}
		if len(reasons) > 0 {
			return true, strings.Join(normalizeStringList(reasons), "; ")
		}
	}

	lower := strings.ToLower(code)
	if strings.Contains(lower, "subprocess.") || strings.Contains(lower, "import subprocess") {
		return true, "generated draft uses subprocess which is unreliable inside the execute_skill sandbox"
	}
	if strings.Contains(lower, "os.system(") || strings.Contains(lower, "os.popen(") {
		return true, "generated draft shells out to the operating system instead of using Python APIs"
	}
	if strings.Contains(lower, "'ping'") || strings.Contains(lower, "\"ping\"") {
		return true, "generated draft depends on the external ping command instead of direct Python networking"
	}
	return false, ""
}

func safeSkillFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "generated_skill"
	}
	return name + ".py"
}

func extractJSONObject(raw string) (string, error) {
	candidates, err := extractJSONObjectCandidates(raw)
	if err != nil {
		return "", err
	}
	return candidates[0], nil
}

func extractJSONObjectCandidates(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return nil, fmt.Errorf("no JSON object found")
	}
	depth := 0
	inString := false
	escaped := false
	objectStart := -1
	candidates := make([]string, 0, 2)
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				objectStart = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && objectStart >= 0 {
				candidates = append(candidates, raw[objectStart:i+1])
				objectStart = -1
			}
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("unterminated JSON object")
	}
	return candidates, nil
}

func loadPlainSkillSecrets(s *Server, skill *tools.SkillRegistryEntry) map[string]string {
	if s.Vault == nil || skill == nil {
		return nil
	}
	secrets := make(map[string]string)
	for _, key := range skill.VaultKeys {
		if strings.HasPrefix(key, "cred:") {
			continue
		}
		value, err := s.Vault.ReadSecret(key)
		if err != nil || value == "" {
			continue
		}
		secrets[key] = value
	}
	return secrets
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
