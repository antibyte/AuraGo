package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func handleOmniRouteStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentOmniRouteConfig(s)
		writeManifestJSON(w, omniRouteStatusForRequest(r.Context(), s, &cfg, r))
	}
}

func handleOmniRouteTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentOmniRouteConfig(s)
		if !applyOmniRoutePatch(w, r, &cfg) {
			return
		}
		writeManifestJSON(w, omniRouteStatusForRequest(r.Context(), s, &cfg, r))
	}
}

func handleOmniRouteStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentOmniRouteConfig(s)
		if !cfg.OmniRoute.Enabled {
			writeManifestJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "OmniRoute integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.OmniRoute.Mode), "external") {
			status := omniRouteStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "OmniRoute is configured in external mode; no sidecar to start"
			writeManifestJSON(w, status)
			return
		}
		if err := s.ensureOmniRouteSecrets(&cfg); err != nil {
			writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.OmniRoute.Mode, "status": "setup_required", "admin_setup_required": true, "message": err.Error()})
			return
		}
		if _, err := tools.ResolveOmniRouteSidecarConfig(&cfg, cfg.Runtime.IsDocker); err != nil {
			writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.OmniRoute.Mode, "status": "setup_required", "admin_setup_required": true, "message": err.Error()})
			return
		}
		browserBaseURL := omniRouteBrowserBaseURLForRequest(s, &cfg, r)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := tools.EnsureOmniRouteSidecarRunningWithBrowserURL(ctx, cfg.Docker.Host, &cfg, browserBaseURL, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[OmniRoute] Manual start failed", "error", err)
			}
		}()
		writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.OmniRoute.Mode, "status": "starting", "message": "OmniRoute sidecar is starting"})
	}
}

func handleOmniRouteStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentOmniRouteConfig(s)
		if !cfg.OmniRoute.Enabled {
			writeManifestJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "OmniRoute integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.OmniRoute.Mode), "external") {
			status := omniRouteStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "OmniRoute is configured in external mode; no sidecar to stop"
			writeManifestJSON(w, status)
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := tools.StopOmniRouteSidecar(ctx, cfg.Docker.Host, &cfg, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[OmniRoute] Manual stop failed", "error", err)
			}
		}()
		writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.OmniRoute.Mode, "status": "stopping", "message": "OmniRoute sidecar is stopping"})
	}
}

func currentOmniRouteConfig(s *Server) config.Config {
	if s == nil || s.Cfg == nil {
		return config.Config{}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return *s.Cfg
}

func (s *Server) ensureOmniRouteSecrets(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !cfg.OmniRoute.Enabled || strings.EqualFold(strings.TrimSpace(cfg.OmniRoute.Mode), "external") {
		return nil
	}
	if err := s.ensureOmniRouteGeneratedSecret(&cfg.OmniRoute.JWTSecret, "omniroute_jwt_secret", 32); err != nil {
		return err
	}
	if err := s.ensureOmniRouteGeneratedSecret(&cfg.OmniRoute.APIKeySecret, "omniroute_api_key_secret", 32); err != nil {
		return err
	}
	if err := s.ensureOmniRouteGeneratedSecret(&cfg.OmniRoute.WSBridgeSecret, "omniroute_ws_bridge_secret", 32); err != nil {
		return err
	}
	if s != nil && s.Cfg != nil {
		if s.Cfg == cfg {
			s.Cfg.OmniRoute.JWTSecret = cfg.OmniRoute.JWTSecret
			s.Cfg.OmniRoute.APIKeySecret = cfg.OmniRoute.APIKeySecret
			s.Cfg.OmniRoute.WSBridgeSecret = cfg.OmniRoute.WSBridgeSecret
		} else {
			s.CfgMu.Lock()
			s.Cfg.OmniRoute.JWTSecret = cfg.OmniRoute.JWTSecret
			s.Cfg.OmniRoute.APIKeySecret = cfg.OmniRoute.APIKeySecret
			s.Cfg.OmniRoute.WSBridgeSecret = cfg.OmniRoute.WSBridgeSecret
			s.CfgMu.Unlock()
		}
	}
	if strings.TrimSpace(cfg.OmniRoute.InitialPassword) == "" {
		return fmt.Errorf("omniroute initial password is required in the vault before first managed start")
	}
	return nil
}

func (s *Server) ensureOmniRouteGeneratedSecret(target *string, vaultKey string, size int) error {
	if target == nil || strings.TrimSpace(*target) != "" {
		return nil
	}
	secret, err := randomSpaceAgentSecret(size)
	if err != nil {
		return err
	}
	*target = secret
	if s != nil && s.Vault != nil {
		if err := s.Vault.WriteSecret(vaultKey, secret); err != nil {
			return err
		}
	}
	return nil
}

func omniRouteStatus(ctx context.Context, s *Server, cfg *config.Config) map[string]interface{} {
	status, err := tools.OmniRouteSidecarStatus(ctx, cfg.Docker.Host, cfg)
	if err != nil {
		return map[string]interface{}{"enabled": cfg.OmniRoute.Enabled, "mode": cfg.OmniRoute.Mode, "status": "error", "message": err.Error()}
	}
	out := map[string]interface{}{
		"enabled":              status.Enabled,
		"mode":                 status.Mode,
		"status":               status.Status,
		"running":              status.Running,
		"url":                  status.URL,
		"provider_base_url":    status.ProviderBaseURL,
		"container_name":       status.ContainerName,
		"admin_setup_required": status.AdminSetupRequired,
	}
	if strings.TrimSpace(status.Message) != "" {
		out["message"] = status.Message
	}
	return out
}

func omniRouteStatusForRequest(ctx context.Context, s *Server, cfg *config.Config, r *http.Request) map[string]interface{} {
	out := omniRouteStatus(ctx, s, cfg)
	omniRouteRewriteBrowserURLForRequest(s, cfg, r, out)
	return out
}

func omniRouteBrowserBaseURLForRequest(s *Server, cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	sidecar, err := tools.ResolveOmniRouteSidecarConfig(cfg, cfg.Runtime.IsDocker)
	if err != nil {
		return ""
	}
	return manifestURLWithRequestHost(sidecar.BrowserBaseURL, r)
}

func omniRouteRewriteBrowserURL(r *http.Request, payload map[string]interface{}) {
	omniRouteRewriteBrowserURLForRequest(nil, nil, r, payload)
}

func omniRouteRewriteBrowserURLForRequest(_ *Server, _ *config.Config, r *http.Request, payload map[string]interface{}) {
	rawURL, ok := payload["url"].(string)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return
	}
	if requestLooksTailscale(r) {
		delete(payload, "url")
		return
	}
	payload["url"] = manifestURLWithRequestHost(rawURL, r)
}

