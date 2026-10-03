package server

import (
	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/security"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ── Auth & Security Status ───────────────────────────────────────────────────

// handleAuthStatus returns whether auth is enabled, if a password is set, and TOTP state.
// This endpoint is always public (whitelisted in middleware).
func handleAuthStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		passwordSet := s.Cfg.Auth.PasswordHash != ""
		totpEnabled := s.Cfg.Auth.TOTPEnabled && s.Cfg.Auth.TOTPSecret != ""
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		var expiry time.Time
		if enabled && secret != "" {
			if cookie, err := r.Cookie(sessionCookieName); err == nil {
				expiry = sessionExpiry(secret, cookie.Value)
			}
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":            enabled,
			"password_set":       passwordSet,
			"totp_enabled":       totpEnabled,
			"authenticated":      !expiry.IsZero(),
			"expires_in_seconds": max(0, int(time.Until(expiry).Seconds())),
		})
	}
}

// ── Login / Logout ───────────────────────────────────────────────────────────

// handleAuthLoginPage serves the embedded login page template.
func handleAuthLoginPage(s *Server, uiFS fs.FS) http.HandlerFunc {
	var tmpl *template.Template
	t, err := template.ParseFS(uiFS, "login.html")
	if err == nil {
		tmpl = t
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		// If auth is not enabled, redirect to home
		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		totpEnabled := s.Cfg.Auth.TOTPEnabled && s.Cfg.Auth.TOTPSecret != ""
		passwordSet := s.Cfg.Auth.PasswordHash != ""
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		s.CfgMu.RUnlock()

		if !enabled {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		if !passwordSet {
			http.Redirect(w, r, "/setup", http.StatusTemporaryRedirect)
			return
		}

		// If already logged in, redirect
		s.CfgMu.RLock()
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if IsAuthenticated(r, secret) {
			redirect := sanitizeRedirectTarget(r.URL.Query().Get("redirect"))
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		}

		if tmpl == nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.login_template_unavailable"), http.StatusInternalServerError)
			return
		}
		data := uiTemplateData(lang)
		setTemplateDataJSON(data, map[string]any{
			"totpEnabled": totpEnabled,
			"redirectURL": r.URL.Query().Get("redirect"),
		})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			s.Logger.Error("[Auth] Failed to render login page", "error", err)
		}
	}
}

func sanitizeRedirectTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		return "/"
	}
	if strings.Contains(target, "\\") || strings.ContainsFunc(target, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) {
		return "/"
	}
	return target
}

