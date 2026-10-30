package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/obsidian"
)

func registerObsidianHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/obsidian/status", handleObsidianStatus(s))
}

func handleObsidianStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg.Obsidian
		s.CfgMu.RUnlock()

		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "disabled",
			})
			return
		}

		client, err := obsidian.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "error",
				"error":  "Failed to initialize Obsidian client",
			})
			return
		}
		defer client.Close()

		status, err := client.Ping(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "offline",
				"error":  "Failed to reach Obsidian REST API",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "online",
			"authenticated":    status.Authenticated,
			"api_version":      status.Versions["self"],
			"obsidian_version": status.Versions["obsidian"],
		})
	}
}

// handleObsidianTest checks the Obsidian REST API with the saved settings or the
// unsaved {"draft": ...} values (nothing is persisted).
func handleObsidianTest(s *Server) http.HandlerFunc {
	return handleConfiguredIntegrationTest(s, integrationTestSpec{
		name:    "Obsidian",
		enabled: func(cfg *config.Config) bool { return cfg.Obsidian.Enabled },
		check: func(ctx context.Context, cfg *config.Config) error {
			client, err := obsidian.NewClient(cfg.Obsidian, nil) // draftTestConfig resolved the key; never re-read the vault
			if err != nil {
				return errors.New("Failed to initialize Obsidian client")
			}
			defer client.Close()
			if _, err := client.Ping(ctx); err != nil {
				return errors.New("Failed to reach Obsidian REST API")
			}
			return nil
		},
	})
}
