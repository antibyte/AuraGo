package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/desktop"
	"aurago/internal/inventory"
)

const (
	desktopRemoteScopeAll          = "desktop:remote"
	desktopRemoteScopeDevicePrefix = "desktop:remote:device:"
	desktopRemoteScopeTagPrefix    = "desktop:remote:tag:"
)

func withDesktopRemoteGuard(s *Server, action, expectedProtocol string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawToken, bearerToken, ok := requireDesktopRemoteBaseAccess(s, w, r)
		if !ok {
			return
		}

		ip := ClientIP(r, s != nil && s.Cfg != nil && s.Cfg.Server.HTTPS.BehindProxy)
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		rateKeys := desktopRemoteRateLimitKeys(action, ip, deviceID)
		if IsLockedOutAny(rateKeys...) {
			auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "rate_limited")
			writeDesktopRemoteGuardError(w, "too_many_remote_attempts", http.StatusTooManyRequests)
			return
		}
		if deviceID == "" {
			if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
				if bearerToken && !desktopRemoteTokenHasAdminOrGlobalScope(s, rawToken) {
					recordDesktopRemoteRateFailure(s, rateKeys...)
					auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "missing_device_id_for_scoped_token")
					writeDesktopRemoteGuardError(w, "missing device_id", http.StatusBadRequest)
					return
				}
				auditDesktopRemoteAttempt(s, r, action, deviceID, "attempt", "multipart_device_id_deferred")
				next(w, r)
				return
			}
			recordDesktopRemoteRateFailure(s, rateKeys...)
			auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "missing_device_id")
			writeDesktopRemoteGuardError(w, "missing device_id", http.StatusBadRequest)
			return
		}
		if s != nil && s.InventoryDB != nil {
			device, err := inventory.GetDeviceByID(s.InventoryDB, deviceID)
			if err != nil {
				recordDesktopRemoteRateFailure(s, rateKeys...)
				auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "device_not_found")
				writeDesktopRemoteGuardError(w, "device not found", http.StatusNotFound)
				return
			}
			if expectedProtocol != "" && strings.TrimSpace(device.Protocol) != expectedProtocol {
				recordDesktopRemoteRateFailure(s, rateKeys...)
				auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "protocol_mismatch")
				writeDesktopRemoteGuardError(w, fmt.Sprintf("device protocol is %q, expected %s", device.Protocol, expectedProtocol), http.StatusBadRequest)
				return
			}
			if bearerToken && !desktopRemoteTokenAllowsDevice(s, rawToken, device) {
				recordDesktopRemoteRateFailure(s, rateKeys...)
				auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "scope_denied")
				writeDesktopRemoteGuardError(w, "desktop remote scope required", http.StatusForbidden)
				return
			}
		} else if bearerToken && !desktopRemoteTokenHasAdminOrGlobalScope(s, rawToken) {
			recordDesktopRemoteRateFailure(s, rateKeys...)
			auditDesktopRemoteAttempt(s, r, action, deviceID, "blocked", "inventory_unavailable_for_scoped_token")
			writeDesktopRemoteGuardError(w, "desktop remote scope required", http.StatusForbidden)
			return
		}

		auditDesktopRemoteAttempt(s, r, action, deviceID, "attempt", "")
		next(w, r)
	}
}

func requireDesktopRemoteBaseAccess(s *Server, w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	rawToken, bearerToken := desktopRemoteBearerToken(r)
	if !bearerToken {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return "", false, false
		}
		return "", false, true
	}
	if rawToken == "" || s == nil || s.TokenManager == nil {
		writeDesktopRemoteGuardError(w, "desktop remote scope required", http.StatusForbidden)
		return rawToken, true, false
	}
	if _, ok := s.TokenManager.Validate(rawToken, ""); ok {
		return rawToken, true, true
	}
	writeDesktopRemoteGuardError(w, "desktop remote scope required", http.StatusForbidden)
	return rawToken, true, false
}

func desktopRemoteBearerToken(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")), true
}

func desktopRemoteTokenAllowsDevice(s *Server, rawToken string, device inventory.DeviceRecord) bool {
	if desktopRemoteTokenHasAdminOrGlobalScope(s, rawToken) {
		return true
	}
	deviceID := strings.TrimSpace(device.ID)
	if deviceID != "" && desktopRemoteTokenHasExactScope(s, rawToken, desktopRemoteScopeDevicePrefix+deviceID) {
		return true
	}
	for _, tag := range device.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && desktopRemoteTokenHasExactScope(s, rawToken, desktopRemoteScopeTagPrefix+tag) {
			return true
		}
	}
	return false
}

func desktopRemoteTokenHasAdminOrGlobalScope(s *Server, rawToken string) bool {
	return desktopRemoteTokenHasExactScope(s, rawToken, desktopScopeAdmin) ||
		desktopRemoteTokenHasExactScope(s, rawToken, desktopRemoteScopeAll)
}

func desktopRemoteTokenHasExactScope(s *Server, rawToken, scope string) bool {
	if s == nil || s.TokenManager == nil || strings.TrimSpace(rawToken) == "" || strings.TrimSpace(scope) == "" {
		return false
	}
	_, ok := s.TokenManager.Validate(rawToken, scope)
	return ok
}

func desktopRemoteRateLimitKeys(action, ip, deviceID string) []string {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "desktop_remote"
	}
	keys := []string{loginScopeKey(action, strings.TrimSpace(ip))}
	if strings.TrimSpace(deviceID) != "" {
		keys = append(keys, loginScopeKey(action+"_device", strings.TrimSpace(ip)+"|"+strings.TrimSpace(deviceID)))
	}
	return keys
}

func recordDesktopRemoteRateFailure(s *Server, keys ...string) {
	maxAttempts := 5
	lockoutMinutes := 15
	if s != nil && s.Cfg != nil {
		s.CfgMu.RLock()
		if s.Cfg.Auth.MaxLoginAttempts > 0 {
			maxAttempts = s.Cfg.Auth.MaxLoginAttempts
		}
		if s.Cfg.Auth.LockoutMinutes > 0 {
			lockoutMinutes = s.Cfg.Auth.LockoutMinutes
		}
		s.CfgMu.RUnlock()
	}
	RecordFailedLoginForKeys(maxAttempts, lockoutMinutes, keys...)
}

func auditDesktopRemoteAttempt(s *Server, r *http.Request, action, deviceID, status, reason string) {
	if s == nil || s.DesktopService == nil || r == nil {
		return
	}
	details := map[string]interface{}{
		"status":      strings.TrimSpace(status),
		"reason":      strings.TrimSpace(reason),
		"method":      r.Method,
		"path":        r.URL.Path,
		"remote_addr": r.RemoteAddr,
	}
	_ = s.DesktopService.AuditWithRequest(r.Context(), action, strings.TrimSpace(deviceID), details, desktop.SourceUser, desktopAuditRequestInfo(s, r))
}

func desktopAuditRequestInfo(s *Server, r *http.Request) desktop.AuditRequestInfo {
	if r == nil {
		return desktop.AuditRequestInfo{}
	}
	behindProxy := false
	if s != nil && s.Cfg != nil {
		behindProxy = s.Cfg.Server.HTTPS.BehindProxy
	}
	return desktop.AuditRequestInfo{
		ClientIP:    ClientIP(r, behindProxy),
		SessionHash: desktopRequestSessionHash(r),
		UserAgent:   r.UserAgent(),
	}
}

func desktopRequestSessionHash(r *http.Request) string {
	if r == nil {
		return ""
	}
	material := ""
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		material = cookie.Value
	}
	if material == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			material = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
	}
	if material == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

func writeDesktopRemoteGuardError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
