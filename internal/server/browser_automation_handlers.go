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
)

func browserAutomationHealthHint(cfg *config.Config) (string, string) {
	if cfg == nil {
		return "", ""
	}
	rawURL := strings.TrimSpace(cfg.BrowserAutomation.URL)
	if rawURL == "" {
		return "", ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch {
	case !cfg.Runtime.IsDocker && host == "browser-automation":
		return "http://127.0.0.1:7331", "AuraGo is not running in Docker, so the Docker service hostname \"browser-automation\" cannot be resolved here. Use http://127.0.0.1:7331 for a local sidecar, or run AuraGo in Docker together with the browser-automation service."
	case cfg.Runtime.IsDocker && (host == "127.0.0.1" || host == "localhost" || host == "::1"):
		return "http://browser-automation:7331", "AuraGo is running in Docker, so localhost points to the AuraGo container itself. Use http://browser-automation:7331 or the reachable sidecar container/service name on the same Docker network."
	default:
		return "", ""
	}
}

func browserAutomationAnnotateHealth(cfg *config.Config, health map[string]interface{}) map[string]interface{} {
	if health == nil {
		return map[string]interface{}{"status": "error", "message": "browser automation health check failed"}
	}
	suggestedURL, hint := browserAutomationHealthHint(cfg)
	if hint == "" {
		return health
	}
	msg, _ := health["message"].(string)
	if msg == "" {
		health["message"] = hint
	} else if !strings.Contains(msg, hint) {
		health["message"] = fmt.Sprintf("%s Hint: %s", msg, hint)
	}
	if suggestedURL != "" {
		health["suggested_url"] = suggestedURL
	}
	return health
}

func ensureBrowserAutomationReady(ctx context.Context, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) map[string]interface{} {
	sidecarCfg, err := tools.ResolveBrowserAutomationSidecarConfig(cfg)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	if strings.EqualFold(cfg.BrowserAutomation.Mode, "sidecar") {
		tools.EnsureBrowserAutomationSidecarRunning(cfg.Docker.Host, sidecarCfg, logger)
	}

	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		health := browserAutomationAnnotateHealth(cfg, tools.BrowserAutomationHealth(ctx, cfg))
		if status, _ := health["status"].(string); status == "success" {
			return health
		}
		if retryable, ok := health["retryable"].(bool); ok && !retryable {
			return health
		}
		select {
		case <-ctx.Done():
			if msg, ok := browserAutomationAnnotateHealth(cfg, tools.BrowserAutomationHealth(context.Background(), cfg))["message"].(string); ok && msg != "" {
				return map[string]interface{}{"status": "error", "message": msg}
			}
			return map[string]interface{}{"status": "error", "message": ctx.Err().Error()}
		case <-deadline.C:
			if msg, ok := health["message"].(string); ok && msg != "" {
				return map[string]interface{}{"status": "error", "message": msg}
			}
			return map[string]interface{}{"status": "error", "message": "browser automation sidecar did not become ready in time"}
		case <-ticker.C:
		}
	}
}

func handleBrowserAutomationStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(browserAutomationAnnotateHealth(&cfg, tools.BrowserAutomationHealth(ctx, &cfg)))
	}
}

func handleBrowserAutomationTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		var req struct {
			BrowserAutomation struct {
				Enabled            *bool  `json:"enabled"`
				Mode               string `json:"mode"`
				URL                string `json:"url"`
				ContainerName      string `json:"container_name"`
				Image              string `json:"image"`
				AutoBuild          *bool  `json:"auto_build"`
				DockerfileDir      string `json:"dockerfile_dir"`
				SessionTTLMinutes  *int   `json:"session_ttl_minutes"`
				MaxSessions        *int   `json:"max_sessions"`
				AllowFileUploads   *bool  `json:"allow_file_uploads"`
				AllowFileDownloads *bool  `json:"allow_file_downloads"`
				AllowedDownloadDir string `json:"allowed_download_dir"`
				Headless           *bool  `json:"headless"`
				ReadOnly           *bool  `json:"readonly"`
				ScreenshotsDir     string `json:"screenshots_dir"`
				Viewport           struct {
					Width  *int `json:"width"`
					Height *int `json:"height"`
				} `json:"viewport"`
			} `json:"browser_automation"`
			ToolEnabled *bool `json:"tool_enabled"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)); err == nil && len(strings.TrimSpace(string(data))) > 0 {
				if err := json.Unmarshal(data, &req); err != nil {
					jsonError(w, "Invalid request payload", http.StatusBadRequest)
					return
				}
			}
		}
		if req.BrowserAutomation.Enabled != nil {
			cfg.BrowserAutomation.Enabled = *req.BrowserAutomation.Enabled
		}
		if req.BrowserAutomation.Mode != "" {
			cfg.BrowserAutomation.Mode = req.BrowserAutomation.Mode
		}
		if req.BrowserAutomation.URL != "" {
			cfg.BrowserAutomation.URL = req.BrowserAutomation.URL
		}
		if req.BrowserAutomation.ContainerName != "" {
			cfg.BrowserAutomation.ContainerName = req.BrowserAutomation.ContainerName
		}
		if req.BrowserAutomation.Image != "" {
			cfg.BrowserAutomation.Image = req.BrowserAutomation.Image
		}
		if req.BrowserAutomation.AutoBuild != nil {
			cfg.BrowserAutomation.AutoBuild = *req.BrowserAutomation.AutoBuild
		}
		if req.BrowserAutomation.DockerfileDir != "" {
			cfg.BrowserAutomation.DockerfileDir = req.BrowserAutomation.DockerfileDir
		}
		if req.BrowserAutomation.SessionTTLMinutes != nil {
			cfg.BrowserAutomation.SessionTTLMinutes = *req.BrowserAutomation.SessionTTLMinutes
		}
		if req.BrowserAutomation.MaxSessions != nil {
			cfg.BrowserAutomation.MaxSessions = *req.BrowserAutomation.MaxSessions
		}
		if req.BrowserAutomation.AllowFileUploads != nil {
			cfg.BrowserAutomation.AllowFileUploads = *req.BrowserAutomation.AllowFileUploads
		}
		if req.BrowserAutomation.AllowFileDownloads != nil {
			cfg.BrowserAutomation.AllowFileDownloads = *req.BrowserAutomation.AllowFileDownloads
		}
		if req.BrowserAutomation.AllowedDownloadDir != "" {
			cfg.BrowserAutomation.AllowedDownloadDir = req.BrowserAutomation.AllowedDownloadDir
		}
		if req.BrowserAutomation.Headless != nil {
			cfg.BrowserAutomation.Headless = *req.BrowserAutomation.Headless
		}
		if req.BrowserAutomation.ReadOnly != nil {
			cfg.BrowserAutomation.ReadOnly = *req.BrowserAutomation.ReadOnly
		}
		if req.BrowserAutomation.ScreenshotsDir != "" {
			cfg.BrowserAutomation.ScreenshotsDir = req.BrowserAutomation.ScreenshotsDir
		}
		if req.BrowserAutomation.Viewport.Width != nil {
			cfg.BrowserAutomation.Viewport.Width = *req.BrowserAutomation.Viewport.Width
		}
		if req.BrowserAutomation.Viewport.Height != nil {
			cfg.BrowserAutomation.Viewport.Height = *req.BrowserAutomation.Viewport.Height
		}
		if cfg.BrowserAutomation.Enabled {
			cfg.Tools.BrowserAutomation.Enabled = true
		} else if req.ToolEnabled != nil {
			cfg.Tools.BrowserAutomation.Enabled = *req.ToolEnabled
		}
		if !cfg.BrowserAutomation.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Browser automation is disabled",
			})
			return
		}
		if !cfg.Tools.BrowserAutomation.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Browser automation tool is disabled",
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(ensureBrowserAutomationReady(ctx, &cfg, s.Logger))
	}
}
