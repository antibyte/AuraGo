package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleTunnelStatus returns the current Cloudflare Tunnel status.
func handleTunnelStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		enabled := s.Cfg.CloudflareTunnel.Enabled
		s.CfgMu.RUnlock()

		if !enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"running": false,
			})
			return
		}

		cfg := s.buildTunnelConfig()
		result := tools.CloudflareTunnelStatus(cfg, s.Registry, s.Logger)
		w.Write([]byte(result))
	}
}

// handleTunnelQuick starts a temporary quick tunnel for sharing demos.
func handleTunnelQuick(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		enabled := s.Cfg.CloudflareTunnel.Enabled
		readOnly := s.Cfg.CloudflareTunnel.ReadOnly
		s.CfgMu.RUnlock()

		if !enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Cloudflare Tunnel is not enabled",
			})
			return
		}
		if readOnly {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Cloudflare Tunnel is in read-only mode",
			})
			return
		}

		var body struct {
			Port int `json:"port"`
		}
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&body)
		}

		cfg := s.buildTunnelConfig()
		result := tools.CloudflareTunnelQuickTunnel(cfg, s.Registry, s.Logger, body.Port)
		w.Write([]byte(result))
	}
}

// buildTunnelConfig creates a CloudflareTunnelConfig from the current server config.
func (s *Server) buildTunnelConfig() tools.CloudflareTunnelConfig {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()

	cfg := s.Cfg
	tc := tools.CloudflareTunnelConfig{
		Enabled:        cfg.CloudflareTunnel.Enabled,
		ReadOnly:       cfg.CloudflareTunnel.ReadOnly,
		Mode:           cfg.CloudflareTunnel.Mode,
		AutoStart:      cfg.CloudflareTunnel.AutoStart,
		AuthMethod:     cfg.CloudflareTunnel.AuthMethod,
		TunnelName:     cfg.CloudflareTunnel.TunnelName,
		AccountID:      cfg.CloudflareTunnel.AccountID,
		ExposeWebUI:    cfg.CloudflareTunnel.ExposeWebUI,
		ExposeHomepage: cfg.CloudflareTunnel.ExposeHomepage,
		MetricsPort:    cfg.CloudflareTunnel.MetricsPort,
		LogLevel:       cfg.CloudflareTunnel.LogLevel,
		DockerHost:     cfg.Docker.Host,
		WebUIPort:      cfg.Server.Port,
		HomepagePort:   cfg.Homepage.WebServerPort,
		DataDir:        cfg.Directories.DataDir,
	}
	for _, r := range cfg.CloudflareTunnel.CustomIngress {
		tc.CustomIngress = append(tc.CustomIngress, tools.CloudflareIngress{
			Hostname: r.Hostname,
			Service:  r.Service,
			Path:     r.Path,
		})
	}
	return tc
}
