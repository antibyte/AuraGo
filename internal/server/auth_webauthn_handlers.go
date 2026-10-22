package server

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"time"

	"aurago/internal/config"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
)

//go:embed auth_webauthn_client.js
var webAuthnClientJS string

//go:embed auth_webauthn_locales.json
var webAuthnLocalesJSON []byte

var webAuthnLocales = func() map[string]map[string]string {
	var messages map[string]map[string]string
	if err := json.Unmarshal(webAuthnLocalesJSON, &messages); err != nil {
		panic("invalid built-in WebAuthn translations")
	}
	return messages
}()

type webAuthnRequest struct {
	Transaction      string                `json:"transaction"`
	Credential       json.RawMessage       `json:"credential"`
	Password         string                `json:"password"`
	CurrentPassword  string                `json:"current_password"`
	CurrentTOTPCode  string                `json:"current_totp_code"`
	Redirect         string                `json:"redirect"`
	Label            string                `json:"label"`
	Settings         config.WebAuthnConfig `json:"settings"`
	ResetCredentials bool                  `json:"reset_credentials"`
}

func webAuthnJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func webAuthnError(w http.ResponseWriter, code string, status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	message := code
	if code == "webauthn_required" {
		message = "WebAuthn required. Sign in in a browser or use an enrolled authenticator code."
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}

func (s *Server) webAuthnRequest(w http.ResponseWriter, r *http.Request, method string, owner bool) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != method {
		w.Header().Set("Allow", method)
		webAuthnError(w, "webauthn_bad_request", http.StatusMethodNotAllowed)
		return false
	}
	if r.Header.Get("Authorization") != "" {
		webAuthnError(w, "webauthn_session_required", http.StatusForbidden)
		return false
	}
	if method != http.MethodGet && !checkCSRFOriginWithPolicy(r, true) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		webAuthnError(w, "webauthn_origin_rejected", http.StatusForbidden)
		return false
	}
	if owner {
		a, _ := s.webAuthnAuthSnapshot()
		if !a.Enabled || a.PasswordHash == "" || a.SessionSecret == "" || !IsAuthenticated(r, a.SessionSecret) {
			webAuthnError(w, "webauthn_session_required", http.StatusUnauthorized)
			return false
		}
	}
	return true
}

// On success the caller owns CfgSaveMu; a concurrent credential change invalidates step-up.
func (s *Server) lockWebAuthnStepUp(w http.ResponseWriter, r *http.Request, req webAuthnRequest) bool {
	a, cfg := s.webAuthnAuthSnapshot()
	vault := s.Vault
	if !requireAdminStepUp(s, w, r, req.CurrentPassword, req.CurrentTOTPCode) {
		return false
	}
	s.CfgSaveMu.Lock()
	current, source := s.webAuthnAuthSnapshot()
	if source != cfg || current.fingerprint() != a.fingerprint() || s.Vault != vault || !IsAuthenticated(r, current.SessionSecret) || r.Context().Err() != nil {
		s.CfgSaveMu.Unlock()
		webAuthnError(w, "webauthn_invalid_ceremony", http.StatusConflict)
		return false
	}
	return true
}

func readWebAuthnRequest(w http.ResponseWriter, r *http.Request) (webAuthnRequest, bool) {
	var req webAuthnRequest
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		webAuthnError(w, "webauthn_bad_request", http.StatusUnsupportedMediaType)
		return req, false
	}
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		webAuthnError(w, "webauthn_bad_request", http.StatusBadRequest)
		return req, false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		webAuthnError(w, "webauthn_bad_request", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

func (s *Server) registerWebAuthnRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/webauthn/client.js", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, webAuthnClientJS)
		}
	})
	mux.HandleFunc("/auth/webauthn/login/begin", handleWebAuthnBegin(s, false))
	mux.HandleFunc("/auth/webauthn/login/finish", handleWebAuthnFinish(s, false))
	mux.HandleFunc("/api/auth/webauthn/register/begin", handleWebAuthnBegin(s, true))
	mux.HandleFunc("/api/auth/webauthn/register/finish", handleWebAuthnFinish(s, true))
	mux.HandleFunc("/api/auth/webauthn/credentials", handleWebAuthnCredentials(s))
	mux.HandleFunc("/api/auth/webauthn/credentials/", handleWebAuthnCredential(s))
	mux.HandleFunc("/api/auth/webauthn/settings", handleWebAuthnSettings(s))
}

