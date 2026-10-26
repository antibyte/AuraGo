package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/realtimespeech"
)

func handleTTSRoutes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			cfg = &config.Config{}
		}
		type routeStatus struct {
			Code     string `json:"code"`
			Provider string `json:"provider"`
			Ready    bool   `json:"ready"`
		}
		provider := strings.TrimSpace(glassTTSConfig(cfg).Provider)
		ready := glassTTSReady(cfg)
		routes := []routeStatus{
			{Code: "web_chat", Provider: strings.TrimSpace(buildChatVoiceOutputTTSConfig(cfg, "").Provider), Ready: chatVoiceOutputTTSConfigured(cfg)},
			{Code: "tool", Provider: provider, Ready: ready},
			{Code: "glass", Provider: provider, Ready: ready},
			{Code: "live_speech"},
		}
		if cfg.RealtimeSpeech.Enabled {
			selected := strings.TrimSpace(cfg.RealtimeSpeech.DefaultProfile)
			for _, profile := range cfg.RealtimeSpeech.Profiles {
				if profile.ID != selected || !profile.Enabled {
					continue
				}
				live := &routes[3]
				live.Provider = strings.TrimSpace(profile.Provider)
				catalog, ok := realtimespeech.Provider(live.Provider)
				live.Ready = ok && (!catalog.Capabilities.RequiresAPIKey || strings.TrimSpace(profile.APIKey) != "")
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"routes": routes})
	}
}
