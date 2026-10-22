package server

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/vaultprompt"
)

func TestWebAuthnTrustedProxyAndCookieBoundary(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "passkey")
	s.Cfg.Auth.WebAuthn.RPID = "aurago.example.test"
	s.Cfg.Auth.WebAuthn.Origins = []string{"https://aurago.example.test"}
	s.Cfg.Server.HTTPS.BehindProxy = true
	s.Cfg.Server.HTTPS.TrustedProxyCIDRs = []string{"192.0.2.10/32"}
	for _, kind := range []string{"direct TLS", "trusted proxy", "forged proxy"} {
		t.Run(kind, func(t *testing.T) {
			url := "http://localhost/auth/webauthn/login/begin"
			if kind == "direct TLS" {
				url = "https://aurago.example.test/auth/webauthn/login/begin"
			}
			req := httptest.NewRequest("POST", url, nil)
			req.RemoteAddr = "192.0.2.10:8123"
			if kind == "forged proxy" {
				req.RemoteAddr = "198.51.100.24:8123"
			}
			req.Header.Set("Origin", "https://aurago.example.test")
			req.Header.Set("X-Forwarded-Host", "aurago.example.test")
			req.Header.Set("X-Forwarded-Proto", "https")
			rec := httptest.NewRecorder()
			trustedProxyMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !s.webAuthnRequest(w, r, "POST", false) {
					return
				}
				a, _ := s.webAuthnAuthSnapshot()
				if _, err := webAuthnClient(a, r.Header.Get("Origin")); err != nil {
					t.Fatal(err)
				}
				if _, err := s.putWebAuthnTransaction(w, r, webAuthnTransaction{}); err != nil {
					t.Fatal(err)
				}
			})).ServeHTTP(rec, req)
			if kind == "forged proxy" {
				if rec.Code != 403 || len(rec.Result().Cookies()) != 0 {
					t.Fatal("forged proxy admitted ceremony")
				}
				return
			}
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
				t.Fatal("insecure browser binding cookie")
			}
		})
	}
}

func TestWebAuthnEnrollmentLimitsAndExplicitRPReset(t *testing.T) {
	s, owner := newWebAuthnTestServer(t, "passkey")
	_, _, state := seedWebAuthnTestCredential(t, s)
	for len(state.Credentials) < webAuthnMaxCredentials {
		_, c := webAuthnTestKey(t)
		state.Credentials = append(state.Credentials, webAuthnCredential{Credential: c, Label: "Additional key"})
	}
	if err := s.saveWebAuthnState(state); err != nil {
		t.Fatal(err)
	}
	rec := webAuthnTestRequest(t, handleWebAuthnBegin(s, true), "POST", "/api/auth/webauthn/register/begin", map[string]string{"label": "Too many", "current_password": stepUpTestPassword}, owner)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "webauthn_credential_limit") {
		t.Fatalf("enrollment limit: %d %s", rec.Code, rec.Body.String())
	}
	oldID := append([]byte(nil), state.UserID...)
	state.Credentials = append(state.Credentials[:1], state.Credentials[0])
	if err := s.saveWebAuthnState(state); err != nil {
		t.Fatal(err)
	}
	if _, err := s.loadWebAuthnState(); err == nil {
		t.Fatal("duplicate registry accepted")
	}
	state.Credentials = state.Credentials[:1]
	if err := s.saveWebAuthnState(state); err != nil {
		t.Fatal(err)
	}
	newSettings := config.WebAuthnConfig{LoginMode: "passkey", RPID: "new.example.test", Origins: []string{"https://new.example.test"}}
	request := map[string]any{"settings": newSettings, "reset_credentials": true, "current_password": stepUpTestPassword}
	rec = webAuthnTestRequest(t, handleWebAuthnSettings(s), "PUT", "/api/auth/webauthn/settings", request, owner)
	if rec.Code != 409 {
		t.Fatal("active RP reset accepted")
	}
	s.Cfg.Auth.WebAuthn.Enabled = false
	request["reset_credentials"] = false
	rec = webAuthnTestRequest(t, handleWebAuthnSettings(s), "PUT", "/api/auth/webauthn/settings", request, owner)
	if rec.Code != 409 {
		t.Fatal("implicit RP reset accepted")
	}
	request["reset_credentials"] = true
	rec = webAuthnTestRequest(t, handleWebAuthnSettings(s), "PUT", "/api/auth/webauthn/settings", request, owner)
	if rec.Code != 200 {
		t.Fatalf("explicit reset: %d %s", rec.Code, rec.Body.String())
	}
	reset, err := s.loadWebAuthnState()
	if err != nil || reset.RPID != newSettings.RPID || len(reset.Credentials) != 0 || bytes.Equal(reset.UserID, oldID) {
		t.Fatal("RP reset retained old identity/keys", err)
	}
}

