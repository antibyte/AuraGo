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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	cfgSnapshot := *s.Cfg
	s.CfgMu.RUnlock()

	return cloudflareTunnelRuntimeConfig(&cfgSnapshot)
}
