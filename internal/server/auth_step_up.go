package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/i18n"
)

// stepUpResult is the outcome of re-verifying the owner's credentials before a
// sensitive auth change. Status 0 means the credentials were accepted.
type stepUpResult struct {
	Status     int
	Code       string
	MessageKey string
}

// adminLoginKeys returns the client IP and the IP/account rate-limit keys
// shared by /auth/login and every credential step-up, so failures in either
// place count toward the same lockout.
func adminLoginKeys(s *Server, r *http.Request) (ip, ipKey, accountKey string) {
	s.CfgMu.RLock()
	behindProxy := s.Cfg.Server.HTTPS.BehindProxy
	s.CfgMu.RUnlock()
	ip = ClientIP(r, behindProxy)
	return ip, loginScopeKey("ip", ip), loginScopeKey("account", "admin")
}

// verifyAdminCredentials re-checks the current password and, while TOTP is
// active, a current code. Wrong credentials are recorded against the login
// lockout; a missing credential is reported without counting as a guess.
func verifyAdminCredentials(s *Server, r *http.Request, currentPassword, currentTOTPCode string) stepUpResult {
	ip, ipKey, accountKey := adminLoginKeys(s, r)
	s.CfgMu.RLock()
	hash := s.Cfg.Auth.PasswordHash
	totpSecret := s.Cfg.Auth.TOTPSecret
	totpActive := s.Cfg.Auth.TOTPEnabled && totpSecret != ""
	maxAttempts := s.Cfg.Auth.MaxLoginAttempts
	lockoutMinutes := s.Cfg.Auth.LockoutMinutes
	s.CfgMu.RUnlock()

	locked := stepUpResult{Status: http.StatusTooManyRequests, Code: "too_many_attempts", MessageKey: "backend.auth_too_many_login_attempts"}
	if hash == "" {
		return stepUpResult{Status: http.StatusConflict, Code: "password_not_set", MessageKey: "backend.auth_not_configured"}
	}
	if IsLockedOutAny(ipKey, accountKey) {
		return locked
	}
	if currentPassword == "" {
		return stepUpResult{Status: http.StatusForbidden, Code: "current_password_required", MessageKey: "backend.auth_current_password_required"}
	}
	if totpActive && strings.TrimSpace(currentTOTPCode) == "" {
		return stepUpResult{Status: http.StatusForbidden, Code: "current_totp_required", MessageKey: "backend.auth_current_totp_required"}
	}
	if delay := LoginBackoffDelay(ipKey, accountKey); delay > 0 {
		time.Sleep(delay)
	}
	if !CheckPassword(currentPassword, hash) || (totpActive && !VerifyTOTP(totpSecret, currentTOTPCode)) {
		RecordFailedLoginForKeys(maxAttempts, lockoutMinutes, ipKey, accountKey)
		if s.Logger != nil {
			s.Logger.Warn("[Auth] Failed credential step-up", "ip", ip, "path", r.URL.Path)
		}
		if IsLockedOutAny(ipKey, accountKey) {
			return locked
		}
		return stepUpResult{Status: http.StatusUnauthorized, Code: "invalid_credentials", MessageKey: "backend.auth_invalid_credentials"}
	}
	return stepUpResult{}
}

// requireAdminStepUp writes the localized error and returns false when the
// step-up fails.
func requireAdminStepUp(s *Server, w http.ResponseWriter, r *http.Request, currentPassword, currentTOTPCode string) bool {
	result := verifyAdminCredentials(s, r, currentPassword, currentTOTPCode)
	if result.Status == 0 {
		return true
	}
	s.CfgMu.RLock()
	lang := s.Cfg.Server.UILanguage
	s.CfgMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": i18n.T(lang, result.MessageKey), "code": result.Code})
	return false
}
