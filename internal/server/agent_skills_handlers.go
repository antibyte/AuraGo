package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"aurago/internal/tools"
)

func handleListAgentSkills(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := s.AgentSkillManager
		if mgr == nil {
			jsonError(w, "Agent Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		q := r.URL.Query()
		enabledOnly := q.Get("enabled") == "true" || q.Get("enabled") == "1"
		skills, err := mgr.ListAgentSkills(enabledOnly, q.Get("search"))
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list Agent Skills", "Failed to list Agent Skills", err)
			return
		}
		if skills == nil {
			skills = []tools.AgentSkillRegistryEntry{}
		}
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
			"count":  len(skills),
			"skills": skills,
		})
	}
}

func handleCreateAgentSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := s.AgentSkillManager
		if mgr == nil {
			jsonError(w, "Agent Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		readOnly, allowUploads, _, useGuardian := agentSkillManagerConfig(s)
		if readOnly || !allowUploads {
			jsonError(w, "Agent Skill creation is disabled", http.StatusForbidden)
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Body        string `json:"body"`
			Content     string `json:"content"`
			SkillMD     string `json:"skill_md"`
			Resources   []struct {
				Path     string `json:"path"`
				Content  string `json:"content"`
				Binary   bool   `json:"binary"`
			} `json:"resources"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		body := req.Body
		if body == "" {
			body = req.SkillMD
		}
		if body == "" {
			body = req.Content
		}
		entry, err := mgr.CreateAgentSkill(r.Context(), req.Name, req.Description, body, "user", s.LLMGuardian, useGuardian)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				status = http.StatusConflict
			}
			jsonLoggedError(w, s.Logger, status, "Failed to create Agent Skill", "Failed to create Agent Skill", err, "agent_skill", req.Name)
			return
		}
		if len(req.Resources) > 0 {
			for _, res := range req.Resources {
				if res.Binary {
					data, decErr := base64.StdEncoding.DecodeString(res.Content)
					if decErr != nil {
						_ = mgr.DeleteAgentSkill(entry.ID, true, "user")
						jsonError(w, "Invalid base64 content in resource "+res.Path, http.StatusBadRequest)
						return
					}
					if wErr := mgr.CreateAgentSkillFile(r.Context(), entry.ID, res.Path, data, true, "user", s.LLMGuardian, useGuardian); wErr != nil {
						_ = mgr.DeleteAgentSkill(entry.ID, true, "user")
						jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to write resource", "Failed to write resource file", wErr, "agent_skill", req.Name, "path", res.Path)
						return
					}
				} else {
					if wErr := mgr.CreateAgentSkillFile(r.Context(), entry.ID, res.Path, []byte(res.Content), false, "user", s.LLMGuardian, useGuardian); wErr != nil {
						_ = mgr.DeleteAgentSkill(entry.ID, true, "user")
						jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to write resource", "Failed to write resource file", wErr, "agent_skill", req.Name, "path", res.Path)
						return
					}
				}
			}
			entry, _ = mgr.GetAgentSkill(entry.ID)
		}
		entry = autoEnableCleanAgentSkill(s, mgr, entry)
		writeAgentSkillJSON(w, http.StatusCreated, map[string]interface{}{
			"status": "created",
			"skill":  entry,
		})
	}
}

func handleImportAgentSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mgr := s.AgentSkillManager
		if mgr == nil {
			jsonError(w, "Agent Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		readOnly, allowUploads, maxSizeMB, useGuardian := agentSkillManagerConfig(s)
		if readOnly || !allowUploads {
			jsonError(w, "Agent Skill import is disabled", http.StatusForbidden)
			return
		}
		if maxSizeMB <= 0 {
			maxSizeMB = 1
		}

		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
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
			if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
				jsonError(w, "Only ZIP Agent Skill packages are allowed", http.StatusBadRequest)
				return
			}
			data, err := io.ReadAll(file)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to read upload", "Failed to read Agent Skill upload", err)
				return
			}
			entry, validation, err := mgr.ImportAgentSkillZIP(r.Context(), data, "user", s.LLMGuardian, useGuardian)
			if err != nil {
				writeAgentSkillJSON(w, http.StatusBadRequest, map[string]interface{}{
					"status":     "rejected",
					"validation": validation,
					"message":    err.Error(),
				})
				return
			}
			entry = autoEnableCleanAgentSkill(s, mgr, entry)
			writeAgentSkillJSON(w, http.StatusCreated, map[string]interface{}{
				"status":     "imported",
				"skill":      entry,
				"validation": validation,
			})
			return
		}

		var req struct {
			SourcePath string `json:"source_path"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		entry, validation, err := mgr.ImportAgentSkillDirectory(r.Context(), req.SourcePath, "user", s.LLMGuardian, useGuardian)
		if err != nil {
			writeAgentSkillJSON(w, http.StatusBadRequest, map[string]interface{}{
				"status":     "rejected",
				"validation": validation,
				"message":    err.Error(),
			})
			return
		}
		entry = autoEnableCleanAgentSkill(s, mgr, entry)
		writeAgentSkillJSON(w, http.StatusCreated, map[string]interface{}{
			"status":     "imported",
			"skill":      entry,
			"validation": validation,
		})
	}
}

func handleAgentSkillItem(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mgr := s.AgentSkillManager
		if mgr == nil {
			jsonError(w, "Agent Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		if id == "" {
			jsonError(w, "Agent Skill ID is required", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			entry, err := mgr.GetAgentSkill(id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Agent Skill not found", "Agent Skill lookup failed", err, "agent_skill_id", id)
				return
			}
			resp := map[string]interface{}{"status": "ok", "skill": entry}
			if r.URL.Query().Get("content") == "true" || r.URL.Query().Get("skill_md") == "true" {
				content, err := mgr.ReadAgentSkillFile(id, "SKILL.md")
				if err != nil {
					jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to read Agent Skill file", "Failed to read Agent Skill file", err, "agent_skill_id", id)
					return
				}
				resp["content"] = content
			}
			writeAgentSkillJSON(w, http.StatusOK, resp)
		case http.MethodPut:
			handleUpdateAgentSkill(s)(w, r)
		case http.MethodDelete:
			handleDeleteAgentSkill(s)(w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleUpdateAgentSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mgr := s.AgentSkillManager
		readOnly, _, _, useGuardian := agentSkillManagerConfig(s)
		if readOnly {
			jsonError(w, "Agent Skill Manager is in read-only mode", http.StatusForbidden)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		var req struct {
			Enabled *bool   `json:"enabled"`
			Content *string `json:"content"`
			SkillMD *string `json:"skill_md"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Content != nil {
			if err := mgr.WriteAgentSkillFile(r.Context(), id, "SKILL.md", *req.Content, "user", s.LLMGuardian, useGuardian); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update Agent Skill", "Failed to update Agent Skill", err, "agent_skill_id", id)
				return
			}
		}
		if req.SkillMD != nil {
			if err := mgr.WriteAgentSkillFile(r.Context(), id, "SKILL.md", *req.SkillMD, "user", s.LLMGuardian, useGuardian); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update Agent Skill", "Failed to update Agent Skill", err, "agent_skill_id", id)
				return
			}
		}
		if req.Enabled != nil {
			if err := mgr.EnableAgentSkill(id, *req.Enabled, "user"); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update Agent Skill", "Failed to toggle Agent Skill", err, "agent_skill_id", id)
				return
			}
		}
		entry, err := mgr.GetAgentSkill(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Agent Skill not found", "Agent Skill lookup failed", err, "agent_skill_id", id)
			return
		}
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "skill": entry})
	}
}

func handleDeleteAgentSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		readOnly, _, _, _ := agentSkillManagerConfig(s)
		if readOnly {
			jsonError(w, "Agent Skill Manager is in read-only mode", http.StatusForbidden)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		deleteFiles := r.URL.Query().Get("delete_files") != "false"
		if err := s.AgentSkillManager.DeleteAgentSkill(id, deleteFiles, "user"); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Agent Skill not found", "Failed to delete Agent Skill", err, "agent_skill_id", id)
			return
		}
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "id": id})
	}
}

func handleVerifyAgentSkill(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		_, _, _, useGuardian := agentSkillManagerConfig(s)
		entry, err := s.AgentSkillManager.VerifyAgentSkill(r.Context(), id, "user", s.LLMGuardian, useGuardian)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Agent Skill verification failed", "Agent Skill verification failed", err, "agent_skill_id", id)
			return
		}
		entry = autoEnableCleanAgentSkill(s, s.AgentSkillManager, entry)
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{
			"status":          "scanned",
			"skill":           entry,
			"security_status": entry.SecurityStatus,
			"report":          entry.SecurityReport,
		})
	}
}

func handleApproveAgentSkillWarning(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		readOnly, _, _, _ := agentSkillManagerConfig(s)
		if readOnly {
			jsonError(w, "Agent Skill Manager is in read-only mode", http.StatusForbidden)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		if err := s.AgentSkillManager.ApproveAgentSkillWarning(id, "user"); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to approve Agent Skill warning", "Failed to approve Agent Skill warning", err, "agent_skill_id", id)
			return
		}
		entry, err := s.AgentSkillManager.GetAgentSkill(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Agent Skill not found", "Agent Skill lookup failed", err, "agent_skill_id", id)
			return
		}
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "approved", "skill": entry})
	}
}

func handleAgentSkillFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		relPath := r.URL.Query().Get("path")
		switch r.Method {
		case http.MethodGet:
			if relPath == "" {
				jsonError(w, "File path is required", http.StatusBadRequest)
				return
			}
			content, err := s.AgentSkillManager.ReadAgentSkillFile(id, relPath)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to read Agent Skill file", "Failed to read Agent Skill file", err, "agent_skill_id", id, "path", relPath)
				return
			}
			writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "path": relPath, "content": content})
		case http.MethodPut, http.MethodPost:
			readOnly, _, _, useGuardian := agentSkillManagerConfig(s)
			if readOnly {
				jsonError(w, "Agent Skill Manager is in read-only mode", http.StatusForbidden)
				return
			}
			var req struct {
				Path     string `json:"path"`
				Content  string `json:"content"`
				Binary   bool   `json:"binary"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
				jsonError(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if relPath == "" {
				relPath = req.Path
			}
			if relPath == "" {
				jsonError(w, "File path is required", http.StatusBadRequest)
				return
			}
			if req.Binary {
				data, err := base64.StdEncoding.DecodeString(req.Content)
				if err != nil {
					jsonError(w, "Invalid base64 content", http.StatusBadRequest)
					return
				}
				if err := s.AgentSkillManager.CreateAgentSkillFile(r.Context(), id, relPath, data, true, "user", s.LLMGuardian, useGuardian); err != nil {
					if strings.Contains(err.Error(), "already exists") {
						if err := s.AgentSkillManager.WriteAgentSkillFileBytes(r.Context(), id, relPath, data, "user", s.LLMGuardian, useGuardian); err != nil {
							jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to write Agent Skill file", "Failed to write Agent Skill file", err, "agent_skill_id", id, "path", relPath)
							return
						}
					} else {
						jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to write Agent Skill file", "Failed to write Agent Skill file", err, "agent_skill_id", id, "path", relPath)
						return
					}
				}
			} else {
				if err := s.AgentSkillManager.WriteAgentSkillFile(r.Context(), id, relPath, req.Content, "user", s.LLMGuardian, useGuardian); err != nil {
					jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to write Agent Skill file", "Failed to write Agent Skill file", err, "agent_skill_id", id, "path", relPath)
					return
				}
			}
			entry, _ := s.AgentSkillManager.GetAgentSkill(id)
			writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "saved", "skill": entry})
		case http.MethodDelete:
			readOnly, _, _, _ := agentSkillManagerConfig(s)
			if readOnly {
				jsonError(w, "Agent Skill Manager is in read-only mode", http.StatusForbidden)
				return
			}
			if relPath == "" {
				jsonError(w, "File path is required", http.StatusBadRequest)
				return
			}
			if err := s.AgentSkillManager.DeleteAgentSkillFile(r.Context(), id, relPath, "user"); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to delete Agent Skill file", "Failed to delete Agent Skill file", err, "agent_skill_id", id, "path", relPath)
				return
			}
			entry, _ := s.AgentSkillManager.GetAgentSkill(id)
			writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "path": relPath, "skill": entry})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleAgentSkillFileRename(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		readOnly, _, _, _ := agentSkillManagerConfig(s)
		if readOnly {
			jsonError(w, "Agent Skill Manager is in read-only mode", http.StatusForbidden)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		var req struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.From == "" || req.To == "" {
			jsonError(w, "Both 'from' and 'to' paths are required", http.StatusBadRequest)
			return
		}
		if err := s.AgentSkillManager.RenameAgentSkillFile(r.Context(), id, req.From, req.To, "user"); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to rename Agent Skill file", "Failed to rename Agent Skill file", err, "agent_skill_id", id)
			return
		}
		entry, _ := s.AgentSkillManager.GetAgentSkill(id)
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{"status": "renamed", "from": req.From, "to": req.To, "skill": entry})
	}
}

func handleAgentSkillFileUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		readOnly, allowUploads, maxSizeMB, useGuardian := agentSkillManagerConfig(s)
		if readOnly || !allowUploads {
			jsonError(w, "Agent Skill file upload is disabled", http.StatusForbidden)
			return
		}
		if maxSizeMB <= 0 {
			maxSizeMB = 1
		}
		_, allowBinary, _ := agentSkillManagerBinaryConfig(s)
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
		relPath := r.FormValue("path")
		if relPath == "" {
			relPath = header.Filename
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(maxSizeMB)<<20+1))
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to read upload", "Failed to read Agent Skill file upload", err)
			return
		}
		isBinary := !isUTF8(data)
		if isBinary && !allowBinary {
			jsonError(w, "Binary asset uploads are disabled", http.StatusForbidden)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		if err := s.AgentSkillManager.CreateAgentSkillFile(r.Context(), id, relPath, data, isBinary, "user", s.LLMGuardian, useGuardian); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				if err := s.AgentSkillManager.WriteAgentSkillFileBytes(r.Context(), id, relPath, data, "user", s.LLMGuardian, useGuardian); err != nil {
					jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to write Agent Skill file", "Failed to write Agent Skill file", err, "agent_skill_id", id, "path", relPath)
					return
				}
			} else {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to upload Agent Skill file", "Failed to upload Agent Skill file", err, "agent_skill_id", id, "path", relPath)
				return
			}
		}
		entry, _ := s.AgentSkillManager.GetAgentSkill(id)
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "uploaded",
			"path":     relPath,
			"binary":   isBinary,
			"size":     len(data),
			"skill":    entry,
		})
	}
}

