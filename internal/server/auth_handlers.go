package server

import (
	"aurago/internal/config"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		passwordSet := s.Cfg.Auth.PasswordHash != ""
		totpEnabled := s.Cfg.Auth.TOTPEnabled && s.Cfg.Auth.TOTPSecret != ""
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		authenticated := false
		if enabled && secret != "" {
			authenticated = IsAuthenticated(r, secret)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":       enabled,
			"password_set":  passwordSet,
			"totp_enabled":  totpEnabled,
			"authenticated": authenticated,
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
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		s.CfgMu.RUnlock()

		if !enabled {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		// If already logged in, redirect
		s.CfgMu.RLock()
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if IsAuthenticated(r, secret) {
			redirect := r.URL.Query().Get("redirect")
			if redirect == "" {
				redirect = "/"
			}
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		}

		if tmpl == nil {
			http.Error(w, "Login template not available", http.StatusInternalServerError)
			return
		}
		data := map[string]interface{}{
			"Lang":        lang,
			"I18N":        getI18NJSON(lang),
			"TOTPEnabled": totpEnabled,
			"Redirect":    r.URL.Query().Get("redirect"),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			s.Logger.Error("[Auth] Failed to render login page", "error", err)
		}
	}
}

// handleAuthLogin processes the login form POST.
func handleAuthLogin(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ip := ClientIP(r)

		s.CfgMu.RLock()
		maxAttempts := s.Cfg.Auth.MaxLoginAttempts
		lockoutMinutes := s.Cfg.Auth.LockoutMinutes
		s.CfgMu.RUnlock()

		// Rate limit check
		if IsLockedOut(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Zu viele Versuche. Bitte warte einige Minuten.",
			})
			return
		}

		// Parse body
		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req struct {
			Password string `json:"password"`
			TOTPCode string `json:"totp_code"`
			Redirect string `json:"redirect"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

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
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Kein Passwort gesetzt."})
			return
		}
		if !CheckPassword(req.Password, hash) {
			RecordFailedLogin(ip, maxAttempts, lockoutMinutes)
			s.Logger.Warn("[Auth] Failed login attempt", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Falsches Passwort."})
			return
		}

		// Validate TOTP if enabled
		if totpEnabled && totpSecret != "" {
			if !VerifyTOTP(totpSecret, req.TOTPCode) {
				RecordFailedLogin(ip, maxAttempts, lockoutMinutes)
				s.Logger.Warn("[Auth] Failed TOTP attempt", "ip", ip)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Ungültiger Authenticator-Code."})
				return
			}
		}

		// Success — set session cookie
		ClearLoginRecord(ip)
		timeout := time.Duration(timeoutHours) * time.Hour
		SetSessionCookie(w, r, secret, timeout)
		s.Logger.Info("[Auth] Successful login", "ip", ip)

		redirect := req.Redirect
		if redirect == "" {
			redirect = "/"
		}
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

// ── Password Management ──────────────────────────────────────────────────────

// handleAuthSetPassword sets or changes the login password.
// Accessible if no password is set yet (first-time) OR the user is authenticated.
func handleAuthSetPassword(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		existingHash := s.Cfg.Auth.PasswordHash
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()

		// Authorization: allowed if first setup (no hash yet) or already authenticated
		firstSetup := existingHash == ""
		authed := IsAuthenticated(r, secret)
		if enabled && !firstSetup && !authed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "unauthorized"})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req struct {
			NewPassword string `json:"new_password"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if len(req.NewPassword) < 8 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Passwort muss mindestens 8 Zeichen haben."})
			return
		}

		newHash, err := HashPassword(req.NewPassword)
		if err != nil {
			s.Logger.Error("[Auth] Failed to hash password", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Always rotate session_secret on password change — this immediately invalidates
		// all existing sessions signed with the old secret.
		newSecret, err := GenerateRandomHex(32)
		if err != nil {
			http.Error(w, "Failed to generate secret", http.StatusInternalServerError)
			return
		}

		// Patch config file
		if err := patchAuthConfig(s, map[string]interface{}{
			"password_hash":  newHash,
			"session_secret": newSecret,
		}); err != nil {
			s.Logger.Error("[Auth] Failed to save password", "error", err)
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Auth] Password updated")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Passwort gesetzt."})
	}
}