func TestWebAuthnBackupPublicationInvalidatesPendingLogin(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "passkey")
	key, credential, state := seedWebAuthnTestCredential(t, s)
	id, challenge, browser := webAuthnTestBegin(t, s, false, nil, "")
	archive := &bytes.Buffer{}
	zw := zip.NewWriter(archive)
	entry, err := zw.Create("data/webauthn-backup-fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{"restored":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	file, err := form.CreateFormFile("file", "fixture.ago")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(archive.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/backup/import", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	handleBackupImport(s)(rec, req)
	if rec.Code != 200 {
		t.Fatalf("backup import: %d %s", rec.Code, rec.Body.String())
	}
	assertion := webAuthnTestCredential(t, key, credential, challenge, state.UserID, false, true)
	rec = webAuthnTestRequest(t, handleWebAuthnFinish(s, false), "POST", "/auth/webauthn/login/finish", map[string]any{"transaction": id, "credential": assertion}, browser)
	if rec.Code != 401 || rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("backup publication retained pending login")
	}
}

func TestWebAuthnVaultRestartAndBackup(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "passkey")
	_, _, state := seedWebAuthnTestCredential(t, s)
	state.Credentials[0].Credential.Authenticator.SignCount = 42
	state.Credentials[0].Credential.Flags.BackupEligible = true
	state.Credentials[0].Credential.Flags.BackupState = true
	if err := s.saveWebAuthnState(state); err != nil {
		t.Fatal(err)
	}
	want := webAuthnRegistryHash(state)
	var err error
	s.Vault, err = security.NewVault(strings.Repeat("c", 64), filepath.Join(filepath.Dir(s.Cfg.ConfigPath), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.loadWebAuthnState()
	if err != nil || webAuthnRegistryHash(reloaded) != want {
		t.Fatal("restart lost credential state", err)
	}
	blob, err := exportVaultSecrets(s.Vault, "backup-password")
	if err != nil {
		t.Fatal(err)
	}
	s.Vault, err = security.NewVault(strings.Repeat("d", 64), filepath.Join(t.TempDir(), "restored.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importVaultSecrets(s.Vault, blob, "backup-password"); err != nil {
		t.Fatal(err)
	}
	restored, err := s.loadWebAuthnState()
	if err != nil || webAuthnRegistryHash(restored) != want {
		t.Fatal("backup lost credential state", err)
	}
	if _, err := s.Vault.ReadSecretForAgent(webAuthnVaultKey); err == nil {
		t.Fatal("backup granted agent access")
	}
	if tools.IsPythonAccessibleSecret(webAuthnVaultKey) {
		t.Fatal("registry exposed to Python")
	}
	if _, err := vaultprompt.NormalizeVaultKey(webAuthnVaultKey); err == nil {
		t.Fatal("generic secret prompt admitted auth registry")
	}
}

func TestWebAuthnVaultWriteFailureDoesNotIssueSession(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows read-only replacement contract")
	}
	s, _ := newWebAuthnTestServer(t, "passkey")
	key, credential, state := seedWebAuthnTestCredential(t, s)
	id, challenge, browser := webAuthnTestBegin(t, s, false, nil, "")
	path := filepath.Join(filepath.Dir(s.Cfg.ConfigPath), "vault.bin")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	assertion := webAuthnTestCredential(t, key, credential, challenge, state.UserID, false, true)
	rec := webAuthnTestRequest(t, handleWebAuthnFinish(s, false), "POST", "/auth/webauthn/login/finish", map[string]any{"transaction": id, "credential": assertion}, browser)
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "webauthn_save_failed") || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("failed write published login: %d %s", rec.Code, rec.Body.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed write damaged persisted Vault", err)
	}
	if _, exists := s.webAuthn.transactions[id]; exists {
		t.Fatal("failed finish can be replayed")
	}
}

