package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/desktopstore"
)

func handleDesktopStoreGodsEyeConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setDesktopStoreNoCacheHeaders(w)
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method == http.MethodPut && rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, "Store unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodGet {
			status, err := store.GodsEyeConfiguration(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		var req desktopstore.GodsEyeConfigUpdate
		if err := decodeDesktopJSON(w, r, &req, 16<<10); err != nil {
			jsonError(w, "Invalid configuration JSON", http.StatusBadRequest)
			return
		}
		op, err := store.ConfigureGodsEye(r.Context(), req)
		if err != nil {
			writeDesktopStoreStartError(w, err)
			return
		}
		s.runDesktopStoreOperation(op.ID)
		writeDesktopStoreOperationAccepted(w, op)
	}
}
