package server

// fritzbox_handlers.go – REST endpoints for Fritz!Box UI integration.
// Provides:
//  GET  /api/fritzbox/status  – current connection/config status
//  POST /api/fritzbox/test    – test connection with optional overrides

import (
	"encoding/json"
	"net/http"

	"aurago/internal/fritzbox"
)

// handleFritzBoxStatus returns a brief status object for the Fritz!Box config panel.
func handleFritzBoxStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		enabled := s.Cfg.FritzBox.Enabled
		host := s.Cfg.FritzBox.Host
		port := s.Cfg.FritzBox.Port
		useHTTPS := s.Cfg.FritzBox.HTTPS
		s.CfgMu.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":    enabled,
			"host":       host,
			"port":       port,
			"https":      useHTTPS,
			"configured": host != "",
		})
	}
}

// handleFritzBoxTest tests the Fritz!Box connection.
// Accepts an optional JSON body {host, port, https, username, password};
// any omitted/empty field falls back to the saved config / vault value.
func handleFritzBoxTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		// Parse optional override body.
		var body struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			HTTPS    *bool  `json:"https"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Build test config from saved config + overrides.
		s.CfgMu.RLock()
		testCfg := *s.Cfg // shallow copy
		s.CfgMu.RUnlock()

		if body.Host != "" {
			testCfg.FritzBox.Host = body.Host
		}
		if body.Port != 0 {
			testCfg.FritzBox.Port = body.Port
		}
		if body.HTTPS != nil {
			testCfg.FritzBox.HTTPS = *body.HTTPS
		}
		if body.Username != "" {
			testCfg.FritzBox.Username = body.Username
		}
		if body.Password != "" {
			testCfg.FritzBox.Password = body.Password
		}

		// Vault fallback for password.
		if testCfg.FritzBox.Password == "" && s.Vault != nil {
			if v, _ := s.Vault.ReadSecret("fritzbox_password"); v != "" {
				testCfg.FritzBox.Password = v
			}
		}

		if testCfg.FritzBox.Host == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Host is required (not set in config)",
			})
			return
		}

		// Force enable for the test regardless of configuration.
		testCfg.FritzBox.Enabled = true
		testCfg.FritzBox.System.Enabled = true

		c, err := fritzbox.NewClient(testCfg)
		if err != nil {
			s.Logger.Error("Failed to initialize FritzBox client", "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to initialize Fritz!Box client",
			})
			return
		}
		defer c.Close()

		info, err := c.GetSystemInfo()
		if err != nil {
			s.Logger.Error("FritzBox system info request failed", "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Failed to connect to Fritz!Box",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"model":    info.ModelName,
			"firmware": info.SoftwareVersion,
		})
	}
}
