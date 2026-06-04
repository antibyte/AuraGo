package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
)

func handleDograhStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		writeDograhJSON(w, dograhStatusForRequest(r.Context(), s, &cfg, r))
	}
}

func handleDograhTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		if !applyDograhPatch(w, r, &cfg) {
			return
		}
		if !cfg.Dograh.Enabled {
			writeDograhJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "Dograh integration is disabled"})
			return
		}
		result := tools.TestDograhConnection(r.Context(), cfg.Dograh.APIURL, cfg.Dograh.APIKey)
		writeDograhJSON(w, result)
	}
}

func handleDograhStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		if !cfg.Dograh.Enabled {
			writeDograhJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "Dograh integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Dograh.Mode), "external") {
			status := dograhStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "Dograh is configured in external mode; no managed stack to start"
			writeDograhJSON(w, status)
			return
		}
		if err := s.ensureDograhSecrets(&cfg); err != nil {
			writeDograhJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Dograh.Mode, "status": "setup_required", "setup_required": true, "message": err.Error()})
			return
		}
		if _, err := tools.ResolveDograhStackConfig(&cfg, cfg.Runtime.IsDocker); err != nil {
			writeDograhJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Dograh.Mode, "status": "setup_required", "setup_required": true, "message": err.Error()})
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if err := tools.EnsureDograhStackRunning(ctx, cfg.Docker.Host, &cfg, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[Dograh] Manual start failed", "error", err)
			}
		}()
		writeDograhJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Dograh.Mode, "status": "starting", "message": "Dograh stack is starting"})
	}
}

func handleDograhStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		if !cfg.Dograh.Enabled {
			writeDograhJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "Dograh integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Dograh.Mode), "external") {
			status := dograhStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "Dograh is configured in external mode; no managed stack to stop"
			writeDograhJSON(w, status)
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := tools.StopDograhStack(ctx, cfg.Docker.Host, &cfg, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[Dograh] Manual stop failed", "error", err)
			}
		}()
		writeDograhJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Dograh.Mode, "status": "stopping", "message": "Dograh stack is stopping"})
	}
}

func handleDograhRecreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		if !cfg.Dograh.Enabled {
			writeDograhJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "Dograh integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Dograh.Mode), "external") {
			status := dograhStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "Dograh is configured in external mode; no managed stack to recreate"
			writeDograhJSON(w, status)
			return
		}
		if err := s.ensureDograhSecrets(&cfg); err != nil {
			writeDograhJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Dograh.Mode, "status": "setup_required", "setup_required": true, "message": err.Error()})
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			if err := tools.StopDograhStack(ctx, cfg.Docker.Host, &cfg, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[Dograh] Recreate stop failed", "error", err)
			}
			if err := tools.EnsureDograhStackRunning(ctx, cfg.Docker.Host, &cfg, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[Dograh] Recreate start failed", "error", err)
			}
		}()
		writeDograhJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Dograh.Mode, "status": "starting", "message": "Dograh stack is recreating"})
	}
}

func handleDograhProvisionWebhook(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		if !cfg.Dograh.Enabled || !cfg.Dograh.CallbackWebhookEnabled {
			writeDograhJSON(w, map[string]interface{}{"status": "disabled", "message": "Dograh webhook bridge is disabled"})
			return
		}
		if s == nil || s.WebhookManager == nil || s.TokenManager == nil {
			jsonError(w, "Webhook manager is not available", http.StatusServiceUnavailable)
			return
		}
		slug := strings.TrimSpace(cfg.Dograh.CallbackWebhookSlug)
		if slug == "" {
			slug = "dograh-callback"
		}
		if existing, err := s.WebhookManager.GetBySlug(slug); err == nil {
			writeDograhJSON(w, map[string]interface{}{
				"status":      "exists",
				"webhook":     webhookMaskSecrets(existing, s.Vault),
				"webhook_url": dograhWebhookURL(r, slug),
				"message":     "Dograh webhook already exists",
			})
			return
		}
		rawToken, tokenMeta, err := s.TokenManager.Create("Dograh webhook", []string{"webhook"}, nil)
		if err != nil {
			jsonError(w, "Failed to create webhook token", http.StatusInternalServerError)
			return
		}
		preset := dograhWebhookPreset()
		created, err := s.WebhookManager.Create(webhooks.Webhook{
			Name:    "Dograh Callback",
			Enabled: true,
			Slug:    slug,
			TokenID: tokenMeta.ID,
			Format:  preset.Format,
			Delivery: webhooks.DeliveryConfig{
				Mode:           webhooks.DeliveryModeMessage,
				PromptTemplate: preset.PromptHint,
				Priority:       "queue",
			},
		})
		if err != nil {
			_ = s.TokenManager.Delete(tokenMeta.ID)
			jsonError(w, "Failed to create Dograh webhook", http.StatusBadRequest)
			return
		}
		writeDograhJSON(w, map[string]interface{}{
			"status":      "created",
			"webhook":     webhookMaskSecrets(created, s.Vault),
			"webhook_url": dograhWebhookURL(r, slug),
			"token":       rawToken,
			"message":     "Dograh webhook created; copy the token now",
		})
	}
}

