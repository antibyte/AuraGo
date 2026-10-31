package server

// fritzbox_handlers.go – REST endpoints for Fritz!Box UI integration.
// Provides:
//  GET  /api/fritzbox/status  – current connection/config status
//  POST /api/fritzbox/test    – test connection with the draft values

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
		webPort := s.Cfg.FritzBox.WebPort
		insecureSkipVerify := s.Cfg.FritzBox.InsecureSkipVerify
		s.CfgMu.RUnlock()

		status := map[string]interface{}{
			"enabled":              enabled,
			"host":                 host,
			"port":                 port,
			"https":                useHTTPS,
			"web_port":             webPort,
			"insecure_skip_verify": insecureSkipVerify,
			"configured":           host != "",
		}
		if r.URL.Query().Get("check") != "1" {
			json.NewEncoder(w).Encode(status)
			return
		}
		if !enabled || host == "" {
			status["connected"] = false
			json.NewEncoder(w).Encode(status)
			return
		}

		s.CfgMu.RLock()
		testCfg := *s.Cfg
		s.CfgMu.RUnlock()
		if testCfg.FritzBox.Password == "" && s.Vault != nil {
			if v, _ := s.Vault.ReadSecret("fritzbox_password"); v != "" {
				testCfg.FritzBox.Password = v
			}
		}
		testCfg.FritzBox.Enabled = true
		testCfg.FritzBox.System.Enabled = true
		c, err := fritzbox.NewClient(testCfg)
		if err != nil {
			status["connected"] = false
			status["message"] = "Failed to initialize Fritz!Box client"
			json.NewEncoder(w).Encode(status)
			return
		}
		defer c.Close()
		info, err := c.GetSystemInfo()
		if err != nil {
			status["connected"] = false
			status["message"] = "Failed to connect to Fritz!Box"
			json.NewEncoder(w).Encode(status)
			return
		}
		status["connected"] = true
		status["model"] = info.ModelName
		status["firmware"] = info.SoftwareVersion
		json.NewEncoder(w).Encode(status)
	}
}

// handleFritzBoxTest tests the Fritz!Box connection.
// The optional body {"draft": {"fritzbox": {...}}} carries the values typed in; the
// password follows the target (draftTestConfig): a stored password is only used while
// host, port, TLS and user stay as saved.
func handleFritzBoxTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		draftCfg, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		testCfg := *draftCfg

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
