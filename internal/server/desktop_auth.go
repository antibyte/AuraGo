package server

import (
	"net/http"
	"strings"
)

const (
	desktopScopeRead  = "desktop:read"
	desktopScopeWrite = "desktop:write"
	desktopScopeAdmin = "desktop:admin"
)

func isDesktopScopedAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/desktop/") || strings.HasPrefix(path, "/api/code-studio/")
}

func requireDesktopPermission(s *Server, w http.ResponseWriter, r *http.Request, requiredScope string) bool {
	if requiredScope == "" {
		requiredScope = desktopScopeRead
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if rawToken != "" && s != nil && s.TokenManager != nil {
			if desktopTokenHasScope(s, rawToken, requiredScope) {
				return true
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"desktop_scope_required","message":"Desktop API scope is required for this endpoint."}`))
		return false
	}

	authEnabled := false
	sessionSecret := ""
	if s != nil && s.Cfg != nil {
		s.CfgMu.RLock()
		authEnabled = s.Cfg.Auth.Enabled
		sessionSecret = s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
	}
	if !authEnabled {
		return true
	}
	if IsAuthenticated(r, sessionSecret) {
		return true
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
	return false
}

func desktopTokenHasScope(s *Server, rawToken, requiredScope string) bool {
	if s == nil || s.TokenManager == nil {
		return false
	}
	if _, ok := s.TokenManager.Validate(rawToken, requiredScope); ok {
		return true
	}
	if _, ok := s.TokenManager.Validate(rawToken, desktopScopeAdmin); ok {
		return true
	}
	if _, ok := s.TokenManager.Validate(rawToken, "admin"); ok {
		return true
	}
	return false
}

func desktopMethodScope(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return desktopScopeRead
	default:
		return desktopScopeWrite
	}
}