func applyOmniRoutePatch(w http.ResponseWriter, r *http.Request, cfg *config.Config) bool {
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
		OmniRoute config.OmniRouteConfig `json:"omniroute"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		jsonError(w, "Invalid request payload", http.StatusBadRequest)
		return false
	}
	var rawReq map[string]json.RawMessage
	var rawOmniRoute map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawReq); err == nil {
		_ = json.Unmarshal(rawReq["omniroute"], &rawOmniRoute)
	}
	patch := req.OmniRoute
	if raw, ok := rawOmniRoute["enabled"]; ok {
		var enabled bool
		if err := json.Unmarshal(raw, &enabled); err == nil {
			cfg.OmniRoute.Enabled = enabled
		}
	} else if patch.Enabled {
		cfg.OmniRoute.Enabled = true
	}
	if strings.TrimSpace(patch.Mode) != "" {
		cfg.OmniRoute.Mode = normalizeOmniRouteMode(patch.Mode)
	}
	if strings.TrimSpace(patch.URL) != "" {
		cfg.OmniRoute.URL = patch.URL
	}
	if strings.TrimSpace(patch.ExternalBaseURL) != "" {
		cfg.OmniRoute.ExternalBaseURL = patch.ExternalBaseURL
	}
	if strings.TrimSpace(patch.ContainerName) != "" {
		cfg.OmniRoute.ContainerName = patch.ContainerName
	}
	if strings.TrimSpace(patch.Image) != "" {
		cfg.OmniRoute.Image = patch.Image
	}
	if strings.TrimSpace(patch.Host) != "" {
		cfg.OmniRoute.Host = patch.Host
	}
	if patch.Port > 0 {
		cfg.OmniRoute.Port = patch.Port
	}
	if patch.HostPort > 0 {
		cfg.OmniRoute.HostPort = patch.HostPort
	}
	if strings.TrimSpace(patch.NetworkName) != "" {
		cfg.OmniRoute.NetworkName = patch.NetworkName
	}
	if strings.TrimSpace(patch.DataVolume) != "" {
		cfg.OmniRoute.DataVolume = patch.DataVolume
	}
	if strings.TrimSpace(patch.HealthPath) != "" {
		cfg.OmniRoute.HealthPath = patch.HealthPath
	}
	if patch.MemoryMB > 0 {
		cfg.OmniRoute.MemoryMB = patch.MemoryMB
	}
	if strings.TrimSpace(patch.APIKey) != "" {
		cfg.OmniRoute.APIKey = patch.APIKey
	}
	if strings.TrimSpace(patch.InitialPassword) != "" {
		cfg.OmniRoute.InitialPassword = patch.InitialPassword
	}
	if strings.TrimSpace(patch.JWTSecret) != "" {
		cfg.OmniRoute.JWTSecret = patch.JWTSecret
	}
	if strings.TrimSpace(patch.APIKeySecret) != "" {
		cfg.OmniRoute.APIKeySecret = patch.APIKeySecret
	}
	if strings.TrimSpace(patch.WSBridgeSecret) != "" {
		cfg.OmniRoute.WSBridgeSecret = patch.WSBridgeSecret
	}
	return true
}

func normalizeOmniRouteMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "external") {
		return "external"
	}
	return "managed"
}