func TestWebAuthnBoundariesAndLimits(t *testing.T) {
	s, owner := newWebAuthnTestServer(t, "second_factor")
	seedWebAuthnTestCredential(t, s)
	for _, path := range []string{"/auth/webauthn/login/begin", "/auth/webauthn/login/finish", "/auth/webauthn/client.js"} {
		if !isAuthBypassed(path) || isAuthBypassed(path+"/extra") || noPasswordExactPaths[path] {
			t.Fatal("incorrect public auth boundary", path)
		}
	}
	for _, path := range []string{"/auth/webauthn/unknown", "/api/auth/webauthn/credentials", "/api/auth/webauthn/settings"} {
		if isAuthBypassed(path) {
			t.Fatal("unexpected auth exemption", path)
		}
	}
	a, _ := s.webAuthnAuthSnapshot()
	for _, origin := range []string{"", "http://localhost/", "http://user@localhost", "http://localhost#", "http://evil.test", "http://localhost:8443"} {
		if _, err := webAuthnClient(a, origin); err == nil {
			t.Fatal("invalid request origin accepted", origin)
		}
	}
	for _, kind := range []string{"origin missing", "origin foreign", "bearer", "unknown field", "oversize", "GET"} {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/auth/webauthn/register/begin", strings.NewReader(`{"label":"key","current_password":"old-password-1"}`))
		req.AddCookie(owner)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost")
		switch kind {
		case "origin missing":
			req.Header.Del("Origin")
		case "origin foreign":
			req.Header.Set("Origin", "https://evil.test")
		case "bearer":
			req.Header.Set("Authorization", "Bearer not-an-owner-session")
		case "unknown field":
			req.Body = httptest.NewRequest("POST", "/", strings.NewReader(`{"mode":"passkey"}`)).Body
		case "oversize":
			req.Body = httptest.NewRequest("POST", "/", strings.NewReader(`{"label":"`+strings.Repeat("a", 129<<10)+`"}`)).Body
		case "GET":
			req.Method = http.MethodGet
		}
		rec := httptest.NewRecorder()
		handleWebAuthnBegin(s, true)(rec, req)
		if rec.Code == 200 || rec.Header().Get("Set-Cookie") != "" {
			t.Fatal("boundary admitted", kind)
		}
	}
	for i := 0; i < 21; i++ {
		if s.allowWebAuthnStart("budget-test") != (i < 20) {
			t.Fatal("begin budget ignored")
		}
	}
	req := httptest.NewRequest("POST", "http://localhost/", nil)
	req.AddCookie(&http.Cookie{Name: webAuthnCookie, Value: base64.RawURLEncoding.EncodeToString(randomWebAuthnBytes())})
	for i := 0; i < 5; i++ {
		_, err := s.putWebAuthnTransaction(httptest.NewRecorder(), req, webAuthnTransaction{})
		if (err == nil) != (i < 4) {
			t.Fatal("per-browser ceremony limit ignored")
		}
	}
	s.Cfg.Auth.PasswordHash = ""
	called := false
	middleware := authMiddleware(s, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	for _, path := range []string{"/auth/webauthn/login/begin", "/auth/webauthn/login/finish"} {
		middleware.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "http://localhost"+path, nil))
	}
	if called {
		t.Fatal("WebAuthn escaped missing-password lockdown")
	}
}