func handleWebAuthnBegin(s *Server, registration bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.webAuthnRequest(w, r, http.MethodPost, registration) {
			return
		}
		req, ok := readWebAuthnRequest(w, r)
		if !ok {
			return
		}
		_, ipKey, accountKey := adminLoginKeys(s, r)
		if IsLockedOut(ipKey) || !s.allowWebAuthnStart(ipKey) {
			webAuthnError(w, "webauthn_rate_limited", http.StatusTooManyRequests)
			return
		}
		a, source := s.webAuthnAuthSnapshot()
		if registration {
			req.Label = strings.TrimSpace(req.Label)
			if !validWebAuthnLabel(req.Label) {
				webAuthnError(w, "webauthn_bad_request", http.StatusBadRequest)
				return
			}
			if !requireAdminStepUp(s, w, r, req.CurrentPassword, req.CurrentTOTPCode) {
				return
			}
		} else {
			if !a.Config.Enabled {
				webAuthnError(w, "webauthn_unavailable", http.StatusConflict)
				return
			}
			if a.Config.LoginMode == "second_factor" {
				release, allowed := beginAdminVerification(r.Context(), ipKey, accountKey)
				if !allowed {
					webAuthnError(w, "webauthn_rate_limited", http.StatusTooManyRequests)
					return
				}
				valid := a.PasswordHash != "" && CheckPassword(req.Password, a.PasswordHash)
				release()
				if !valid {
					RecordFailedLoginForKeys(a.MaxAttempts, a.LockoutMinutes, ipKey, accountKey)
					webAuthnError(w, "webauthn_invalid_credentials", http.StatusUnauthorized)
					return
				}
			}
		}
		client, err := webAuthnClient(a, r.Header.Get("Origin"))
		if err != nil {
			webAuthnError(w, "webauthn_origin_rejected", http.StatusForbidden)
			return
		}
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		current, cfg := s.webAuthnAuthSnapshot()
		if cfg != source || current.fingerprint() != a.fingerprint() {
			webAuthnError(w, "webauthn_invalid_ceremony", http.StatusConflict)
			return
		}
		state, err := s.loadWebAuthnState()
		if err != nil {
			webAuthnError(w, "webauthn_registry_unavailable", http.StatusServiceUnavailable)
			return
		}
		if state.RPID != "" && state.RPID != a.Config.RPID {
			webAuthnError(w, "webauthn_reset_required", http.StatusConflict)
			return
		}
		var options any
		var session *wa.SessionData
		tx := webAuthnTransaction{Origin: r.Header.Get("Origin"), Label: req.Label, Redirect: sanitizeRedirectTarget(req.Redirect), Config: source, Vault: s.Vault, AuthHash: a.fingerprint()}
		if registration {
			if len(state.Credentials) >= webAuthnMaxCredentials {
				webAuthnError(w, "webauthn_credential_limit", http.StatusConflict)
				return
			}
			if len(state.UserID) == 0 {
				state.UserID = randomWebAuthnBytes()
				state.RPID = a.Config.RPID
				if err := s.saveWebAuthnState(state); err != nil {
					webAuthnError(w, "webauthn_save_failed", http.StatusServiceUnavailable)
					return
				}
			}
			cookie, _ := r.Cookie(sessionCookieName)
			if cookie == nil || !IsAuthenticated(r, a.SessionSecret) {
				webAuthnError(w, "webauthn_session_required", http.StatusUnauthorized)
				return
			}
			tx.OwnerSession = sha256.Sum256([]byte(cookie.Value))
			tx.Operation = "register"
			options, session, err = client.BeginRegistration(state, wa.WithRegistrationOrigin(tx.Origin), wa.WithExclusions(wa.Credentials(state.WebAuthnCredentials()).CredentialDescriptors()))
		} else {
			if len(state.Credentials) == 0 {
				webAuthnError(w, "webauthn_credentials_required", http.StatusConflict)
				return
			}
			tx.Operation = "login"
			tx.FirstFactor = a.Config.LoginMode == "second_factor"
			options, session, err = client.BeginLogin(state, wa.WithUserVerification(protocol.VerificationRequired), wa.WithLoginOrigin(tx.Origin))
		}
		if err != nil {
			webAuthnError(w, "webauthn_unavailable", http.StatusServiceUnavailable)
			return
		}
		tx.Session = *session
		tx.RegistryHash = webAuthnRegistryHash(state)
		id, err := s.putWebAuthnTransaction(w, r, tx)
		if err != nil {
			webAuthnError(w, "webauthn_rate_limited", http.StatusTooManyRequests)
			return
		}
		webAuthnJSON(w, map[string]any{"options": options, "transaction": id})
	}
}

