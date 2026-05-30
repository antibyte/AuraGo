package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/desktop"
	"aurago/internal/inventory"
)

func withDesktopRemoteGuard(s *Server, action, expectedProtocol string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
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
		}

		auditDesktopRemoteAttempt(s, r, action, deviceID, "attempt", "")
		next(w, r)
	}
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
		"client_ip":   ClientIP(r, s.Cfg != nil && s.Cfg.Server.HTTPS.BehindProxy),
		"user_agent":  r.UserAgent(),
		"method":      r.Method,
		"path":        r.URL.Path,
		"remote_addr": r.RemoteAddr,
	}
	_ = s.DesktopService.Audit(r.Context(), action, strings.TrimSpace(deviceID), details, desktop.SourceUser)
}

func writeDesktopRemoteGuardError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
