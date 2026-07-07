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
		cfg := s.buildTunnelConfig()
		status := tools.CloudflareTunnelStatus(cfg, s.Registry, s.Logger)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"enabled": cfg.Enabled,
			"tunnel":  status,
		})
	}
}

// handleCloudflareTunnelRestart stops and starts the tunnel.
func handleCloudflareTunnelRestart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		tunnelCfg := s.buildTunnelConfig()
		if !tunnelCfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Cloudflare Tunnel is not enabled in config",
			})
			return
		}

		result := tools.CloudflareTunnelRestart(
			tunnelCfg,
			s.Vault,
			s.Registry,
			s.Logger,
		)

		writeCloudflareTunnelToolResponse(w, result)
	}
}

func writeCloudflareTunnelToolResponse(w http.ResponseWriter, result string) {
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": result,
			"error":   result,
		})
		return
	}
	if status, _ := resp["status"].(string); status == "" {
		resp["status"] = "ok"
	}
	if status, _ := resp["status"].(string); status != "" && status != "ok" {
		if _, ok := resp["error"]; !ok {
			if msg, _ := resp["message"].(string); msg != "" {
				resp["error"] = msg
			}
		}
	}
	json.NewEncoder(w).Encode(resp)
}
