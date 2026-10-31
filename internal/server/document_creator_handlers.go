package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

// handleGotenbergTest checks whether the configured Gotenberg sidecar is reachable.
func handleGotenbergTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Optional {"draft": ...} tests the typed-in URL (nothing is persisted).
		draftCfg, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		cfg := draftCfg.Tools.DocumentCreator
		// The managed-container restart check inspects the running container, so it
		// stays on the saved configuration.
		snapshot := s.ConfigSnapshot()
		if snapshot == nil {
			snapshot = draftCfg
		}

		if cfg.Gotenberg.URL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "error",
				"message":   "Gotenberg URL is not configured",
				"reachable": false, "security_verified": false,
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		restartRequired := tools.GotenbergRestartRequired(ctx, snapshot)

		resultStr := tools.GotenbergHealth(ctx, &cfg.Gotenberg)

		// Inject the active backend so the UI can confirm what the server is using
		var resultMap map[string]interface{}
		if json.Unmarshal([]byte(resultStr), &resultMap) == nil {
			resultMap["active_backend"] = cfg.Backend
			resultMap["restart_required"] = restartRequired
			if _, ok := resultMap["reachable"]; !ok {
				resultMap["reachable"] = false
			}
			if _, ok := resultMap["security_verified"]; !ok {
				resultMap["security_verified"] = false
			}
			out, _ := json.Marshal(resultMap)
			w.Write(out)
		} else {
			w.Write([]byte(resultStr))
		}
	}
}