func TestWebAuthnConfigAndLastFactorGuards(t *testing.T) {
	s, owner := newWebAuthnTestServer(t, "second_factor")
	s.Cfg.Auth.WebAuthn.Enabled = true
	patch := map[string]interface{}{"auth": map[string]interface{}{"enabled": false}}
	rec := webAuthnTestRequest(t, handleUpdateConfig(s), "PUT", "/api/config", patch, owner)
	if rec.Code != 403 {
		t.Fatalf("generic policy bypass: %d", rec.Code)
	}
	if _, err := applyConfigPatch(s, patch); err == nil {
		t.Fatal("setup patch bypassed policy")
	}
	secret := enableStepUpTestTOTP(t, s)
	stepUp := map[string]string{"current_password": stepUpTestPassword, "current_totp_code": currentStepUpTestCode(t, secret)}
	rec = webAuthnTestRequest(t, handleAuthTOTPDelete(s), "DELETE", "/api/auth/totp", stepUp, owner)
	if rec.Code != 409 || !s.Cfg.Auth.TOTPEnabled {
		t.Fatal("last second factor removed")
	}
	seedWebAuthnTestCredential(t, s)
	rec = webAuthnTestRequest(t, handleAuthTOTPDelete(s), "DELETE", "/api/auth/totp", stepUp, owner)
	if rec.Code != 200 || s.Cfg.Auth.TOTPEnabled {
		t.Fatalf("alternative key did not permit TOTP removal: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebAuthnConcurrentCeremonyAndCounterRegression(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "passkey")
	key, credential, state := seedWebAuthnTestCredential(t, s)
	first, challenge, browser := webAuthnTestBegin(t, s, false, nil, "")
	second, otherChallenge, otherBrowser := webAuthnTestBegin(t, s, false, nil, "")
	assertion := webAuthnTestCredential(t, key, credential, challenge, state.UserID, false, true)
	rec := webAuthnTestRequest(t, handleWebAuthnFinish(s, false), "POST", "/auth/webauthn/login/finish", map[string]any{"transaction": first, "credential": assertion}, browser)
	if rec.Code != 200 {
		t.Fatalf("first login: %d %s", rec.Code, rec.Body.String())
	}
	assertion = webAuthnTestCredential(t, key, credential, otherChallenge, state.UserID, false, true)
	rec = webAuthnTestRequest(t, handleWebAuthnFinish(s, false), "POST", "/auth/webauthn/login/finish", map[string]any{"transaction": second, "credential": assertion}, otherBrowser)
	if rec.Code != 409 || rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("stale registry result accepted")
	}
	state.Credentials[0].Credential.Authenticator.SignCount = 5
	if err := s.saveWebAuthnState(state); err != nil {
		t.Fatal(err)
	}
	id, challenge, browser := webAuthnTestBegin(t, s, false, nil, "")
	assertion = webAuthnTestCredential(t, key, credential, challenge, state.UserID, false, true)
	response := assertion["response"].(map[string]any)
	authData, _ := base64.RawURLEncoding.DecodeString(response["authenticatorData"].(string))
	binary.BigEndian.PutUint32(authData[33:], 4)
	clientData, _ := base64.RawURLEncoding.DecodeString(response["clientDataJSON"].(string))
	clientHash := sha256.Sum256(clientData)
	signed := sha256.Sum256(append(authData, clientHash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, key, signed[:])
	if err != nil {
		t.Fatal(err)
	}
	response["authenticatorData"] = base64.RawURLEncoding.EncodeToString(authData)
	response["signature"] = base64.RawURLEncoding.EncodeToString(signature)
	rec = webAuthnTestRequest(t, handleWebAuthnFinish(s, false), "POST", "/auth/webauthn/login/finish", map[string]any{"transaction": id, "credential": assertion}, browser)
	if rec.Code != 401 || rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("positive counter regression accepted")
	}
	stored, err := s.loadWebAuthnState()
	if err != nil || stored.Credentials[0].Credential.Authenticator.SignCount != 5 {
		t.Fatal("rejected counter regression mutated registry", err)
	}
}