func handleAgentSkillFileRaw(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			jsonError(w, "File path is required", http.StatusBadRequest)
			return
		}
		data, isBinary, err := s.AgentSkillManager.ReadAgentSkillFileBytes(id, relPath)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to read Agent Skill file", "Failed to read Agent Skill file", err, "agent_skill_id", id, "path", relPath)
			return
		}
		ext := filepath.Ext(relPath)
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			if isBinary {
				contentType = "application/octet-stream"
			} else {
				contentType = "text/plain; charset=utf-8"
			}
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(relPath)+"\"")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func isUTF8(data []byte) bool {
	for i := 0; i < len(data); {
		r, size := rune(data[i]), 1
		if r < 0x80 {
			i++
			continue
		} else if r&0xE0 == 0xC0 {
			if i+1 >= len(data) || data[i+1]&0xC0 != 0x80 {
				return false
			}
			r = (r&0x1F)<<6 | rune(data[i+1]&0x3F)
			size = 2
		} else if r&0xF0 == 0xE0 {
			if i+2 >= len(data) || data[i+1]&0xC0 != 0x80 || data[i+2]&0xC0 != 0x80 {
				return false
			}
			r = (r&0x0F)<<12 | (rune(data[i+1]&0x3F)<<6) | rune(data[i+2]&0x3F)
			size = 3
		} else if r&0xF8 == 0xF0 {
			if i+3 >= len(data) || data[i+1]&0xC0 != 0x80 || data[i+2]&0xC0 != 0x80 || data[i+3]&0xC0 != 0x80 {
				return false
			}
			r = (r&0x07)<<18 | (rune(data[i+1]&0x3F)<<12) | (rune(data[i+2]&0x3F)<<6) | rune(data[i+3]&0x3F)
			size = 4
		} else {
			return false
		}
		if r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF) {
			return false
		}
		i += size
	}
	return true
}

