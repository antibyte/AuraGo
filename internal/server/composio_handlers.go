package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	"gopkg.in/yaml.v3"
)

type composioSelectionPayload struct {
	Enabled                   *bool                           `json:"enabled"`
	BaseURL                   string                          `json:"base_url"`
	UserID                    string                          `json:"user_id"`
	ReadOnly                  *bool                           `json:"read_only"`
	AllowDestructive          *bool                           `json:"allow_destructive"`
	AllowNaturalLanguageInput *bool                           `json:"allow_natural_language_input"`
	RequestTimeoutSeconds     *int                            `json:"request_timeout_seconds"`
	CacheTTLSeconds           *int                            `json:"cache_ttl_seconds"`
	MaxResultBytes            *int                            `json:"max_result_bytes"`
	Toolkits                  *[]config.ComposioToolkitConfig `json:"toolkits"`
}

func handleComposioStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := composioConfigSnapshot(s)
		writeComposioJSON(w, map[string]interface{}{
			"status":            composioStatusString(cfg),
			"enabled":           cfg.Enabled,
			"configured":        strings.TrimSpace(cfg.APIKey) != "",
			"base_url":          cfg.BaseURL,
			"user_id":           cfg.UserID,
			"read_only":         cfg.ReadOnly,
			"allow_destructive": cfg.AllowDestructive,
			"selected_count":    len(enabledComposioToolkits(cfg.Toolkits)),
		})
	}
}

func handleComposioTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, _, err := composioClientFromServer(s)
		if err != nil {
			writeComposioJSON(w, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		page, err := client.ListToolkits(ctx, tools.ComposioListQuery{Limit: 1})
		if err != nil {
			writeComposioJSON(w, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		writeComposioJSON(w, map[string]interface{}{
			"status":       "ok",
			"message":      "Composio API connection works.",
			"sample_count": len(page.Items),
			"next_cursor":  page.NextCursor,
		})
	}
}

func handleComposioToolkits(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, _, err := composioClientFromServer(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		page, err := client.ListToolkits(r.Context(), tools.ComposioListQuery{
			Query:  r.URL.Query().Get("q"),
			Cursor: r.URL.Query().Get("cursor"),
			Limit:  parseComposioLimit(r.URL.Query().Get("limit")),
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeComposioJSON(w, map[string]interface{}{
			"items":       page.Items,
			"next_cursor": page.NextCursor,
			"total":       page.Total,
		})
	}
}

func handleComposioTools(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, cfg, err := composioClientFromServer(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		page, err := client.ListTools(r.Context(), tools.ComposioToolQuery{
			ComposioListQuery: tools.ComposioListQuery{
				Query:  r.URL.Query().Get("q"),
				Cursor: r.URL.Query().Get("cursor"),
				Limit:  parseComposioLimit(r.URL.Query().Get("limit")),
			},
			ToolkitSlug: r.URL.Query().Get("toolkit_slug"),
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		policy := tools.ComposioPolicyFromConfig(cfg)
		if isComposioPreviewRequest(r) {
			policy = composioPreviewPolicy(policy, r.URL.Query().Get("toolkit_slug"))
		}
		items := make([]map[string]interface{}, 0, len(page.Items))
		for _, item := range page.Items {
			decision := tools.EvaluateComposioToolPolicy(policy, item)
			items = append(items, map[string]interface{}{
				"tool":            item,
				"policy_decision": decision,
			})
		}
		writeComposioJSON(w, map[string]interface{}{
			"items":       items,
			"next_cursor": page.NextCursor,
			"total":       page.Total,
		})
	}
}

func isComposioPreviewRequest(r *http.Request) bool {
	raw := strings.TrimSpace(r.URL.Query().Get("preview"))
	return raw == "1" || strings.EqualFold(raw, "true")
}

func composioPreviewPolicy(policy tools.ComposioPolicyConfig, toolkitSlug string) tools.ComposioPolicyConfig {
	toolkitSlug = strings.TrimSpace(toolkitSlug)
	if toolkitSlug == "" {
		return policy
	}
	policy.Enabled = true
	for i := range policy.Toolkits {
		if strings.EqualFold(policy.Toolkits[i].Slug, toolkitSlug) {
			policy.Toolkits[i].Enabled = true
			return policy
		}
	}
	policy.Toolkits = append(policy.Toolkits, tools.ComposioToolkitPolicy{
		Slug:    toolkitSlug,
		Enabled: true,
	})
	return policy
}

func handleComposioAuthConfigs(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, _, err := composioClientFromServer(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		page, err := client.ListAuthConfigs(r.Context(), r.URL.Query().Get("toolkit_slug"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeComposioJSON(w, map[string]interface{}{"items": page.Items, "next_cursor": page.NextCursor, "total": page.Total})
	}
}

func handleComposioConnectedAccounts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, cfg, err := composioClientFromServer(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		page, err := client.ListConnectedAccounts(r.Context(), r.URL.Query().Get("toolkit_slug"), cfg.UserID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeComposioJSON(w, map[string]interface{}{"items": page.Items, "next_cursor": page.NextCursor, "total": page.Total})
	}
}

func handleComposioConnectLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, cfg, err := composioClientFromServer(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		var req tools.ComposioConnectLinkRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.UserID) == "" {
			req.UserID = cfg.UserID
		}
		if strings.TrimSpace(req.CallbackURL) == "" {
			req.CallbackURL = composioDefaultCallbackURL(r)
		}
		if strings.TrimSpace(req.AuthConfigID) == "" {
			if strings.TrimSpace(req.ToolkitSlug) == "" {
				jsonError(w, "Composio toolkit slug or auth config ID is required", http.StatusBadRequest)
				return
			}
			authConfig, err := ensureComposioAuthConfig(r.Context(), client, req.ToolkitSlug)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			req.AuthConfigID = authConfig.ID
		}
		if strings.TrimSpace(req.AuthConfigID) == "" {
			jsonError(w, "Composio auth config ID is required", http.StatusBadGateway)
			return
		}
		link, err := client.CreateConnectLink(r.Context(), req)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeComposioJSON(w, map[string]interface{}{"status": "ok", "link": link})
	}
}

func ensureComposioAuthConfig(ctx context.Context, client *tools.ComposioClient, toolkitSlug string) (tools.ComposioAuthConfig, error) {
	toolkitSlug = strings.TrimSpace(toolkitSlug)
	if toolkitSlug == "" {
		return tools.ComposioAuthConfig{}, fmt.Errorf("composio toolkit slug is required")
	}
	page, err := client.ListAuthConfigs(ctx, toolkitSlug)
	if err != nil {
		return tools.ComposioAuthConfig{}, err
	}
	if authConfig, ok := preferredComposioAuthConfig(page.Items); ok {
		return authConfig, nil
	}
	authConfig, err := client.CreateAuthConfig(ctx, toolkitSlug)
	if err != nil {
		return tools.ComposioAuthConfig{}, err
	}
	if strings.TrimSpace(authConfig.ID) == "" {
		return tools.ComposioAuthConfig{}, fmt.Errorf("composio auth config creation returned no ID")
	}
	return authConfig, nil
}

func preferredComposioAuthConfig(items []tools.ComposioAuthConfig) (tools.ComposioAuthConfig, bool) {
	for _, item := range items {
		if item.Enabled && item.IsComposioManaged && strings.TrimSpace(item.ID) != "" {
			return item, true
		}
	}
	for _, item := range items {
		if item.Enabled && strings.TrimSpace(item.ID) != "" {
			return item, true
		}
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) != "" {
			return item, true
		}
	}
	return tools.ComposioAuthConfig{}, false
}

func handleComposioSelection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg := composioConfigSnapshot(s)
			writeComposioJSON(w, composioSelectionResponse(cfg))
		case http.MethodPut:
			handlePutComposioSelection(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handlePutComposioSelection(s *Server, w http.ResponseWriter, r *http.Request) {
	var payload composioSelectionPayload
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	err := persistComposioSectionUpdate(s, func(section map[string]interface{}) error {
		if payload.Enabled != nil {
			section["enabled"] = *payload.Enabled
		}
		if strings.TrimSpace(payload.BaseURL) != "" {
			section["base_url"] = strings.TrimRight(strings.TrimSpace(payload.BaseURL), "/")
		}
		if strings.TrimSpace(payload.UserID) != "" {
			section["user_id"] = strings.TrimSpace(payload.UserID)
		}
		if payload.ReadOnly != nil {
			section["read_only"] = *payload.ReadOnly
		}
		if payload.AllowDestructive != nil {
			section["allow_destructive"] = *payload.AllowDestructive
		}
		if payload.AllowNaturalLanguageInput != nil {
			section["allow_natural_language_input"] = *payload.AllowNaturalLanguageInput
		}
		if payload.RequestTimeoutSeconds != nil {
			section["request_timeout_seconds"] = *payload.RequestTimeoutSeconds
		}
		if payload.CacheTTLSeconds != nil {
			section["cache_ttl_seconds"] = *payload.CacheTTLSeconds
		}
		if payload.MaxResultBytes != nil {
			section["max_result_bytes"] = *payload.MaxResultBytes
		}
		if payload.Toolkits != nil {
			section["toolkits"] = composioToolkitsForYAML(*payload.Toolkits)
		}
		delete(section, "api_key")
		return nil
	})
	if err != nil {
		jsonError(w, "Failed to save Composio selection: "+err.Error(), http.StatusInternalServerError)
		return
	}
	cfg := composioConfigSnapshot(s)
	writeComposioJSON(w, composioSelectionResponse(cfg))
}

func composioClientFromServer(s *Server) (*tools.ComposioClient, config.ComposioConfig, error) {
	cfg := composioConfigSnapshot(s)
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, cfg, fmt.Errorf("Composio API key not found in vault")
	}
	return tools.NewComposioClientFromConfig(cfg), cfg, nil
}

func composioConfigSnapshot(s *Server) config.ComposioConfig {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if s.Cfg == nil {
		return config.ComposioConfig{}
	}
	cfg := s.Cfg.Composio
	cfg.Toolkits = append([]config.ComposioToolkitConfig(nil), cfg.Toolkits...)
	return cfg
}

func persistComposioSectionUpdate(s *Server, mutate func(map[string]interface{}) error) error {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()

	configPath := ""
	s.CfgMu.RLock()
	if s.Cfg != nil {
		configPath = s.Cfg.ConfigPath
	}
	s.CfgMu.RUnlock()
	if configPath == "" {
		return os.ErrInvalid
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return err
	}
	section, ok := rawCfg["composio"].(map[string]interface{})
	if !ok {
		section = map[string]interface{}{}
	}
	if err := mutate(section); err != nil {
		return err
	}
	rawCfg["composio"] = section
	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return err
	}
	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		return err
	}

	newCfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	newCfg.ConfigPath = configPath
	newCfg.ApplyVaultSecrets(s.Vault)
	newCfg.ApplyOAuthTokens(s.Vault)
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(newCfg)
	s.CfgMu.Unlock()
	return nil
}

func composioSelectionResponse(cfg config.ComposioConfig) map[string]interface{} {
	return map[string]interface{}{
		"enabled":                      cfg.Enabled,
		"configured":                   strings.TrimSpace(cfg.APIKey) != "",
		"base_url":                     cfg.BaseURL,
		"user_id":                      cfg.UserID,
		"read_only":                    cfg.ReadOnly,
		"allow_destructive":            cfg.AllowDestructive,
		"allow_natural_language_input": cfg.AllowNaturalLanguageInput,
		"request_timeout_seconds":      cfg.RequestTimeoutSeconds,
		"cache_ttl_seconds":            cfg.CacheTTLSeconds,
		"max_result_bytes":             cfg.MaxResultBytes,
		"toolkits":                     cfg.Toolkits,
	}
}

func composioToolkitsForYAML(toolkits []config.ComposioToolkitConfig) []interface{} {
	out := make([]interface{}, 0, len(toolkits))
	for _, tk := range toolkits {
		slug := strings.TrimSpace(tk.Slug)
		if slug == "" {
			continue
		}
		item := map[string]interface{}{
			"slug":    slug,
			"enabled": tk.Enabled,
		}
		if strings.TrimSpace(tk.PreferredConnectedAccountID) != "" {
			item["preferred_connected_account_id"] = strings.TrimSpace(tk.PreferredConnectedAccountID)
		}
		if tk.ReadOnly != nil {
			item["read_only"] = *tk.ReadOnly
		}
		if tk.AllowDestructive != nil {
			item["allow_destructive"] = *tk.AllowDestructive
		}
		if tk.AllowNaturalLanguageInput != nil {
			item["allow_natural_language_input"] = *tk.AllowNaturalLanguageInput
		}
		if len(tk.AllowedToolSlugs) > 0 {
			item["allowed_tool_slugs"] = tk.AllowedToolSlugs
		}
		if len(tk.BlockedToolSlugs) > 0 {
			item["blocked_tool_slugs"] = tk.BlockedToolSlugs
		}
		out = append(out, item)
	}
	return out
}

func composioStatusString(cfg config.ComposioConfig) string {
	switch {
	case !cfg.Enabled:
		return "disabled"
	case strings.TrimSpace(cfg.APIKey) == "":
		return "missing_api_key"
	default:
		return "ready"
	}
}

func enabledComposioToolkits(toolkits []config.ComposioToolkitConfig) []config.ComposioToolkitConfig {
	out := make([]config.ComposioToolkitConfig, 0, len(toolkits))
	for _, tk := range toolkits {
		if tk.Enabled {
			out = append(out, tk)
		}
	}
	return out
}

func parseComposioLimit(raw string) int {
	var limit int
	_, _ = fmt.Sscanf(strings.TrimSpace(raw), "%d", &limit)
	if limit <= 0 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func composioDefaultCallbackURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host + "/config.html?composio=connected"
}

func writeComposioJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
