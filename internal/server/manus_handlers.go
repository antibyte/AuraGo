package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/manus"
	"aurago/internal/security"
)

func handleManusStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := manusConfigSnapshot(s)
		status := "disabled"
		if cfg.Enabled {
			status = "missing_key"
			if strings.TrimSpace(cfg.APIKey) != "" {
				status = "ready"
			}
		}
		writeManusJSON(w, http.StatusOK, map[string]interface{}{
			"status": status, "enabled": cfg.Enabled, "configured": strings.TrimSpace(cfg.APIKey) != "",
			"read_only": cfg.ReadOnly, "allow_create_tasks": cfg.AllowCreateTasks,
			"allow_send_messages": cfg.AllowSendMessages, "allow_stop_tasks": cfg.AllowStopTasks,
			"allow_file_uploads": cfg.AllowFileUploads, "allow_file_downloads": cfg.AllowFileDownloads,
			"allowed_project_count": len(cfg.AllowedProjectIDs), "allowed_connector_count": len(cfg.AllowedConnectorIDs),
			"allowed_skill_count": len(cfg.AllowedSkillIDs), "default_agent_profile": cfg.DefaultAgentProfile,
		})
	}
}

func handleManusTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, err := manusClientFromServer(s)
		if err != nil {
			writeManusJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		handleManusTestWithClient(client)(w, r)
	}
}

func handleManusTestWithClient(client *manus.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		credits, err := client.AvailableCredits(ctx)
		if err != nil {
			writeManusJSON(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		writeManusJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "message": "Manus API connection works.", "credits": credits.Data})
	}
}

func handleManusProjects(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client, cfg, ok := manusCatalogClient(w, r, s)
		if !ok {
			return
		}
		handleManusProjectsWithClient(client, cfg.AllowedProjectIDs)(w, r)
	}
}

func handleManusProjectsWithClient(client *manus.Client, allowed []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items, err := client.ListProjects(r.Context())
		if err != nil {
			writeManusJSON(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			result = append(result, map[string]interface{}{"id": item.ID, "name": item.Name, "instruction": item.Instruction, "allowed": manusAllowedID(allowed, item.ID)})
		}
		writeManusJSON(w, http.StatusOK, map[string]interface{}{"items": result})
	}
}

func handleManusConnectors(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client, cfg, ok := manusCatalogClient(w, r, s)
		if !ok {
			return
		}
		items, err := client.ListConnectors(r.Context())
		if err != nil {
			writeManusJSON(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			result = append(result, map[string]interface{}{"id": item.ID, "name": item.Name, "type": item.Type, "description": item.Description, "category": item.Category, "allowed": manusAllowedID(cfg.AllowedConnectorIDs, item.ID)})
		}
		writeManusJSON(w, http.StatusOK, map[string]interface{}{"items": result})
	}
}

func handleManusSkills(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client, cfg, ok := manusCatalogClient(w, r, s)
		if !ok {
			return
		}
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		if projectID != "" && !manusAllowedID(cfg.AllowedProjectIDs, projectID) {
			writeManusJSON(w, http.StatusForbidden, map[string]interface{}{"status": "error", "message": "Project is not allowlisted"})
			return
		}
		items, err := client.ListSkills(r.Context(), projectID)
		if err != nil {
			writeManusJSON(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			result = append(result, map[string]interface{}{"id": item.ID, "name": item.Name, "description": item.Description, "owner_type": item.OwnerType, "allowed": manusAllowedID(cfg.AllowedSkillIDs, item.ID)})
		}
		writeManusJSON(w, http.StatusOK, map[string]interface{}{"items": result})
	}
}

func manusCatalogClient(w http.ResponseWriter, r *http.Request, s *Server) (*manus.Client, config.ManusConfig, bool) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, config.ManusConfig{}, false
	}
	client, err := manusClientFromServer(s)
	if err != nil {
		writeManusJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"status": "error", "message": err.Error()})
		return nil, config.ManusConfig{}, false
	}
	return client, manusConfigSnapshot(s), true
}

func manusClientFromServer(s *Server) (*manus.Client, error) {
	cfg := manusConfigSnapshot(s)
	if !cfg.Enabled {
		return nil, fmt.Errorf("Manus integration is disabled")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("Manus API key is not configured in the vault")
	}
	return manus.NewClient(cfg.APIKey, manus.ClientConfig{
		Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second, MaxResultBytes: int64(cfg.MaxResultBytes),
	})
}

func manusConfigSnapshot(s *Server) config.ManusConfig {
	if s == nil || s.Cfg == nil {
		return config.ManusConfig{}
	}
	cfg := s.Cfg.Manus
	if strings.TrimSpace(cfg.APIKey) == "" && s.Vault != nil {
		if key, err := s.Vault.ReadSecret("manus_api_key"); err == nil {
			cfg.APIKey = key
		}
	}
	return cfg
}

func manusAllowedID(allowed []string, id string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == strings.TrimSpace(id) && strings.TrimSpace(id) != "" {
			return true
		}
	}
	return false
}

func writeManusJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
