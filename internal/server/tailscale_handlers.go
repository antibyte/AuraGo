package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

func registerTailscaleHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/tailscale/status", handleTailscaleAPIStatus(s))
	mux.HandleFunc("/api/tailscale/test", handleTailscaleAPITest(s))
}

func handleTailscaleAPIStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.CfgMu.RLock()
		cfg := s.Cfg.Tailscale
		s.CfgMu.RUnlock()
		writeTailscaleAPIProbeResult(w, tailscaleProbeInput{Enabled: cfg.Enabled, APIKey: cfg.APIKey, Tailnet: cfg.Tailnet, ReadOnly: cfg.ReadOnly}, false)
	}
}

func handleTailscaleAPITest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// The test uses the values typed into the form (draft); the key comes from the draft or the vault.
		draft, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		cfg := draft.Tailscale
		// A test of unsaved values does not depend on the integration being switched on yet.
		writeTailscaleAPIProbeResult(w, tailscaleProbeInput{Context: r.Context(), Enabled: true, APIKey: cfg.APIKey, Tailnet: cfg.Tailnet, ReadOnly: cfg.ReadOnly}, true)
	}
}

// tailscaleProbeInput is the Tailscale API section as the probe needs it.
type tailscaleProbeInput struct {
	Context  context.Context
	Enabled  bool
	APIKey   string
	Tailnet  string
	ReadOnly bool
}

func writeTailscaleAPIProbeResult(w http.ResponseWriter, cfg tailscaleProbeInput, test bool) {
	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disabled",
			"message": "Tailscale API integration is not enabled",
		})
		return
	}
	if cfg.APIKey == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_credentials",
			"message": "Tailscale API key is not configured",
		})
		return
	}

	tsCfg := tools.TailscaleConfig{
		Context:  cfg.Context,
		APIKey:   cfg.APIKey,
		Tailnet:  cfg.Tailnet,
		ReadOnly: cfg.ReadOnly,
	}
	raw := tools.TailscaleListDevices(tsCfg)
	raw = strings.TrimPrefix(raw, "Tool Output: ")

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to parse Tailscale API response",
		})
		return
	}

	status, _ := payload["status"].(string)
	if status != "ok" {
		msg, _ := payload["message"].(string)
		if msg == "" {
			if body, ok := payload["body"].(string); ok && body != "" {
				msg = body
			} else {
				msg = "Tailscale API connection failed"
			}
		}
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": msg,
		})
		return
	}

	count, _ := payload["count"].(float64)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Tailscale API connection successful",
		"count":   int(count),
	})
}
