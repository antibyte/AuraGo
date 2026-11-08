package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// handleMirageStatus reports the Mirage sidecar state for the Vision config banner.
func handleMirageStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		target := tools.MirageStatusTargetFor(s.Cfg)
		s.CfgMu.RUnlock()

		_ = json.NewEncoder(w).Encode(tools.MirageStatus(target))
	}
}

// mirageInputsChanged reports whether a settings save changed anything the
// sidecar container depends on.
func mirageInputsChanged(oldCfg, newCfg *config.Config) bool {
	return oldCfg.Vision.Mirage != newCfg.Vision.Mirage
}