func handleWebAuthnFinish(s *Server, registration bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.webAuthnRequest(w, r, http.MethodPost, registration) {
			return
		}
		req, ok := readWebAuthnRequest(w, r)
		if !ok {
			return
		}
		_, ipKey, accountKey := adminLoginKeys(s, r)
		if IsLockedOut(ipKey) {
			webAuthnError(w, "webauthn_rate_limited", http.StatusTooManyRequests)
			return
		}
		release, allowed := beginAdminVerification(r.Context(), ipKey, accountKey)
		if !allowed {
			webAuthnError(w, "webauthn_rate_limited", http.StatusTooManyRequests)
			return
		}
		defer release()
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		a, cfg := s.webAuthnAuthSnapshot()
		operation := "login"
		if registration {
			operation = "register"
		}
		tx, ok := s.takeWebAuthnTransaction(r, req.Transaction, operation, a, cfg)
		if !ok {
			webAuthnError(w, "webauthn_invalid_ceremony", http.StatusUnauthorized)
			return
		}
		if !registration && (!a.Config.Enabled || a.Config.LoginMode == "second_factor" && !tx.FirstFactor) {
			webAuthnError(w, "webauthn_invalid_ceremony", http.StatusUnauthorized)
			return
		}
		client, err := webAuthnClient(a, tx.Origin)
		if err != nil {
			webAuthnError(w, "webauthn_origin_rejected", http.StatusForbidden)
			return
		}
		state, err := s.loadWebAuthnState()
		if err != nil {
			webAuthnError(w, "webauthn_registry_unavailable", http.StatusServiceUnavailable)
			return
		}
		// shortcut: registry writes invalidate concurrent ceremonies; use credential revisions if parallel logins become common.
		if webAuthnRegistryHash(state) != tx.RegistryHash {
			webAuthnError(w, "webauthn_invalid_ceremony", http.StatusConflict)
			return
		}
		var credential *wa.Credential
		if registration {
			parsed, parseErr := protocol.ParseCredentialCreationResponseBytes(req.Credential)
			if parseErr == nil {
				credential, err = client.CreateCredential(state, tx.Session, parsed)
			} else {
				err = parseErr
			}
		} else {
			parsed, parseErr := protocol.ParseCredentialRequestResponseBytes(req.Credential)
			if parseErr == nil {
				credential, err = client.ValidateLogin(state, tx.Session, parsed)
			} else {
				err = parseErr
			}
		}
		if err != nil || credential == nil || !credential.Flags.UserVerified || credential.Authenticator.CloneWarning {
			RecordFailedLoginForKeys(a.MaxAttempts, a.LockoutMinutes, ipKey, accountKey)
			if s.Logger != nil {
				s.Logger.Warn("[Auth] WebAuthn verification rejected")
			}
			webAuthnError(w, "webauthn_invalid_credentials", http.StatusUnauthorized)
			return
		}
		index := webAuthnCredentialIndex(state, webAuthnCredentialRef(credential.ID))
		if registration {
			if index >= 0 || len(state.Credentials) >= webAuthnMaxCredentials {
				webAuthnError(w, "webauthn_credential_limit", http.StatusConflict)
				return
			}
			state.Credentials = append(state.Credentials, webAuthnCredential{Credential: *credential, Label: tx.Label, CreatedAt: time.Now()})
		} else {
			if index < 0 {
				webAuthnError(w, "webauthn_invalid_credentials", http.StatusUnauthorized)
				return
			}
			state.Credentials[index].Credential = *credential
			state.Credentials[index].LastUsedAt = time.Now()
		}
		if r.Context().Err() != nil {
			webAuthnError(w, "webauthn_invalid_ceremony", http.StatusRequestTimeout)
			return
		}
		if err := s.saveWebAuthnState(state); err != nil {
			webAuthnError(w, "webauthn_save_failed", http.StatusServiceUnavailable)
			return
		}
		ClearLoginRecords(ipKey, accountKey)
		if !registration {
			SetSessionCookie(w, r, a.SessionSecret, time.Duration(a.TimeoutHours)*time.Hour)
		}
		webAuthnJSON(w, map[string]any{"ok": true, "redirect": tx.Redirect})
	}
}