func agentSkillManagerBinaryConfig(s *Server) (readOnly, allowBinary bool, maxSizeMB int) {
	maxSizeMB = 1
	if s == nil || s.Cfg == nil {
		return readOnly, allowBinary, maxSizeMB
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	readOnly = s.Cfg.Tools.SkillManager.ReadOnly
	allowBinary = s.Cfg.Tools.SkillManager.AllowBinaryAssets
	maxSizeMB = s.Cfg.Tools.SkillManager.MaxUploadSizeMB
	return readOnly, allowBinary, maxSizeMB
}

func handleRunAgentSkillScript(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractSkillPathID(r.URL.Path, "/api/agent-skills/")
		var req struct {
			Script string                 `json:"script"`
			Args   map[string]interface{} `json:"args"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Args == nil {
			req.Args = map[string]interface{}{}
		}
		output, err := s.AgentSkillManager.RunAgentSkillScript(r.Context(), id, req.Script, req.Args)
		status := "ok"
		message := ""
		if err != nil {
			status = "error"
			message = err.Error()
		}
		writeAgentSkillJSON(w, http.StatusOK, map[string]interface{}{
			"status":  status,
			"output":  output,
			"message": message,
		})
	}
}

func autoEnableCleanAgentSkill(s *Server, mgr *tools.AgentSkillManager, entry *tools.AgentSkillRegistryEntry) *tools.AgentSkillRegistryEntry {
	if s == nil || s.Cfg == nil || mgr == nil || entry == nil {
		return entry
	}
	if s.Cfg.Tools.SkillManager.AutoEnableClean && entry.SecurityStatus == tools.SecurityClean {
		if err := mgr.EnableAgentSkill(entry.ID, true, "system:auto_enable"); err == nil {
			if updated, getErr := mgr.GetAgentSkill(entry.ID); getErr == nil {
				return updated
			}
		}
	}
	return entry
}

func agentSkillManagerConfig(s *Server) (readOnly, allowUploads bool, maxSizeMB int, useGuardian bool) {
	allowUploads = true
	maxSizeMB = 1
	if s == nil || s.Cfg == nil {
		return readOnly, allowUploads, maxSizeMB, useGuardian
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	readOnly = s.Cfg.Tools.SkillManager.ReadOnly
	allowUploads = s.Cfg.Tools.SkillManager.AllowUploads
	maxSizeMB = s.Cfg.Tools.SkillManager.MaxUploadSizeMB
	useGuardian = s.Cfg.Tools.SkillManager.ScanWithGuardian
	return readOnly, allowUploads, maxSizeMB, useGuardian
}

func writeAgentSkillJSON(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
