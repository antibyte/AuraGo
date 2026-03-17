package server

import (
	"encoding/json"
	"net/http"
)

// handleTsNetStatus returns the current status of the tsnet embedded node.
func handleTsNetStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.TsNetManager == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": false,
				"running": false,
			})
			return
		}

		status := s.TsNetManager.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":  s.Cfg.Tailscale.TsNet.Enabled,
			"running":  status.Running,
			"hostname": status.Hostname,
			"dns":      status.DNS,
			"ips":      status.IPs,
			"cert_dns": status.CertDNS,
			"error":    status.Error,
		})
	}
}

// handleTsNetStart starts the tsnet node.
func handleTsNetStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.TsNetManager == nil {
			http.Error(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}

		// Use the main mux handler wrapped with security headers
		// Since we don't have the mux here, we return an error and let the user
		// enable tsnet via config → restart instead of runtime start
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "info",
			"message": "Enable tsnet in config and restart AuraGo to start the Tailscale node",
		})
	}
}

// handleTsNetStop stops the tsnet node.
func handleTsNetStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.TsNetManager == nil {
			http.Error(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}

		if err := s.TsNetManager.Stop(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	}
}
