package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/agent"
	taskrules "aurago/internal/rules"
	promptsembed "aurago/prompts"

	"gopkg.in/yaml.v3"
)

type ruleSaveRequest struct {
	Title     string   `json:"title"`
	Enabled   bool     `json:"enabled"`
	Priority  int      `json:"priority"`
	Tools     []string `json:"tools"`
	Workflows []string `json:"workflows"`
	Keywords  []string `json:"keywords"`
	Body      string   `json:"body"`
	Design    string   `json:"design"`
}

type ruleCandidate struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

func handleConfigRules(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			catalog, err := loadServerRulesCatalog(s)
			if err != nil {
				jsonError(w, "Failed to load rules", http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{
				"enabled":    s.Cfg.Rules.Enabled,
				"rules":      catalog.Rules,
				"candidates": buildRuleCandidates(s),
			})
		case http.MethodPost:
			var req struct {
				ID string `json:"id"`
				ruleSaveRequest
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Bad request", http.StatusBadRequest)
				return
			}
			if err := saveRuleOverride(s, req.ID, req.ruleSaveRequest); err != nil {
				jsonError(w, err.Error(), statusForRuleError(err))
				return
			}
			writeJSON(w, map[string]string{"status": "ok", "id": req.ID})
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleConfigRuleByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := ruleIDFromPath(r.URL.Path, "/api/config/rules/")
		if strings.HasSuffix(id, "/restore") {
			id = strings.TrimSuffix(id, "/restore")
		}
		if err := taskrules.ValidateRuleID(id); err != nil {
			jsonError(w, "Invalid rule id", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			catalog, err := loadServerRulesCatalog(s)
			if err != nil {
				jsonError(w, "Failed to load rules", http.StatusInternalServerError)
				return
			}
			rule, ok := catalog.Rule(id)
			if !ok {
				jsonError(w, "Rule not found", http.StatusNotFound)
				return
			}
			design, _ := catalog.Design(id)
			writeJSON(w, map[string]interface{}{"rule": rule, "design": design.Content})
		case http.MethodPut:
			var req ruleSaveRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Bad request", http.StatusBadRequest)
				return
			}
			if err := saveRuleOverride(s, id, req); err != nil {
				jsonError(w, err.Error(), statusForRuleError(err))
				return
			}
			writeJSON(w, map[string]string{"status": "ok", "id": id})
		case http.MethodDelete:
			if err := deleteRuleOverride(s, id); err != nil {
				jsonError(w, err.Error(), statusForRuleError(err))
				return
			}
			writeJSON(w, map[string]string{"status": "ok"})
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleConfigRuleRestore(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSuffix(ruleIDFromPath(r.URL.Path, "/api/config/rules/"), "/restore")
		if err := taskrules.ValidateRuleID(id); err != nil {
			jsonError(w, "Invalid rule id", http.StatusBadRequest)
			return
		}
		if err := deleteRuleOverride(s, id); err != nil && !os.IsNotExist(err) {
			jsonError(w, err.Error(), statusForRuleError(err))
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "id": id})
	}
}

func buildRuleCandidates(s *Server) map[string][]ruleCandidate {
	var cfg = s.Cfg
	toolSummaries := agent.ToolSummariesFromConfig(cfg)
	tools := make([]ruleCandidate, 0, len(toolSummaries))
	for _, summary := range toolSummaries {
		id, desc, ok := strings.Cut(summary, ": ")
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		candidate := ruleCandidate{ID: id, Label: id}
		if ok {
			candidate.Description = strings.TrimSpace(desc)
		}
		tools = append(tools, candidate)
	}

	return map[string][]ruleCandidate{
		"tools":     tools,
		"workflows": defaultRuleWorkflowCandidates(),
	}
}

func defaultRuleWorkflowCandidates() []ruleCandidate {
	return []ruleCandidate{
		{ID: "homepage", Label: "Homepage"},
		{ID: "website", Label: "Website"},
		{ID: "landing_page", Label: "Landing page"},
		{ID: "web_design", Label: "Web design"},
		{ID: "build", Label: "Build"},
		{ID: "preview", Label: "Preview"},
		{ID: "deploy", Label: "Deploy"},
		{ID: "cronjobs", Label: "Cronjobs"},
		{ID: "missions", Label: "Missions"},
		{ID: "heartbeat", Label: "Heartbeat"},
		{ID: "remote_execution", Label: "Remote execution"},
		{ID: "containers", Label: "Containers"},
		{ID: "docker", Label: "Docker"},
		{ID: "proxmox", Label: "Proxmox"},
		{ID: "truenas", Label: "TrueNAS"},
		{ID: "tailscale", Label: "Tailscale"},
		{ID: "cloudflare_tunnel", Label: "Cloudflare Tunnel"},
		{ID: "email", Label: "Email"},
		{ID: "agentmail", Label: "AgentMail"},
		{ID: "webhooks", Label: "Webhooks"},
		{ID: "research", Label: "Research"},
		{ID: "browser", Label: "Browser automation"},
		{ID: "scraper", Label: "Scraper"},
		{ID: "secrets", Label: "Secrets and vault"},
		{ID: "security", Label: "Security"},
		{ID: "audit", Label: "Audit"},
		{ID: "media", Label: "Media"},
		{ID: "image_generation", Label: "Image generation"},
		{ID: "video_generation", Label: "Video generation"},
		{ID: "tts", Label: "Text to speech"},
		{ID: "smart_home", Label: "Smart home"},
		{ID: "home_assistant", Label: "Home Assistant"},
		{ID: "mcp", Label: "MCP"},
		{ID: "skills", Label: "Skills"},
		{ID: "co_agents", Label: "Co-agents"},
		{ID: "sql", Label: "SQL"},
		{ID: "s3", Label: "S3"},
		{ID: "webdav", Label: "WebDAV"},
		{ID: "google_workspace", Label: "Google Workspace"},
		{ID: "onedrive", Label: "OneDrive"},
		{ID: "paperless", Label: "Paperless-ngx"},
		{ID: "notes", Label: "Notes"},
		{ID: "memory", Label: "Memory"},
		{ID: "document_creation", Label: "Document creation"},
		{ID: "virtual_desktop", Label: "Virtual desktop"},
	}
}

func loadServerRulesCatalog(s *Server) (*taskrules.Catalog, error) {
	return taskrules.LoadCatalog(taskrules.LoadOptions{
		PromptsDir: s.Cfg.Directories.PromptsDir,
		EmbeddedFS: promptsembed.FS,
	})
}

func saveRuleOverride(s *Server, id string, req ruleSaveRequest) error {
	if err := taskrules.ValidateRuleID(id); err != nil {
		return err
	}
	ruleDir := filepath.Join(s.Cfg.Directories.PromptsDir, "rules", id)
	if err := os.MkdirAll(ruleDir, 0755); err != nil {
		return fmt.Errorf("create rule dir: %w", err)
	}
	rule := taskrules.Rule{
		ID:        id,
		Title:     strings.TrimSpace(req.Title),
		Enabled:   req.Enabled,
		Priority:  req.Priority,
		Tools:     req.Tools,
		Workflows: req.Workflows,
		Keywords:  req.Keywords,
	}
	if rule.Title == "" {
		rule.Title = id
	}
	meta, err := yaml.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal rule metadata: %w", err)
	}
	body := strings.TrimSpace(req.Body)
	if len(body) > taskrules.MaxRuleBytes {
		return fmt.Errorf("rule body exceeds %d bytes", taskrules.MaxRuleBytes)
	}
	content := "---\n" + strings.TrimSpace(string(meta)) + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("write rule: %w", err)
	}
	design := strings.TrimSpace(req.Design)
	if design != "" {
		if len(design) > taskrules.MaxDesignBytes {
			return fmt.Errorf("design exceeds %d bytes", taskrules.MaxDesignBytes)
		}
		if err := os.WriteFile(filepath.Join(ruleDir, "DESIGN.md"), []byte(design+"\n"), 0644); err != nil {
			return fmt.Errorf("write design: %w", err)
		}
	} else if err := os.Remove(filepath.Join(ruleDir, "DESIGN.md")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove design: %w", err)
	}
	return nil
}

func deleteRuleOverride(s *Server, id string) error {
	if err := taskrules.ValidateRuleID(id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.Cfg.Directories.PromptsDir, "rules", id))
}

func ruleIDFromPath(path, prefix string) string {
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}

func statusForRuleError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if strings.Contains(err.Error(), "invalid rule id") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