func handleWebAuthnCredentials(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.webAuthnRequest(w, r, http.MethodGet, true) {
			return
		}
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		state, err := s.loadWebAuthnState()
		if err != nil {
			webAuthnError(w, "webauthn_registry_unavailable", http.StatusServiceUnavailable)
			return
		}
		credentials := make([]map[string]any, 0, len(state.Credentials))
		for _, c := range state.Credentials {
			credentials = append(credentials, map[string]any{"id": webAuthnCredentialRef(c.Credential.ID), "label": c.Label, "created_at": c.CreatedAt, "last_used_at": c.LastUsedAt})
		}
		a, _ := s.webAuthnAuthSnapshot()
		webAuthnJSON(w, map[string]any{"credentials": credentials, "settings": a.Config, "registry_rp_id": state.RPID})
	}
}

func handleWebAuthnCredential(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch && r.Method != http.MethodDelete {
			w.Header().Set("Allow", "PATCH, DELETE")
			webAuthnError(w, "webauthn_bad_request", http.StatusMethodNotAllowed)
			return
		}
		if !s.webAuthnRequest(w, r, r.Method, true) {
			return
		}
		req, ok := readWebAuthnRequest(w, r)
		if !ok {
			return
		}
		if !s.lockWebAuthnStepUp(w, r, req) {
			return
		}
		defer s.CfgSaveMu.Unlock()
		state, err := s.loadWebAuthnState()
		if err != nil {
			webAuthnError(w, "webauthn_registry_unavailable", http.StatusServiceUnavailable)
			return
		}
		index := webAuthnCredentialIndex(state, strings.TrimPrefix(r.URL.Path, "/api/auth/webauthn/credentials/"))
		if index < 0 {
			webAuthnError(w, "webauthn_bad_request", http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPatch {
			req.Label = strings.TrimSpace(req.Label)
			if !validWebAuthnLabel(req.Label) {
				webAuthnError(w, "webauthn_bad_request", http.StatusBadRequest)
				return
			}
			state.Credentials[index].Label = req.Label
		} else {
			a, _ := s.webAuthnAuthSnapshot()
			if a.Config.Enabled && a.Config.LoginMode == "second_factor" && !(a.TOTPEnabled && a.TOTPSecret != "") && len(state.Credentials) == 1 {
				webAuthnError(w, "webauthn_last_factor", http.StatusConflict)
				return
			}
			state.Credentials = append(state.Credentials[:index], state.Credentials[index+1:]...)
		}
		if err := s.saveWebAuthnState(state); err != nil {
			webAuthnError(w, "webauthn_save_failed", http.StatusServiceUnavailable)
			return
		}
		s.clearWebAuthnTransactions()
		webAuthnJSON(w, map[string]bool{"ok": true})
	}
}

func handleWebAuthnSettings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.webAuthnRequest(w, r, http.MethodPut, true) {
			return
		}
		req, ok := readWebAuthnRequest(w, r)
		if !ok {
			return
		}
		if err := config.ValidateWebAuthnConfig(req.Settings); err != nil {
			webAuthnError(w, "webauthn_invalid_settings", http.StatusBadRequest)
			return
		}
		if !s.lockWebAuthnStepUp(w, r, req) {
			return
		}
		defer s.CfgSaveMu.Unlock()
		a, _ := s.webAuthnAuthSnapshot()
		state, err := s.loadWebAuthnState()
		if err != nil {
			webAuthnError(w, "webauthn_registry_unavailable", http.StatusServiceUnavailable)
			return
		}
		fields := map[string]interface{}{"webauthn": req.Settings}
		if req.Settings.RPID == "" && state.RPID != "" {
			webAuthnError(w, "webauthn_invalid_settings", http.StatusBadRequest)
			return
		}
		if state.RPID != "" && state.RPID != req.Settings.RPID {
			if !req.ResetCredentials || req.Settings.Enabled || a.Config.Enabled {
				webAuthnError(w, "webauthn_reset_required", http.StatusConflict)
				return
			}
			fresh := webAuthnState{Version: 1, RPID: req.Settings.RPID, UserID: randomWebAuthnBytes()}
			raw, _ := json.Marshal(fresh)
			fields["webauthn_state"] = string(raw)
		}
		if req.Settings.Enabled && (len(state.Credentials) == 0 || req.Settings.RPID != state.RPID) {
			webAuthnError(w, "webauthn_credentials_required", http.StatusConflict)
			return
		}
		if err := patchAuthConfig(s, fields); err != nil {
			webAuthnError(w, "webauthn_save_failed", http.StatusServiceUnavailable)
			return
		}
		s.clearWebAuthnTransactions()
		webAuthnJSON(w, map[string]bool{"ok": true})
	}
}

