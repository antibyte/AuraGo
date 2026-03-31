package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// handleTsNetStatus returns the current status of the tsnet embedded node.
func handleTsNetStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		host := ""
		if len(status.CertDNS) > 0 {
			host = status.CertDNS[0]
		} else if status.DNS != "" {
			host = status.DNS
		}
		host = strings.TrimSuffix(host, ".")
		webUIURL := ""
		homepageURL := ""
		publicURL := ""
		if host != "" && status.ServingHTTP {
			scheme := "https"
			if status.HTTPFallback {
				scheme = "http"
			}
			webUIURL = fmt.Sprintf("%s://%s", scheme, host)
			if status.FunnelActive {
				publicURL = "https://" + host
			}
		}
		if host != "" && status.HomepageServing {
			homepageURL = fmt.Sprintf("https://%s:8443", host)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":          s.Cfg.Tailscale.TsNet.Enabled,
			"serve_http":       s.Cfg.Tailscale.TsNet.ServeHTTP,
			"expose_homepage":  s.Cfg.Tailscale.TsNet.ExposeHomepage,
			"funnel":           s.Cfg.Tailscale.TsNet.Funnel,
			"running":          status.Running,
			"starting":         status.Starting,
			"serving_http":     status.ServingHTTP,
			"homepage_serving": status.HomepageServing,
			"http_fallback":    status.HTTPFallback,
			"funnel_active":    status.FunnelActive,
			"hostname":         status.Hostname,
			"dns":              status.DNS,
			"ips":              status.IPs,
			"cert_dns":         status.CertDNS,
			"web_ui_url":       webUIURL,
			"homepage_url":     homepageURL,
			"public_url":       publicURL,
			"error":            status.Error,
			"login_url":        status.LoginURL,
		})
	}
}

// handleTsNetStart (re)starts the tsnet node.
func handleTsNetStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.TsNetManager == nil {
			jsonError(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}

		if !s.Cfg.Tailscale.TsNet.Enabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Enable tsnet in config first",
			})
			return
		}

		handler := s.tsNetHandler
		if handler == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "tsnet handler not ready — restart AuraGo to initialize",
			})
			return
		}

		// If the node is already running, reconcile the active listeners with the
		// current config in-place instead of failing with "already running".
		if st := s.TsNetManager.GetStatus(); st.Running {
			go func() {
				if err := s.TsNetManager.ReconfigureExposure(handler); err != nil {
					s.Logger.Error("[tsnet] exposure reconfigure failed", "error", err)
				}
			}()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "starting"})
			return
		}

		// Launch in background — Start() blocks until auth/cert are ready
		go func() {
			if err := s.TsNetManager.Start(handler); err != nil {
				s.Logger.Error("[tsnet] Start via API failed", "error", err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "starting"})
	}
}

// handleTsNetStop stops the tsnet node.
func handleTsNetStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.TsNetManager == nil {
			jsonError(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}

		if err := s.TsNetManager.Stop(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			s.Logger.Error("Failed to stop tsnet node", "error", err)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to stop tsnet"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	}
}