func handleDograhRegisterAuraGoMCPTool(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentDograhConfig(s)
		if !cfg.Dograh.Enabled || !cfg.Dograh.MCPServerToolEnabled {
			writeDograhJSON(w, map[string]interface{}{"status": "disabled", "message": "Dograh MCP tool bridge is disabled"})
			return
		}
		if cfg.Dograh.ReadOnly {
			jsonError(w, "Dograh integration is read-only; registering a tool would modify Dograh", http.StatusForbidden)
			return
		}
		if strings.TrimSpace(cfg.Dograh.APIKey) == "" {
			jsonError(w, "Dograh API key is required", http.StatusBadRequest)
			return
		}
		if !cfg.MCPServer.Enabled || !cfg.MCPServer.RequireAuth {
			jsonError(w, "AuraGo MCP server must be enabled with require_auth=true before registering it in Dograh", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(cfg.Dograh.AuraGoMCPCredentialUUID) == "" {
			jsonError(w, "Dograh AuraGo MCP credential UUID is required", http.StatusBadRequest)
			return
		}
		result, err := tools.RegisterDograhAuraGoMCPTool(r.Context(), cfg.Dograh, dograhAuraGoMCPURL(r))
		if err != nil && result.Status == "" {
			result.Status = "error"
			result.Message = err.Error()
		}
		writeDograhJSON(w, result)
	}
}

func currentDograhConfig(s *Server) config.Config {
	if s == nil || s.Cfg == nil {
		return config.Config{}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return *s.Cfg
}

func (s *Server) ensureDograhSecrets(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !cfg.Dograh.Enabled || strings.EqualFold(strings.TrimSpace(cfg.Dograh.Mode), "external") {
		return nil
	}
	type secretTarget struct {
		key    string
		size   int
		target *string
	}
	targets := []secretTarget{
		{key: "dograh_oss_jwt_secret", size: 32, target: &cfg.Dograh.OSSJWTSecret},
		{key: "dograh_postgres_password", size: 24, target: &cfg.Dograh.PostgresPassword},
		{key: "dograh_redis_password", size: 24, target: &cfg.Dograh.RedisPassword},
		{key: "dograh_minio_root_password", size: 24, target: &cfg.Dograh.MinioRootPassword},
	}
	for _, item := range targets {
		if strings.TrimSpace(*item.target) != "" {
			continue
		}
		secret, err := randomSpaceAgentSecret(item.size)
		if err != nil {
			return err
		}
		*item.target = secret
		if s != nil && s.Vault != nil {
			if err := s.Vault.WriteSecret(item.key, secret); err != nil {
				return err
			}
		}
	}
	if s != nil && s.Cfg != nil {
		if s.Cfg == cfg {
			s.Cfg.Dograh.OSSJWTSecret = cfg.Dograh.OSSJWTSecret
			s.Cfg.Dograh.PostgresPassword = cfg.Dograh.PostgresPassword
			s.Cfg.Dograh.RedisPassword = cfg.Dograh.RedisPassword
			s.Cfg.Dograh.MinioRootPassword = cfg.Dograh.MinioRootPassword
		} else {
			s.CfgMu.Lock()
			s.Cfg.Dograh.OSSJWTSecret = cfg.Dograh.OSSJWTSecret
			s.Cfg.Dograh.PostgresPassword = cfg.Dograh.PostgresPassword
			s.Cfg.Dograh.RedisPassword = cfg.Dograh.RedisPassword
			s.Cfg.Dograh.MinioRootPassword = cfg.Dograh.MinioRootPassword
			s.CfgMu.Unlock()
		}
	}
	return nil
}

func dograhStatus(ctx context.Context, s *Server, cfg *config.Config) map[string]interface{} {
	status, err := tools.DograhStackStatus(ctx, cfg.Docker.Host, cfg)
	if err != nil {
		return map[string]interface{}{"enabled": cfg.Dograh.Enabled, "mode": cfg.Dograh.Mode, "status": "error", "message": err.Error()}
	}
	out := map[string]interface{}{
		"enabled":              status.Enabled,
		"mode":                 status.Mode,
		"status":               status.Status,
		"running":              status.Running,
		"api_url":              status.APIURL,
		"ui_url":               status.UIURL,
		"mcp_url":              status.MCPURL,
		"containers":           status.Containers,
		"setup_required":       status.SetupRequired,
		"admin_setup_required": status.AdminSetupRequired,
	}
	if strings.TrimSpace(status.Message) != "" {
		out["message"] = status.Message
	}
	return out
}

func dograhStatusForRequest(ctx context.Context, s *Server, cfg *config.Config, r *http.Request) map[string]interface{} {
	out := dograhStatus(ctx, s, cfg)
	dograhRewriteBrowserURLs(r, out)
	return out
}

func dograhRewriteBrowserURLs(r *http.Request, payload map[string]interface{}) {
	for _, key := range []string{"api_url", "ui_url"} {
		rawURL, ok := payload[key].(string)
		if !ok || strings.TrimSpace(rawURL) == "" {
			continue
		}
		payload[key] = dograhURLWithRequestHost(rawURL, r)
	}
}

func dograhURLWithRequestHost(rawURL string, r *http.Request) string {
	if requestLooksTailscale(r) && dograhURLIsLoopback(rawURL) {
		return ""
	}
	return manifestURLWithRequestHost(rawURL, r)
}

func dograhURLIsLoopback(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" || host == "::"
}

func applyDograhPatch(w http.ResponseWriter, r *http.Request, cfg *config.Config) bool {
	if r.Body == nil {
		return true
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		if err != nil {
			jsonError(w, "Invalid request payload", http.StatusBadRequest)
			return false
		}
		return true
	}
	var req struct {
		Dograh config.DograhConfig `json:"dograh"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		jsonError(w, "Invalid request payload", http.StatusBadRequest)
		return false
	}
	patch := req.Dograh
	if patch.Enabled {
		cfg.Dograh.Enabled = true
	}
	if strings.TrimSpace(patch.Mode) != "" {
		cfg.Dograh.Mode = patch.Mode
	}
	if strings.TrimSpace(patch.APIURL) != "" {
		cfg.Dograh.APIURL = patch.APIURL
	}
	if strings.TrimSpace(patch.UIURL) != "" {
		cfg.Dograh.UIURL = patch.UIURL
	}
	if strings.TrimSpace(patch.APIKey) != "" {
		cfg.Dograh.APIKey = patch.APIKey
	}
	if strings.TrimSpace(patch.AuraGoMCPCredentialUUID) != "" {
		cfg.Dograh.AuraGoMCPCredentialUUID = patch.AuraGoMCPCredentialUUID
	}
	return true
}

func dograhAuraGoMCPURL(r *http.Request) string {
	return strings.TrimRight(requestBaseURL(r), "/") + "/mcp"
}

func dograhWebhookURL(r *http.Request, slug string) string {
	return strings.TrimRight(requestBaseURL(r), "/") + "/webhook/" + strings.TrimLeft(slug, "/")
}

func requestBaseURL(r *http.Request) string {
	if r == nil {
		return "http://127.0.0.1"
	}
	scheme := "http"
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = strings.Split(proto, ",")[0]
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
}

func dograhWebhookPreset() webhooks.Preset {
	for _, preset := range webhooks.Presets() {
		if preset.Key == "dograh" {
			return preset
		}
	}
	return webhooks.Preset{
		Key:   "dograh",
		Label: "Dograh",
		Format: webhooks.WebhookFormat{
			AcceptedContentTypes: []string{"application/json"},
			Description:          "Dograh workflow callback events.",
		},
		PromptHint: "[Dograh Event: {{webhook_name}}]\nWorkflow: {{field.workflow}}\nEvent: {{field.event}}\nPayload:\n{{payload}}",
	}
}

func writeDograhJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
