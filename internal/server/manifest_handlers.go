package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func handleManifestStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentManifestConfig(s)
		writeManifestJSON(w, manifestStatusForRequest(r.Context(), s, &cfg, r))
	}
}

func handleManifestTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// The test runs with the values typed in; stored secrets follow the target (draftTestConfig).
		draft, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		cfg := *draft
		config.NormalizeManifestRoutingConfig(&cfg.Manifest.Routing)
		writeManifestJSON(w, manifestStatusForRequest(r.Context(), s, &cfg, r))
	}
}

func handleManifestStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentManifestConfig(s)
		if !cfg.Manifest.Enabled {
			writeManifestJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "Manifest integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Manifest.Mode), "external") {
			status := manifestStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "Manifest is configured in external mode; no sidecars to start"
			writeManifestJSON(w, status)
			return
		}
		if err := s.ensureManifestSecrets(&cfg); err != nil {
			writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Manifest.Mode, "status": "setup_required", "admin_setup_required": true, "message": err.Error()})
			return
		}
		if _, err := tools.ResolveManifestSidecarConfig(&cfg, cfg.Runtime.IsDocker); err != nil {
			writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Manifest.Mode, "status": "setup_required", "admin_setup_required": true, "message": err.Error()})
			return
		}
		browserBaseURL := manifestBrowserBaseURLForRequest(s, &cfg, r)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := tools.EnsureManifestSidecarsRunningWithBrowserURL(ctx, cfg.Docker.Host, &cfg, browserBaseURL, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[Manifest] Manual start failed", "error", err)
			}
		}()
		writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Manifest.Mode, "status": "starting", "message": "Manifest sidecars are starting"})
	}
}

func handleManifestStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := currentManifestConfig(s)
		if !cfg.Manifest.Enabled {
			writeManifestJSON(w, map[string]interface{}{"enabled": false, "status": "disabled", "message": "Manifest integration is disabled"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Manifest.Mode), "external") {
			status := manifestStatusForRequest(r.Context(), s, &cfg, r)
			status["message"] = "Manifest is configured in external mode; no sidecars to stop"
			writeManifestJSON(w, status)
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := tools.StopManifestSidecars(ctx, cfg.Docker.Host, &cfg, s.Logger); err != nil && s.Logger != nil {
				s.Logger.Warn("[Manifest] Manual stop failed", "error", err)
			}
		}()
		writeManifestJSON(w, map[string]interface{}{"enabled": true, "mode": cfg.Manifest.Mode, "status": "stopping", "message": "Manifest sidecars are stopping"})
	}
}

func currentManifestConfig(s *Server) config.Config {
	if s == nil || s.Cfg == nil {
		return config.Config{}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return *s.Cfg
}

func (s *Server) ensureManifestSecrets(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !cfg.Manifest.Enabled || strings.EqualFold(strings.TrimSpace(cfg.Manifest.Mode), "external") {
		return nil
	}
	if strings.TrimSpace(cfg.Manifest.PostgresPassword) == "" {
		secret, err := randomSpaceAgentSecret(24)
		if err != nil {
			return err
		}
		cfg.Manifest.PostgresPassword = secret
		if s != nil && s.Vault != nil {
			if err := s.Vault.WriteSecret("manifest_postgres_password", secret); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(cfg.Manifest.BetterAuthSecret) == "" {
		secret, err := randomSpaceAgentSecret(32)
		if err != nil {
			return err
		}
		cfg.Manifest.BetterAuthSecret = secret
		if s != nil && s.Vault != nil {
			if err := s.Vault.WriteSecret("manifest_better_auth_secret", secret); err != nil {
				return err
			}
		}
	}
	if s != nil && s.Cfg != nil {
		if s.Cfg == cfg {
			// Config hot-reload already holds CfgMu while calling this with the
			// live config pointer. Locking again would self-deadlock the UI.
			s.Cfg.Manifest.PostgresPassword = cfg.Manifest.PostgresPassword
			s.Cfg.Manifest.BetterAuthSecret = cfg.Manifest.BetterAuthSecret
		} else {
			s.CfgMu.Lock()
			s.Cfg.Manifest.PostgresPassword = cfg.Manifest.PostgresPassword
			s.Cfg.Manifest.BetterAuthSecret = cfg.Manifest.BetterAuthSecret
			s.CfgMu.Unlock()
		}
	}
	return nil
}

func manifestStatus(ctx context.Context, s *Server, cfg *config.Config) map[string]interface{} {
	status, err := tools.ManifestSidecarStatus(ctx, cfg.Docker.Host, cfg)
	if err != nil {
		return map[string]interface{}{"enabled": cfg.Manifest.Enabled, "mode": cfg.Manifest.Mode, "status": "error", "message": err.Error()}
	}
	out := map[string]interface{}{
		"enabled":              status.Enabled,
		"mode":                 status.Mode,
		"status":               status.Status,
		"running":              status.Running,
		"url":                  status.URL,
		"provider_base_url":    status.ProviderBaseURL,
		"container_name":       status.ContainerName,
		"postgres_container":   status.PostgresContainer,
		"admin_setup_required": status.AdminSetupRequired,
	}
	if strings.TrimSpace(status.Message) != "" {
		out["message"] = status.Message
	}
	return out
}

func manifestStatusForRequest(ctx context.Context, s *Server, cfg *config.Config, r *http.Request) map[string]interface{} {
	out := manifestStatus(ctx, s, cfg)
	manifestRewriteBrowserURLForRequest(s, cfg, r, out)
	return out
}

func manifestBrowserBaseURLForRequest(s *Server, cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	sidecar, err := tools.ResolveManifestSidecarConfig(cfg, cfg.Runtime.IsDocker)
	if err != nil {
		return ""
	}
	return manifestBrowserURL(s, cfg, r, sidecar.BrowserBaseURL)
}

func manifestRewriteBrowserURL(r *http.Request, payload map[string]interface{}) {
	manifestRewriteBrowserURLForRequest(nil, nil, r, payload)
}

func manifestRewriteBrowserURLForRequest(s *Server, cfg *config.Config, r *http.Request, payload map[string]interface{}) {
	rawURL, ok := payload["url"].(string)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return
	}
	if cfg != nil {
		rewrite := manifestBrowserURL(s, cfg, r, rawURL)
		if strings.TrimSpace(rewrite) == "" {
			delete(payload, "url")
			return
		}
		payload["url"] = rewrite
		return
	}
	if requestLooksTailscale(r) {
		delete(payload, "url")
		return
	}
	payload["url"] = manifestURLWithRequestHost(rawURL, r)
}

func writeManifestJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
