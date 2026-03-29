package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleCloudflareTunnelStatus returns the current tunnel status.
func handleCloudflareTunnelStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.CfgMu.RLock()
		cfg := tools.CloudflareTunnelConfig{
			Enabled:        s.Cfg.CloudflareTunnel.Enabled,
			ReadOnly:       s.Cfg.CloudflareTunnel.ReadOnly,
			Mode:           s.Cfg.CloudflareTunnel.Mode,
			AuthMethod:     s.Cfg.CloudflareTunnel.AuthMethod,
			TunnelName:     s.Cfg.CloudflareTunnel.TunnelName,
			AccountID:      s.Cfg.CloudflareTunnel.AccountID,
			LoopbackPort:   s.Cfg.CloudflareTunnel.LoopbackPort,
			ExposeWebUI:    s.Cfg.CloudflareTunnel.ExposeWebUI,
			ExposeHomepage: s.Cfg.CloudflareTunnel.ExposeHomepage,
			MetricsPort:    s.Cfg.CloudflareTunnel.MetricsPort,
			LogLevel:       s.Cfg.CloudflareTunnel.LogLevel,
			WebUIPort:      s.Cfg.Server.Port,
			HTTPSEnabled:   s.Cfg.Server.HTTPS.Enabled,
			HTTPSPort:      s.Cfg.Server.HTTPS.HTTPSPort,
		}
		enabled := s.Cfg.CloudflareTunnel.Enabled
		s.CfgMu.RUnlock()
		status := tools.CloudflareTunnelStatus(cfg, s.Registry, s.Logger)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"enabled": enabled,
			"tunnel":  status,
		})
	}
}

// handleCloudflareTunnelRestart stops and starts the tunnel.
func handleCloudflareTunnelRestart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.CloudflareTunnel.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Cloudflare Tunnel is not enabled in config",
			})
			return
		}

		s.CfgMu.RLock()
		tunnelCfg := tools.CloudflareTunnelConfig{
			Enabled:        s.Cfg.CloudflareTunnel.Enabled,
			ReadOnly:       s.Cfg.CloudflareTunnel.ReadOnly,
			Mode:           s.Cfg.CloudflareTunnel.Mode,
			AutoStart:      s.Cfg.CloudflareTunnel.AutoStart,
			AuthMethod:     s.Cfg.CloudflareTunnel.AuthMethod,
			TunnelName:     s.Cfg.CloudflareTunnel.TunnelName,
			AccountID:      s.Cfg.CloudflareTunnel.AccountID,
			TunnelID:       s.Cfg.CloudflareTunnel.TunnelID,
			LoopbackPort:   s.Cfg.CloudflareTunnel.LoopbackPort,
			ExposeWebUI:    s.Cfg.CloudflareTunnel.ExposeWebUI,
			ExposeHomepage: s.Cfg.CloudflareTunnel.ExposeHomepage,
			MetricsPort:    s.Cfg.CloudflareTunnel.MetricsPort,
			LogLevel:       s.Cfg.CloudflareTunnel.LogLevel,
			DockerHost:     s.Cfg.Docker.Host,
			DataDir:        s.Cfg.Directories.DataDir,
			WebUIPort:      s.Cfg.Server.Port,
			HTTPSEnabled:   s.Cfg.Server.HTTPS.Enabled,
			HTTPSPort:      s.Cfg.Server.HTTPS.HTTPSPort,
		}
		s.CfgMu.RUnlock()
		result := tools.CloudflareTunnelRestart(
			tunnelCfg,
			s.Vault,
			s.Registry,
			s.Logger,
		)

		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"message": result,
			})
			return
		}
		resp["status"] = "ok"
		json.NewEncoder(w).Encode(resp)
	}
}
