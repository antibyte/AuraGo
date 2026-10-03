package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	stepUpTestPassword      = "old-password-1"
	stepUpTestSessionSecret = "step-up-session-secret"
)

func resetLoginRecordsForStepUpTest(t *testing.T) {
	t.Helper()
	reset := func() {
		loginMu.Lock()
		loginRecords = make(map[string]*loginRecord)
		loginMu.Unlock()
	}
	reset()
	t.Cleanup(reset)
}

// newStepUpTestServer returns a configured instance with an owner password and
// a valid browser session cookie.
func newStepUpTestServer(t *testing.T) (*Server, *http.Cookie) {
	t.Helper()
	resetLoginRecordsForStepUpTest(t)
	s := newBootstrapTestServer(t, "127.0.0.1")
	hash, err := HashPassword(stepUpTestPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := s.Vault.WriteSecret("auth_password_hash", hash); err != nil {
		t.Fatalf("WriteSecret(hash): %v", err)
	}
	if err := s.Vault.WriteSecret("auth_session_secret", stepUpTestSessionSecret); err != nil {
		t.Fatalf("WriteSecret(session): %v", err)
	}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = hash
	s.Cfg.Auth.SessionSecret = stepUpTestSessionSecret
	cookie := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(stepUpTestSessionSecret, time.Now().Add(time.Hour))}
	return s, cookie
}

func enableStepUpTestTOTP(t *testing.T, s *Server) string {
	t.Helper()
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if err := s.Vault.WriteSecret("auth_totp_secret", secret); err != nil {
		t.Fatalf("WriteSecret(totp): %v", err)
	}
	s.Cfg.Auth.TOTPEnabled = true
	s.Cfg.Auth.TOTPSecret = secret
	return secret
}

func currentStepUpTestCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totpCode(secret, uint64(time.Now().Unix()/30))
	if err != nil {
		t.Fatalf("totpCode: %v", err)
	}
	return code
}

func stepUpRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "198.51.100.20:43000"
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func TestAuthSetPasswordRequiresCurrentPassword(t *testing.T) {
	s, cookie := newStepUpTestServer(t)
	for _, tc := range []struct {
		name, body string
		wantStatus int
		wantCode   string
	}{
		{"missing current password", `{"new_password":"attacker-pass-1"}`, http.StatusForbidden, "current_password_required"},
		{"wrong current password", `{"new_password":"attacker-pass-1","current_password":"guess-123"}`, http.StatusUnauthorized, "invalid_credentials"},
	} {
		rec := httptest.NewRecorder()
		handleAuthSetPassword(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/password", tc.body, cookie))
		if rec.Code != tc.wantStatus || responseCode(t, rec) != tc.wantCode {
			t.Fatalf("%s: status = %d code = %q, want %d %q; body=%s", tc.name, rec.Code, responseCode(t, rec), tc.wantStatus, tc.wantCode, rec.Body.String())
		}
		stored, err := s.Vault.ReadSecret("auth_password_hash")
		if err != nil || !CheckPassword(stepUpTestPassword, stored) {
			t.Fatalf("%s: stored password changed", tc.name)
		}
	}

	rec := httptest.NewRecorder()
	body := `{"new_password":"new-password-1","current_password":"` + stepUpTestPassword + `"}`
	handleAuthSetPassword(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/password", body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("change with current password: status = %d body=%s", rec.Code, rec.Body.String())
	}
	stored, err := s.Vault.ReadSecret("auth_password_hash")
	if err != nil || !CheckPassword("new-password-1", stored) {
		t.Fatal("new password was not stored")
	}
}

func TestAuthSetPasswordRequiresCurrentTOTPWhenEnabled(t *testing.T) {
	s, cookie := newStepUpTestServer(t)
	secret := enableStepUpTestTOTP(t, s)

	rec := httptest.NewRecorder()
	body := `{"new_password":"new-password-1","current_password":"` + stepUpTestPassword + `"}`
	handleAuthSetPassword(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/password", body, cookie))
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "current_totp_required" {
		t.Fatalf("without TOTP code: status = %d code = %q", rec.Code, responseCode(t, rec))
	}

	rec = httptest.NewRecorder()
	body = `{"new_password":"new-password-1","current_password":"` + stepUpTestPassword + `","current_totp_code":"` + currentStepUpTestCode(t, secret) + `"}`
	handleAuthSetPassword(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/password", body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("with TOTP code: status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthTOTPDeleteRequiresPasswordAndCode(t *testing.T) {
	s, cookie := newStepUpTestServer(t)
	secret := enableStepUpTestTOTP(t, s)

	for _, body := range []string{
		"",
		`{"current_password":"` + stepUpTestPassword + `"}`,
		`{"current_password":"wrong-pass","current_totp_code":"000000"}`,
	} {
		rec := httptest.NewRecorder()
		handleAuthTOTPDelete(s).ServeHTTP(rec, stepUpRequest(http.MethodDelete, "/api/auth/totp", body, cookie))
		if rec.Code == http.StatusOK {
			t.Fatalf("TOTP disabled without full step-up (body %q)", body)
		}
		if got, _ := s.Vault.ReadSecret("auth_totp_secret"); got != secret {
			t.Fatalf("TOTP secret changed by rejected request (body %q)", body)
		}
	}

	rec := httptest.NewRecorder()
	body := `{"current_password":"` + stepUpTestPassword + `","current_totp_code":"` + currentStepUpTestCode(t, secret) + `"}`
	handleAuthTOTPDelete(s).ServeHTTP(rec, stepUpRequest(http.MethodDelete, "/api/auth/totp", body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("TOTP delete with step-up: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if s.Cfg.Auth.TOTPEnabled {
		t.Fatal("TOTP still enabled after successful delete")
	}
}

func TestAuthTOTPConfirmRequiresStepUp(t *testing.T) {
	s, cookie := newStepUpTestServer(t)
	first, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	rec := httptest.NewRecorder()
	body := `{"secret":"` + first + `","code":"` + currentStepUpTestCode(t, first) + `"}`
	handleAuthTOTPConfirm(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/totp/confirm", body, cookie))
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "current_password_required" {
		t.Fatalf("enroll without password: status = %d code = %q", rec.Code, responseCode(t, rec))
	}

	rec = httptest.NewRecorder()
	body = `{"secret":"` + first + `","code":"` + currentStepUpTestCode(t, first) + `","current_password":"` + stepUpTestPassword + `"}`
	handleAuthTOTPConfirm(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/totp/confirm", body, cookie))
	if rec.Code != http.StatusOK || s.Cfg.Auth.TOTPSecret != first {
		t.Fatalf("enroll with password: status = %d secret active = %v", rec.Code, s.Cfg.Auth.TOTPSecret == first)
	}

	replacement, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	rec = httptest.NewRecorder()
	body = `{"secret":"` + replacement + `","code":"` + currentStepUpTestCode(t, replacement) + `","current_password":"` + stepUpTestPassword + `"}`
	handleAuthTOTPConfirm(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/totp/confirm", body, cookie))
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "current_totp_required" {
		t.Fatalf("replace without current code: status = %d code = %q", rec.Code, responseCode(t, rec))
	}

	rec = httptest.NewRecorder()
	body = `{"secret":"` + replacement + `","code":"` + currentStepUpTestCode(t, replacement) + `","current_password":"` + stepUpTestPassword + `","current_totp_code":"` + currentStepUpTestCode(t, first) + `"}`
	handleAuthTOTPConfirm(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/totp/confirm", body, cookie))
	if rec.Code != http.StatusOK || s.Cfg.Auth.TOTPSecret != replacement {
		t.Fatalf("replace with current code: status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthStepUpFailuresDoNotLockCorrectLoginFromAnotherIP(t *testing.T) {
	s, cookie := newStepUpTestServer(t)
	s.Cfg.Auth.MaxLoginAttempts = 2
	s.Cfg.Auth.LockoutMinutes = 15

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		body := `{"new_password":"attacker-pass-1","current_password":"guess-` + strconv.Itoa(i) + `"}`
		handleAuthSetPassword(s).ServeHTTP(rec, stepUpRequest(http.MethodPost, "/api/auth/password", body, cookie))
		if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("wrong step-up %d: status = %d body=%s", i, rec.Code, rec.Body.String())
		}
	}

	login := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"password":"`+stepUpTestPassword+`"}`))
	login.RemoteAddr = "203.0.113.99:5000" // account backoff must allow correct credentials from another IP
	login.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleAuthLogin(s).ServeHTTP(rec, login)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct login from another IP: status = %d, want 200", rec.Code)
	}
}