func webAuthnPasswordLoginAllowed(a webAuthnAuth) bool {
	return !a.Config.Enabled || a.Config.LoginMode == "passkey" || a.Config.LoginMode == "second_factor" && a.TOTPEnabled && a.TOTPSecret != ""
}

// Generic config patches must not replace the protected WebAuthn policy or its factors.
func rejectWebAuthnConfigPatch(cfg *config.Config, patch map[string]interface{}) bool {
	if cfg == nil {
		return true
	}
	for key := range patch {
		if strings.HasPrefix(key, "auth.") {
			return true
		}
	}
	value, present := patch["auth"]
	if !present {
		return false
	}
	auth, ok := value.(map[string]interface{})
	if !ok {
		return cfg.Auth.WebAuthn.Enabled
	}
	if value, present := auth["webauthn"]; present {
		if value == nil {
			return true
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return true
		}
		candidate := cfg.Auth.WebAuthn
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&candidate) != nil || !reflect.DeepEqual(candidate, cfg.Auth.WebAuthn) {
			return true
		}
	}
	if _, present := auth["webauthn_state"]; present {
		return true
	}
	if !cfg.Auth.WebAuthn.Enabled {
		return false
	}
	for key, value := range auth {
		switch key {
		case "enabled":
			if !reflect.DeepEqual(value, cfg.Auth.Enabled) {
				return true
			}
		case "totp_enabled":
			if !reflect.DeepEqual(value, cfg.Auth.TOTPEnabled) {
				return true
			}
		case "password_hash", "admin_password", "totp_secret", "session_secret":
			return true
		}
	}
	return false
}