// ── TOTP Management ──────────────────────────────────────────────────────────

// handleAuthTOTPSetup generates a new TOTP secret and returns the otpauth URI.
// Does NOT activate it yet — user must confirm with a valid code first.
func handleAuthTOTPSetup(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireSession(s, w, r) {
			return
		}

		newSecret, err := GenerateTOTPSecret()
		if err != nil {
			http.Error(w, "Failed to generate TOTP secret", http.StatusInternalServerError)
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
func handleAuthTOTPConfirm(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireSession(s, w, r) {
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req struct {
			Secret string `json:"secret"`
			Code   string `json:"code"`
		}
		if err := json.Unmarshal(body, &req); err != nil || req.Secret == "" || req.Code == "" {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if !VerifyTOTP(req.Secret, req.Code) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Ungültiger Code. Bitte erneut versuchen."})
			return
		}

		// Activate TOTP
		if err := patchAuthConfig(s, map[string]interface{}{
			"totp_secret":  req.Secret,
			"totp_enabled": true,
		}); err != nil {
			http.Error(w, "Failed to save TOTP config", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Auth] TOTP activated")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Authenticator aktiviert."})
	}
}

// handleAuthTOTPDelete disables TOTP authentication.
func handleAuthTOTPDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireSession(s, w, r) {
			return
		}

		if err := patchAuthConfig(s, map[string]interface{}{
			"totp_secret":  "",
			"totp_enabled": false,
		}); err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Auth] TOTP disabled")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Authenticator deaktiviert."})
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
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "unauthorized"})
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

// patchAuthConfig writes the given key-value pairs for the "auth" config section.
// Vault-only fields (password_hash, session_secret, totp_secret) are stored in the
// encrypted vault; remaining fields go into config.yaml. Both paths hot-reload the
// running config so the change takes effect immediately without a restart.
func patchAuthConfig(s *Server, fields map[string]interface{}) error {
	configPath := s.Cfg.ConfigPath

	// Split vault-only fields from regular YAML-persisted fields.
	vaultUpdates := map[string]string{}
	yamlFields := map[string]interface{}{}
	for k, v := range fields {
		if vaultKey, isVault := vaultAuthKeys[k]; isVault {
			if str, ok := v.(string); ok {
				vaultUpdates[vaultKey] = str
			}
		} else {
			yamlFields[k] = v
		}
	}

	// Write sensitive fields to the vault.
	if s.Vault != nil {
		for vaultKey, val := range vaultUpdates {
			if err := s.Vault.WriteSecret(vaultKey, val); err != nil {
				return fmt.Errorf("writing %q to vault: %w", vaultKey, err)
			}
		}
	}

	// Write non-vault fields to config.yaml (skip file I/O when there is nothing to persist).
	if len(yamlFields) > 0 {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			return err
		}
		authSection, ok := rawCfg["auth"].(map[string]interface{})
		if !ok {
			authSection = make(map[string]interface{})
		}
		for k, v := range yamlFields {
			authSection[k] = v
		}
		rawCfg["auth"] = authSection
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(configPath, out, 0644); err != nil {
			return err
		}
	}

	// Hot-reload: re-read config.yaml and re-apply vault secrets so that all
	// vault-only fields (including the ones just written above) are populated.
	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr == nil {
		newCfg.ApplyVaultSecrets(s.Vault)
		savedPath := s.Cfg.ConfigPath
		*s.Cfg = *newCfg
		s.Cfg.ConfigPath = savedPath
	}
	s.CfgMu.Unlock()
	return loadErr
}

// handleSecurityStatus returns security configuration status (HTTPS, Auth, etc.)
// This endpoint is always public (whitelisted in middleware).
func handleSecurityStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
				"enabled":      s.Cfg.Server.HTTPS.Enabled,
				"domain":       s.Cfg.Server.HTTPS.Domain,
				"email":        s.Cfg.Server.HTTPS.Email,
				"https_port":   s.Cfg.Server.HTTPS.HTTPSPort,
				"http_port":    s.Cfg.Server.HTTPS.HTTPPort,
				"behind_proxy": s.Cfg.Server.HTTPS.BehindProxy,
			},
			"connection": map[string]interface{}{
				"secure": IsSecureRequest(r),
				"proto":  r.Header.Get("X-Forwarded-Proto"),
			},
		}
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
