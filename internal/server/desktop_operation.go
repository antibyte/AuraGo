package server

import (
	"encoding/json"
	"net/http"
)

// Desktop operations are independent of token scopes: an administrator still
// observes readonly, while authenticated cleanup remains available after revocation.
type desktopOperation uint8

const (
	desktopRead desktopOperation = iota
	desktopWrite
	desktopExecute
	desktopStop
)

func desktopMethodOperation(method string) desktopOperation {
	if method == http.MethodGet || method == http.MethodHead {
		return desktopRead
	}
	return desktopWrite
}

func requireDesktopOperation(s *Server, w http.ResponseWriter, r *http.Request, scope string, operation desktopOperation) bool {
	if !requireDesktopPermission(s, w, r, scope) {
		return false
	}
	if operation == desktopRead || operation == desktopStop {
		return true
	}
	s.CfgMu.RLock()
	enabled, readonly := s.Cfg.VirtualDesktop.Enabled, s.Cfg.VirtualDesktop.ReadOnly
	s.CfgMu.RUnlock()
	if readonly {
		writeDesktopPolicyError(w, "desktop_readonly", "The desktop is read-only.")
		return false
	}
	if !enabled {
		writeDesktopPolicyError(w, "desktop_disabled", "The desktop is disabled.")
		return false
	}
	return true
}

func writeDesktopPolicyError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code, "message": message})
}
