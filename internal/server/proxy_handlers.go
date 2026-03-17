package server

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleProxyStatus returns the current status of the security proxy container.
func handleProxyStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		status, err := s.ProxyManager.Status()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"enabled": s.Cfg.SecurityProxy.Enabled,
			"proxy":   status,
		})
	}
}

// handleProxyStart starts (or restarts) the security proxy container.
func handleProxyStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.SecurityProxy.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Security proxy is not enabled in configuration",
			})
			return
		}

		if err := s.ProxyManager.Start(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Security proxy started",
		})
	}
}

// handleProxyStop stops the security proxy container.
func handleProxyStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if err := s.ProxyManager.Stop(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Security proxy stopped",
		})
	}
}

// handleProxyDestroy stops and removes the security proxy container.
func handleProxyDestroy(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if err := s.ProxyManager.Destroy(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Security proxy destroyed",
		})
	}
}

// handleProxyReload regenerates the Caddyfile and reloads Caddy configuration.
func handleProxyReload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if err := s.ProxyManager.Reload(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Security proxy configuration reloaded",
		})
	}
}

// handleProxyLogs returns the last N lines of the proxy container logs.
func handleProxyLogs(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		tail := 100
		if t := r.URL.Query().Get("tail"); t != "" {
			if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 && parsed <= 1000 {
				tail = parsed
			}
		}

		logs, err := s.ProxyManager.Logs(tail)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"logs":   logs,
		})
	}
}