// handleAuthLogin processes the login form POST.
func handleAuthLogin(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaErr != nil || mediaType != "application/json" {
			jsonError(w, "application/json required", http.StatusUnsupportedMediaType)
			return
		}
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !requestOriginMatches(r, origin) {
			jsonError(w, "Login origin rejected", http.StatusForbidden)
			return
		}
		if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" && site != "none" {
			jsonError(w, "Login origin rejected", http.StatusForbidden)
			return
		}
		if r.Header.Get("Origin") == "" && r.Header.Get("Sec-Fetch-Site") == "" &&
			(r.Header.Get("Sec-Fetch-Mode") != "" || r.Header.Get("Sec-Fetch-Dest") != "") {
			jsonError(w, "Login origin required", http.StatusForbidden)
			return
		}

		ip, ipKey, accountKey := adminLoginKeys(s, r)

		s.CfgMu.RLock()
		maxAttempts := s.Cfg.Auth.MaxLoginAttempts
		lockoutMinutes := s.Cfg.Auth.LockoutMinutes
		s.CfgMu.RUnlock()

		// Rate limit check
		if IsLockedOut(ipKey) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_too_many_login_attempts"),
			})
			return
		}

		// Parse body
		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_bad_request"), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req struct {
			Password string `json:"password"`
			TOTPCode string `json:"totp_code"`
			Redirect string `json:"redirect"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_json"), http.StatusBadRequest)
			return
		}

		releaseVerification, allowed := beginAdminVerification(r.Context(), ipKey, accountKey)
		if !allowed {
			http.Error(w, "Too many login attempts", http.StatusTooManyRequests)
			return
		}
		defer releaseVerification()

		s.CfgMu.RLock()
		hash := s.Cfg.Auth.PasswordHash
		totpEnabled := s.Cfg.Auth.TOTPEnabled
		totpSecret := s.Cfg.Auth.TOTPSecret
		secret := s.Cfg.Auth.SessionSecret
		timeoutHours := s.Cfg.Auth.SessionTimeoutHours
		s.CfgMu.RUnlock()

		// Validate password
		if hash == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":          i18n.T(s.Cfg.Server.UILanguage, "backend.auth_not_configured"),
				"redirect":       "/setup",
				"setup_required": true,
			})
			return
		}
		if !CheckPassword(req.Password, hash) {
			RecordFailedLoginForKeys(maxAttempts, lockoutMinutes, ipKey, accountKey)
			s.Logger.Warn("[Auth] Failed login attempt", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			if IsLockedOutAny(ipKey, accountKey) {
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_too_many_login_attempts")})
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_credentials")})
			return
		}

		// Validate TOTP if enabled
		if totpEnabled && totpSecret != "" {
			if !VerifyTOTP(totpSecret, req.TOTPCode) {
				RecordFailedLoginForKeys(maxAttempts, lockoutMinutes, ipKey, accountKey)
				s.Logger.Warn("[Auth] Failed TOTP attempt", "ip", ip)
				w.Header().Set("Content-Type", "application/json")
				if IsLockedOutAny(ipKey, accountKey) {
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_too_many_login_attempts")})
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_credentials")})
				return
			}
		}

		// Success — set session cookie
		ClearLoginRecords(ipKey, accountKey)
		timeout := time.Duration(timeoutHours) * time.Hour
		SetSessionCookie(w, r, secret, timeout)
		s.Logger.Info("[Auth] Successful login", "ip", ip)

		redirect := sanitizeRedirectTarget(req.Redirect)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"redirect": redirect,
		})
	}
}

// handleAuthLogout clears the session cookie and instructs the browser to
// purge its cache for this origin so the back button cannot reveal old pages.
func handleAuthLogout(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !checkCSRFOriginWithPolicy(r, true) {
			http.Error(w, "Invalid origin", http.StatusForbidden)
			return
		}
		if !revokeRequestSession(s, r) {
			http.Error(w, "Session revocation temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		ClearSessionCookie(w, r)
		// Discard cached page content so the back button cannot reveal old pages.
		// Only "cache" is cleared here — the cookie is already expired via Set-Cookie MaxAge:-1
		// above. Including "cookies" here can cause a race where the browser forwards the old
		// cookie on the redirect request before Clear-Site-Data takes effect, causing the login
		// page to see an authenticated session and redirect back to chat.
		w.Header().Set("Clear-Site-Data", `"cache"`)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Pragma", "no-cache")
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":       true,
				"redirect": "/auth/login",
			})
			return
		}
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
	}
}

// handleAuthLogoutAPI clears the session cookie without browser redirect logic.
// This is used by the Web UI on mobile/PWA to avoid brittle navigation behavior
// around redirects and Clear-Site-Data handling.
func handleAuthLogoutAPI(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		if !checkCSRFOriginWithPolicy(r, true) {
			http.Error(w, "Invalid origin", http.StatusForbidden)
			return
		}
		if !revokeRequestSession(s, r) {
			http.Error(w, "Session revocation temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		ClearSessionCookie(w, r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Pragma", "no-cache")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"redirect": "/auth/login",
		})
	}
}

// ── Password Management ──────────────────────────────────────────────────────

// handleAuthSetPassword sets or changes the login password.
// The first password (no hash yet) follows the setup bootstrap rules. Changing
// an existing password requires an authenticated browser session plus the
// current password and, while TOTP is active, a current code.
func handleAuthSetPassword(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		existingHash := s.Cfg.Auth.PasswordHash
		secret := s.Cfg.Auth.SessionSecret
		authEnabled := s.Cfg.Auth.Enabled
		s.CfgMu.RUnlock()

		// Authorization: allowed if first setup (no hash yet) or already authenticated.
		// When a password hash exists the user MUST be authenticated regardless of
		// whether auth is currently enabled — otherwise an attacker could pre-set a
		// password and silently take over when auth is re-enabled.
		firstSetup := existingHash == ""
		authed := IsAuthenticated(r, secret)
		if firstSetup {
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if !strings.EqualFold(mediaType, "application/json") || !checkCSRFOriginWithPolicy(r, true) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.setup_invalid_csrf_token")})
				return
			}
			// During the lockdown this request claims the instance, so it needs
			// the same bootstrap proof as the setup wizard. With auth disabled
			// every API is open anyway and the flow stays unchanged.
			if authEnabled && !authed && !authorizeSetupBootstrap(s, w, r) {
				return
			}
		}
		if !firstSetup && !authed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_unauthorized")})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_bad_request"), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req struct {
			NewPassword     string `json:"new_password"`
			CurrentPassword string `json:"current_password"`
			CurrentTOTPCode string `json:"current_totp_code"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_json"), http.StatusBadRequest)
			return
		}

		if len(req.NewPassword) < 8 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_password_min_length")})
			return
		}

		// Re-verify the owner before replacing an existing password so an open
		// session alone cannot lock the owner out.
		if !firstSetup && !requireAdminStepUp(s, w, r, req.CurrentPassword, req.CurrentTOTPCode) {
			return
		}

		newHash, err := HashPassword(req.NewPassword)
		if err != nil {
			s.Logger.Error("[Auth] Failed to hash password", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_internal_error"), http.StatusInternalServerError)
			return
		}

		// Always rotate session_secret on password change — this immediately invalidates
		// all existing sessions signed with the old secret.
		newSecret, err := GenerateRandomHex(32)
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_generate_secret"), http.StatusInternalServerError)
			return
		}

		// Patch config file under the config serialization lock.
		s.CfgSaveMu.Lock()
		patchErr := patchAuthConfig(s, map[string]interface{}{
			"password_hash":  newHash,
			"session_secret": newSecret,
		})
		s.CfgSaveMu.Unlock()
		if err := patchErr; err != nil {
			s.Logger.Error("[Auth] Failed to save password", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_save_config"), http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Auth] Password updated")
		if firstSetup {
			s.clearSetupBootstrapToken()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_password_set")})
	}
}

