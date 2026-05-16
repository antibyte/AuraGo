package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
				"enabled": s.Cfg.Rules.Enabled,
				"rules":   catalog.Rules,
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
