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
	"gopkg.in/yaml.v3"
)

// handleSkillTemplates returns available skill templates.
// GET /api/skills/templates
func handleSkillTemplates(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		skillsDir := s.Cfg.Directories.SkillsDir
		s.CfgMu.RUnlock()

		if readOnly || !allowUploads {
			jsonError(w, "Skill creation is disabled", http.StatusForbidden)
			return
		}

		var req struct {
			TemplateName  string   `json:"template_name"`
			SkillName     string   `json:"skill_name"`
			Description   string   `json:"description"`
			Category      string   `json:"category"`
			Tags          []string `json:"tags"`
			BaseURL       string   `json:"base_url"`
			Dependencies  []string `json:"dependencies"`
			VaultKeys     []string `json:"vault_keys"`
			Documentation string   `json:"documentation"`
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
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create skill from template", "Failed to create skill from template", err, "template", req.TemplateName, "skill_name", req.SkillName)
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
			if strings.TrimSpace(req.Documentation) != "" {
				if docErr := s.SkillManager.SetSkillDocumentation(skillID, req.Documentation, "user"); docErr != nil {
					s.Logger.Warn("Failed to save skill documentation from template", "id", skillID, "error", docErr)
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

// handleExportSkill exports a skill as an AuraGo skill bundle.
// GET /api/skills/{id}/export
func handleExportSkill(s *Server) http.HandlerFunc {
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
		bundle, err := s.SkillManager.ExportSkillBundle(id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to export skill", "Failed to export skill bundle", err, "skill_id", id)
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
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to import skill bundle", "Failed to import skill bundle", err)
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

// handleGenerateSkillDraft asks the configured LLM for a draft AuraGo skill.
// POST /api/skills/generate
func handleGenerateSkillDraft(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		systemPrompt := "You generate AuraGo Python skills. Return exactly one JSON object and nothing else: no markdown, no fences, no explanation, no wrapper object, no schema echo. " +
			"Required schema with double-quoted JSON keys only: " +
			`{"name":"skill_name","description":"what it does","category":"category","tags":["tag1","tag2"],"dependencies":["dep1"],"code":"python code as a single JSON string","documentation":"markdown manual"}. ` +
			"Do not output placeholders such as \"...\", `[ ... ]`, or `{ ... }`. Do not output keys like status, enabled, draft, notes, reasoning, examples, or comments. " +
			"Rules for the Python code: read one JSON object from stdin, write exactly one JSON object to stdout, catch errors and return them in JSON, keep code compact and production-ready. " +
			"The skill runs inside AuraGo's execute_skill Python sandbox. Never use subprocess, os.system, os.popen, shell commands, ping, curl, wget, dig, nslookup, host, nmap, sudo, or any external CLI/tool/binary. " +
			"Use Python stdlib or direct libraries/APIs only. For DNS checks use socket or a Python DNS library, never ping. For HTTP use requests/httpx, never curl. " +
			"Bad example: subprocess.run(['ping', ...]). Good example: socket.gethostbyname(host). " +
			"The 'documentation' field MUST contain a Markdown manual (max 8 KB) for the agent that reuses this skill later. " +
			"It MUST include the sections: '## Description', '## Parameters', '## Output', '## Example', '## Errors'. " +
			"Use concrete JSON examples in fenced code blocks. Never embed secrets, API keys or credentials in the manual."
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
			jsonError(w, "LLM generation failed", http.StatusBadGateway)
			return
		}
		if len(resp.Choices) == 0 {
			s.Logger.Warn("[Skills] AI draft generation returned no choices")
			jsonError(w, "LLM generation returned no response", http.StatusBadGateway)
			return
		}
		draft, err := decodeSkillDraft(resp.Choices[0].Message.Content)
		if err != nil {
			s.Logger.Warn("[Skills] Failed to decode generated skill draft", "error", err, "response_preview", truncateForLog(resp.Choices[0].Message.Content, 500))
			jsonError(w, "Failed to parse generated skill draft", http.StatusBadGateway)
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
		if strings.TrimSpace(draft.Name) == "" {
			draft.Name = fallbackGeneratedSkillName(req.Prompt)
		}
		if req.Category != "" {
			draft.Category = req.Category
		}
		if len(req.Dependencies) > 0 {
			draft.Dependencies = req.Dependencies
		}
		if placeholderIssues := generatedSkillPlaceholderIssues(draft); len(placeholderIssues) > 0 {
			jsonError(w, "Generated draft still contains placeholder fields: "+strings.Join(placeholderIssues, "; "), http.StatusBadGateway)
			return
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
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Tags          []string `json:"tags"`
	Dependencies  []string `json:"dependencies"`
	Code          string   `json:"code"`
	Documentation string   `json:"documentation"`
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
		if strings.TrimSpace(draft.Code) == "" {
			lastErr = fmt.Errorf("draft code is missing")
			continue
		}
		if placeholderIssues := generatedSkillPlaceholderIssues(&draft); len(placeholderIssues) > 0 {
			lastErr = fmt.Errorf("%s", strings.Join(placeholderIssues, "; "))
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
		direct.Documentation = strings.TrimSpace(direct.Documentation)
		if direct.Name != "" || direct.Code != "" || direct.Description != "" || direct.Category != "" || len(direct.Tags) > 0 || len(direct.Dependencies) > 0 {
			return direct, nil
		}
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		if yamlErr := yaml.Unmarshal(raw, &generic); yamlErr != nil {
			return generatedSkillDraft{}, err
		}
	}
	if nested, ok := generic["draft"].(map[string]any); ok {
		generic = nested
	}

	draft := generatedSkillDraft{
		Name:          firstNonEmptyString(generic, "name", "skill_name", "skill"),
		Description:   firstNonEmptyString(generic, "description", "summary"),
		Category:      firstNonEmptyString(generic, "category"),
		Tags:          coerceStringList(generic["tags"]),
		Dependencies:  coerceStringList(generic["dependencies"]),
		Code:          firstNonEmptyString(generic, "code", "python_code", "script"),
		Documentation: firstNonEmptyString(generic, "documentation", "manual", "doc", "readme"),
	}
	draft.Name = strings.TrimSpace(draft.Name)
	draft.Description = strings.TrimSpace(draft.Description)
	draft.Category = strings.TrimSpace(draft.Category)
	draft.Tags = normalizeStringList(draft.Tags)
	draft.Dependencies = normalizeStringList(draft.Dependencies)
	draft.Code = strings.TrimSpace(draft.Code)
	draft.Documentation = strings.TrimSpace(draft.Documentation)
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
		"Return exactly one JSON object and nothing else: no markdown, no fences, no explanation, no wrapper object. " +
		"Use this exact schema with double-quoted JSON keys only: " +
		`{"name":"skill_name","description":"what it does","category":"category","tags":["tag1","tag2"],"dependencies":["dep1"],"code":"python code as a single JSON string"}. ` +
		"Keep the intended functionality, but remove sandbox-incompatible behavior. " +
		"Never use subprocess, os.system, os.popen, shell commands, ping, curl, wget, dig, nslookup, host, nmap, sudo, or any external CLI/tool/binary. " +
		"Use Python stdlib or direct libraries/APIs only, read one JSON object from stdin, and write exactly one JSON object to stdout. " +
		"Do not emit placeholders like \"...\" and do not emit extra keys such as status, enabled, or draft."
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
	if placeholderIssues := generatedSkillPlaceholderIssues(draft); len(placeholderIssues) > 0 {
		return true, strings.Join(placeholderIssues, "; ")
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
	if strings.Contains(lower, "'ping'") || strings.Contains(lower, `"ping"`) {
		return true, "generated draft depends on the external ping command instead of direct Python networking"
	}
	return false, ""
}

func generatedSkillLooksLikePlaceholder(draft *generatedSkillDraft) bool {
	return len(generatedSkillPlaceholderIssues(draft)) > 0
}

func generatedSkillPlaceholderIssues(draft *generatedSkillDraft) []string {
	if draft == nil {
		return []string{"generated draft is missing"}
	}
	type fieldCheck struct {
		label string
		value string
	}
	fields := []fieldCheck{
		{label: "name", value: strings.TrimSpace(draft.Name)},
		{label: "description", value: strings.TrimSpace(draft.Description)},
		{label: "category", value: strings.TrimSpace(draft.Category)},
		{label: "code", value: strings.TrimSpace(draft.Code)},
	}
	var issues []string
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		if field.label == "code" {
			if isPlaceholderCodeValue(field.value) {
				issues = append(issues, "generated draft still contains placeholder code text instead of real Python code")
			}
			continue
		}
		if isPlaceholderValue(field.value) {
			issues = append(issues, fmt.Sprintf("generated draft still contains placeholder %s", field.label))
		}
	}
	for _, tag := range draft.Tags {
		if isPlaceholderValue(tag) {
			issues = append(issues, "generated draft still contains placeholder tags")
			break
		}
	}
	for _, dep := range draft.Dependencies {
		if isPlaceholderValue(dep) {
			issues = append(issues, "generated draft still contains placeholder dependencies")
			break
		}
	}
	return normalizeStringList(issues)
}

func isPlaceholderValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	switch value {
	case "...", "…", "<...>", "[...]", "{...}":
		return true
	}
	trimmed := strings.Trim(value, ". <>[]{}\"'")
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	return lower == "name" || lower == "description" || lower == "category" || lower == "code" || lower == "tags" || lower == "dependencies" ||
		lower == "skill_name" || lower == "what it does" || lower == "tag1" || lower == "tag2" || lower == "dep1"
}

func isPlaceholderCodeValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if isPlaceholderValue(value) {
		return true
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "python code as a single json string") {
		return true
	}
	if strings.Contains(lower, "single json string") && strings.Contains(lower, "python code") {
		return true
	}
	if strings.Contains(lower, "write python code here") || strings.Contains(lower, "insert code here") {
		return true
	}
	if strings.ContainsAny(value, "\n:=") {
		return false
	}
	if strings.Contains(value, "import ") || strings.Contains(value, "json.dump") || strings.Contains(value, "def ") || strings.Contains(value, "return ") {
		return false
	}
	return strings.Contains(lower, "code")
}

func safeSkillFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "generated_skill"
	}
	return name + ".py"
}

func fallbackGeneratedSkillName(prompt string) string {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return "generated_skill"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range prompt {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteByte('_')
			lastUnderscore = true
		}
		if b.Len() >= 40 {
			break
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		return "generated_skill"
	}
	return name
}

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " "))
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
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