// ── TOTP Management ──────────────────────────────────────────────────────────

// handleAuthTOTPSetup generates a new TOTP secret and returns the otpauth URI.
// Does NOT activate it yet — user must confirm with a valid code first.
func handleAuthTOTPSetup(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		if !requireSession(s, w, r) {
			return
		}

		newSecret, err := GenerateTOTPSecret()
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_generate_totp_secret"), http.StatusInternalServerError)
			return
		}

		uri := TOTPAuthURI(newSecret, "AuraGo", "admin")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"secret": newSecret,
			"uri":    uri,
		})
	}
}

// handleAuthTOTPConfirm verifies the user's first TOTP code and activates 2FA.
// Enrolling or replacing a secret requires the current password and, when a
// secret is already active, a current code from it.
func handleAuthTOTPConfirm(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		if !requireSession(s, w, r) {
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_bad_request"), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req struct {
			Secret          string `json:"secret"`
			Code            string `json:"code"`
			CurrentPassword string `json:"current_password"`
			CurrentTOTPCode string `json:"current_totp_code"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.Secret == "" || req.Code == "" {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_request"), http.StatusBadRequest)
			return
		}

		if !VerifyTOTP(req.Secret, req.Code) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_code")})
			return
		}
		if !requireAdminStepUp(s, w, r, req.CurrentPassword, req.CurrentTOTPCode) {
			return
		}

		// Activate TOTP under the config serialization lock.
		s.CfgSaveMu.Lock()
		patchErr := patchAuthConfig(s, map[string]interface{}{
			"totp_secret":  req.Secret,
			"totp_enabled": true,
		})
		s.CfgSaveMu.Unlock()
		if err := patchErr; err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_save_totp_config"), http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Auth] TOTP activated")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": i18n.T(s.Cfg.Server.UILanguage, "backend.authenticator_activated")})
	}
}

// handleAuthTOTPDelete disables TOTP authentication. The JSON body must carry
// the current password and, while TOTP is active, a current code.
func handleAuthTOTPDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		if !requireSession(s, w, r) {
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_bad_request"), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		var req struct {
			CurrentPassword string `json:"current_password"`
			CurrentTOTPCode string `json:"current_totp_code"`
		}
		if strings.TrimSpace(string(body)) != "" {
			if err := json.Unmarshal(body, &req); err != nil {
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_json"), http.StatusBadRequest)
				return
			}
		}
		if !requireAdminStepUp(s, w, r, req.CurrentPassword, req.CurrentTOTPCode) {
			return
		}

		s.CfgSaveMu.Lock()
		patchErr := patchAuthConfig(s, map[string]interface{}{
			"totp_secret":  "",
			"totp_enabled": false,
		})
		s.CfgSaveMu.Unlock()
		if err := patchErr; err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_save_config"), http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Auth] TOTP disabled")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": i18n.T(s.Cfg.Server.UILanguage, "backend.authenticator_deactivated")})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// requireSession checks authentication for handlers inside the config UI.
// Returns false and writes 401 if not authenticated.
func requireSession(s *Server, w http.ResponseWriter, r *http.Request) bool {
	s.CfgMu.RLock()
	enabled := s.Cfg.Auth.Enabled
	secret := s.Cfg.Auth.SessionSecret
	s.CfgMu.RUnlock()

	if !enabled {
		return true // auth not active, allow all
	}
	if !IsAuthenticated(r, secret) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": i18n.T(s.Cfg.Server.UILanguage, "backend.auth_unauthorized")})
		return false
	}
	return true
}

// vaultAuthKeys maps auth config field names to their vault key names.
// These fields carry yaml:"-" and must be stored in the vault, not config.yaml.
var vaultAuthKeys = map[string]string{
	"password_hash":  "auth_password_hash",
	"session_secret": "auth_session_secret",
	"totp_secret":    "auth_totp_secret",
}

// patchAuthConfig persists auth fields and publishes a fresh config snapshot.
// Vault-only fields (password_hash, session_secret, totp_secret) are written to
// the encrypted vault in one batch; the remaining fields go into config.yaml.
//
// Callers must hold s.CfgSaveMu (the config serialization lock);
// handleSetupSave already does, so this function never locks it itself.
// config.yaml is written first, then the vault batch, then the candidate
// snapshot is loaded. Any failure restores the previous config.yaml bytes and
// vault values and leaves the live snapshot untouched.
func patchAuthConfig(s *Server, fields map[string]interface{}) error {
	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()
	if configPath == "" {
		return errors.New("config path not set")
	}

	// Split vault-only fields from regular YAML-persisted fields.
	vaultUpdates := map[string]string{}
	yamlFields := map[string]interface{}{}
	for k, v := range fields {
		if vaultKey, isVault := vaultAuthKeys[k]; isVault {
			str, ok := v.(string)
			if !ok {
				return fmt.Errorf("auth field %q must be a string", k)
			}
			vaultUpdates[vaultKey] = str
		} else {
			yamlFields[k] = v
		}
	}
	if len(vaultUpdates) > 0 && s.Vault == nil {
		return errors.New("vault unavailable: auth secrets cannot be stored")
	}

	// Stage config.yaml in memory before anything is written.
	var originalYAML, updatedYAML []byte
	if len(yamlFields) > 0 {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			return err
		}
		rawCfg = normalizeConfigYAMLMap(rawCfg)
		authSection, ok := rawCfg["auth"].(map[string]interface{})
		if !ok {
			authSection = make(map[string]interface{})
		}
		for k, v := range yamlFields {
			authSection[k] = v
		}
		rawCfg["auth"] = authSection
		rawCfg = normalizeConfigYAMLMap(rawCfg)
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			return err
		}
		originalYAML, updatedYAML = data, out
	}

	// Remember the current vault values so a later failure can restore them.
	previousVault := map[string]string{}
	var absentVault []string
	for vaultKey := range vaultUpdates {
		value, err := s.Vault.ReadSecret(vaultKey)
		switch {
		case err == nil:
			previousVault[vaultKey] = value
		case errors.Is(err, security.ErrSecretNotFound):
			absentVault = append(absentVault, vaultKey)
		default:
			return fmt.Errorf("reading %q from vault: %w", vaultKey, err)
		}
	}

	restoreYAML := func() {
		if originalYAML == nil {
			return
		}
		if err := config.WriteFileAtomic(configPath, originalYAML, 0o600); err != nil && s.Logger != nil {
			s.Logger.Error("[Auth] Failed to restore config.yaml after a failed auth update", "error", err)
		}
	}
	restoreVault := func() {
		if len(vaultUpdates) == 0 {
			return
		}
		if err := s.Vault.WriteSecrets(previousVault, absentVault); err != nil && s.Logger != nil {
			s.Logger.Error("[Auth] Failed to restore auth vault secrets after a failed auth update", "error", err)
		}
	}

	if updatedYAML != nil {
		if err := config.WriteFileAtomic(configPath, updatedYAML, 0o600); err != nil {
			return err
		}
	}
	if len(vaultUpdates) > 0 {
		if err := s.Vault.WriteSecrets(vaultUpdates, nil); err != nil {
			restoreYAML()
			return fmt.Errorf("writing auth secrets to vault: %w", err)
		}
	}

	// Load and normalize the candidate snapshot before publishing it.
	newCfg, err := config.Load(configPath)
	if err != nil {
		restoreVault()
		restoreYAML()
		return err
	}
	newCfg.ApplyVaultSecrets(s.Vault)
	newCfg.ResolveProviders()
	if s.Vault != nil {
		newCfg.ApplyOAuthTokens(s.Vault)
	}
	newCfg.ConfigPath = configPath

	s.CfgMu.Lock()
	newCfg.Runtime = s.Cfg.Runtime
	s.replaceConfigSnapshot(newCfg)
	s.CfgMu.Unlock()
	return nil
}

// handleSecurityStatus returns security configuration status (HTTPS, Auth, etc.)
// This endpoint is always public (whitelisted in middleware).
func handleSecurityStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		response := map[string]interface{}{
			"auth": map[string]interface{}{
				"enabled":      s.Cfg.Auth.Enabled,
				"password_set": s.Cfg.Auth.PasswordHash != "",
				"totp_enabled": s.Cfg.Auth.TOTPEnabled && s.Cfg.Auth.TOTPSecret != "",
			},
			"https": map[string]interface{}{
				"enabled": s.Cfg.Server.HTTPS.Enabled,
			},
			"connection": map[string]interface{}{
				"secure": IsSecureRequest(r),
			},
		}
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
