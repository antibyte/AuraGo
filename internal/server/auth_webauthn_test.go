package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"gopkg.in/yaml.v3"
)

func newWebAuthnTestServer(t *testing.T, mode string) (*Server, *http.Cookie) {
	t.Helper()
	s, cookie := newStepUpTestServer(t)
	data, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]interface{}
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document["directories"].(map[string]interface{})["data_dir"] = t.TempDir()
	data, err = yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Cfg.ConfigPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	s.CfgSaveMu.Lock()
	err = patchAuthConfig(s, map[string]interface{}{"enabled": true, "session_timeout_hours": 24, "webauthn": config.WebAuthnConfig{LoginMode: mode, RPID: "localhost", Origins: []string{"http://localhost"}}})
	s.CfgSaveMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return s, cookie
}

func webAuthnTestRequest(t *testing.T, handler http.HandlerFunc, method, path string, value any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost")
	req.RemoteAddr = "198.51.100.31:1234"
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func webAuthnTestBegin(t *testing.T, s *Server, registration bool, owner *http.Cookie, password string) (string, string, *http.Cookie) {
	t.Helper()
	path := "/auth/webauthn/login/begin"
	body := map[string]string{"password": password}
	if registration {
		path = "/api/auth/webauthn/register/begin"
		body = map[string]string{"label": "Test key", "current_password": stepUpTestPassword}
	}
	rec := webAuthnTestRequest(t, handleWebAuthnBegin(s, registration), http.MethodPost, path, body, owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("begin %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var data struct {
		Transaction string `json:"transaction"`
		Options     struct {
			PublicKey struct {
				Challenge string `json:"challenge"`
			} `json:"publicKey"`
		} `json:"options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	var browser *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == webAuthnCookie {
			browser = c
		}
	}
	if browser == nil {
		t.Fatal("missing browser binding cookie")
	}
	return data.Transaction, data.Options.PublicKey.Challenge, browser
}

func webAuthnTestKey(t *testing.T) (*ecdsa.PrivateKey, wa.Credential) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	return key, wa.Credential{ID: randomWebAuthnBytes(), PublicKey: publicKey, Flags: wa.NewCredentialFlags(protocol.AuthenticatorFlags(5))}
}

func webAuthnTestCredential(t *testing.T, key *ecdsa.PrivateKey, c wa.Credential, challenge string, userID []byte, registration, uv bool) map[string]any {
	t.Helper()
	encode := base64.RawURLEncoding.EncodeToString
	typ := "webauthn.get"
	if registration {
		typ = "webauthn.create"
	}
	clientData, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": "http://localhost", "crossOrigin": false})
	rp := sha256.Sum256([]byte("localhost"))
	authData := append([]byte(nil), rp[:]...)
	flags := byte(1)
	if uv {
		flags |= 4
	}
	if registration {
		flags |= 64
	}
	authData = append(authData, flags, 0, 0, 0, 0)
	response := map[string]any{"clientDataJSON": encode(clientData)}
	if registration {
		authData = append(authData, make([]byte, 16)...)
		authData = binary.BigEndian.AppendUint16(authData, uint16(len(c.ID)))
		authData = append(authData, c.ID...)
		authData = append(authData, c.PublicKey...)
		attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "authData": authData, "attStmt": map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		response["attestationObject"] = encode(attestation)
		response["transports"] = []string{"internal"}
	} else {
		clientHash := sha256.Sum256(clientData)
		signed := sha256.Sum256(append(authData, clientHash[:]...))
		signature, err := ecdsa.SignASN1(rand.Reader, key, signed[:])
		if err != nil {
			t.Fatal(err)
		}
		response["authenticatorData"] = encode(authData)
		response["signature"] = encode(signature)
		response["userHandle"] = encode(userID)
	}
	return map[string]any{"id": encode(c.ID), "rawId": encode(c.ID), "type": "public-key", "response": response, "clientExtensionResults": map[string]any{}}
}

func TestWebAuthnRegistrationAndBothLoginModes(t *testing.T) {
	s, owner := newWebAuthnTestServer(t, "passkey")
	key, credential := webAuthnTestKey(t)
	id, challenge, browser := webAuthnTestBegin(t, s, true, owner, "")
	created := webAuthnTestCredential(t, key, credential, challenge, nil, true, true)
	reg := webAuthnTestRequest(t, handleWebAuthnFinish(s, true), http.MethodPost, "/api/auth/webauthn/register/finish", map[string]any{"transaction": id, "credential": created}, owner, browser)
	if reg.Code != 200 {
		t.Fatalf("registration: %d %s", reg.Code, reg.Body.String())
	}
	state, err := s.loadWebAuthnState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Credentials) != 1 || len(state.UserID) != 32 {
		t.Fatal("credential not stored")
	}
	if _, err := s.Vault.ReadSecretForAgent(webAuthnVaultKey); err == nil {
		t.Fatal("registry exposed to agent")
	}
	if reg.Header().Get("Set-Cookie") != "" {
		t.Fatal("registration must not mint a login session")
	}
	for _, mode := range []string{"passkey", "second_factor"} {
		s.Cfg.Auth.WebAuthn.Enabled = true
		s.Cfg.Auth.WebAuthn.LoginMode = mode
		password := ""
		if mode == "second_factor" {
			password = stepUpTestPassword
		}
		id, challenge, browser = webAuthnTestBegin(t, s, false, nil, password)
		assertion := webAuthnTestCredential(t, key, credential, challenge, state.UserID, false, true)
		body := map[string]any{"transaction": id, "credential": assertion}
		login := webAuthnTestRequest(t, handleWebAuthnFinish(s, false), http.MethodPost, "/auth/webauthn/login/finish", body, browser)
		if login.Code != 200 {
			t.Fatalf("%s login: %d %s", mode, login.Code, login.Body.String())
		}
		valid := false
		for _, c := range login.Result().Cookies() {
			if c.Name == sessionCookieName {
				valid = validateSessionValue(s.Cfg.Auth.SessionSecret, c.Value)
			}
		}
		if !valid {
			t.Fatal("missing valid login cookie")
		}
		replay := webAuthnTestRequest(t, handleWebAuthnFinish(s, false), http.MethodPost, "/auth/webauthn/login/finish", body, browser)
		if replay.Code == 200 || replay.Header().Get("Set-Cookie") != "" {
			t.Fatal("assertion replay accepted")
		}
	}
}

func TestWebAuthnPasswordFallbackPolicy(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "second_factor")
	s.Cfg.Auth.WebAuthn.Enabled = true
	for _, mode := range []string{"second_factor", "passkey", "invalid"} {
		s.Cfg.Auth.WebAuthn.LoginMode = mode
		rec := webAuthnTestRequest(t, handleAuthLogin(s), http.MethodPost, "/auth/login", map[string]string{"password": stepUpTestPassword})
		if (rec.Code == 200) != (mode == "passkey") {
			t.Fatalf("%s: %d %s", mode, rec.Code, rec.Body.String())
		}
		if mode != "passkey" && rec.Header().Get("Set-Cookie") != "" {
			t.Fatal("password-only cookie escaped policy")
		}
	}
	s.Cfg.Auth.WebAuthn.LoginMode = "second_factor"
	secret := enableStepUpTestTOTP(t, s)
	rec := webAuthnTestRequest(t, handleAuthLogin(s), http.MethodPost, "/auth/login", map[string]string{"password": stepUpTestPassword, "totp_code": currentStepUpTestCode(t, secret)})
	if rec.Code != 200 {
		t.Fatalf("TOTP fallback: %d %s", rec.Code, rec.Body.String())
	}
	wrong := webAuthnTestRequest(t, handleWebAuthnBegin(s, false), http.MethodPost, "/auth/webauthn/login/begin", map[string]string{"password": "wrong"})
	if wrong.Code == 200 || wrong.Header().Get("Set-Cookie") != "" {
		t.Fatal("wrong first factor produced ceremony/cookie")
	}
}

func seedWebAuthnTestCredential(t *testing.T, s *Server) (*ecdsa.PrivateKey, wa.Credential, *webAuthnState) {
	t.Helper()
	key, credential := webAuthnTestKey(t)
	state := &webAuthnState{Version: 1, RPID: "localhost", UserID: randomWebAuthnBytes(), Credentials: []webAuthnCredential{{Credential: credential, Label: "Owner key", CreatedAt: time.Now()}}}
	if err := s.saveWebAuthnState(state); err != nil {
		t.Fatal(err)
	}
	s.Cfg.Auth.WebAuthn.Enabled = true
	return key, credential, state
}

func TestWebAuthnRejectsInvalidAssertionsAndBindings(t *testing.T) {
	for _, kind := range []string{"UV absent", "wrong signature", "wrong browser", "wrong origin", "expired", "mode changed", "password changed", "first factor absent", "key removed", "wrong RP", "wrong user", "Vault replaced"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := newWebAuthnTestServer(t, "second_factor")
			key, c, state := seedWebAuthnTestCredential(t, s)
			id, challenge, browser := webAuthnTestBegin(t, s, false, nil, stepUpTestPassword)
			if kind == "wrong signature" {
				key, _ = webAuthnTestKey(t)
			}
			if kind == "wrong user" {
				state.UserID = randomWebAuthnBytes()
			}
			assertion := webAuthnTestCredential(t, key, c, challenge, state.UserID, false, kind != "UV absent")
			s.webAuthn.mu.Lock()
			tx := s.webAuthn.transactions[id]
			switch kind {
			case "expired":
				tx.Expires = time.Now().Add(-time.Second)
			case "first factor absent":
				tx.FirstFactor = false
			case "wrong RP":
				tx.Session.RelyingPartyID = "foreign.test"
			}
			s.webAuthn.transactions[id] = tx
			s.webAuthn.mu.Unlock()
			if kind == "mode changed" {
				s.Cfg.Auth.WebAuthn.LoginMode = "passkey"
			}
			if kind == "password changed" {
				s.Cfg.Auth.PasswordHash = "replacement-hash"
			}
			if kind == "key removed" {
				state.Credentials = nil
				if err := s.saveWebAuthnState(state); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "wrong browser" {
				browser.Value = base64.RawURLEncoding.EncodeToString(randomWebAuthnBytes())
			}
			if kind == "wrong origin" {
				tx.Origin = "https://evil.test"
				s.webAuthn.transactions[id] = tx
			}
			if kind == "Vault replaced" {
				var err error
				s.Vault, err = security.NewVault(strings.Repeat("ab", 32), t.TempDir()+"/vault.bin")
				if err != nil {
					t.Fatal(err)
				}
			}
			rec := webAuthnTestRequest(t, handleWebAuthnFinish(s, false), http.MethodPost, "/auth/webauthn/login/finish", map[string]any{"transaction": id, "credential": assertion}, browser)
			if rec.Code == 200 || rec.Header().Get("Set-Cookie") != "" {
				t.Fatalf("accepted %s: %d", kind, rec.Code)
			}
		})
	}
}

func TestWebAuthnManagementAndGenericPolicyGuards(t *testing.T) {
	s, owner := newWebAuthnTestServer(t, "second_factor")
	_, c, _ := seedWebAuthnTestCredential(t, s)
	path := "/api/auth/webauthn/credentials/" + webAuthnCredentialRef(c.ID)
	deleted := webAuthnTestRequest(t, handleWebAuthnCredential(s), http.MethodDelete, path, map[string]string{"current_password": stepUpTestPassword}, owner)
	if deleted.Code != http.StatusConflict {
		t.Fatalf("last factor removal: %d", deleted.Code)
	}
	renamed := webAuthnTestRequest(t, handleWebAuthnCredential(s), http.MethodPatch, path, map[string]string{"current_password": stepUpTestPassword, "label": "Backup key"}, owner)
	if renamed.Code != 200 {
		t.Fatalf("rename: %d %s", renamed.Code, renamed.Body.String())
	}
	for _, patch := range []map[string]interface{}{
		{"auth": map[string]interface{}{"webauthn": map[string]interface{}{"enabled": false}}},
		{"auth": nil}, {"auth": map[string]interface{}{"enabled": false}},
		{"auth": map[string]interface{}{"totp_enabled": false, "totp_secret": ""}},
		{"auth.webauthn.enabled": false},
	} {
		if !rejectWebAuthnConfigPatch(s.Cfg, patch) {
			t.Fatalf("generic bypass accepted: %v", patch)
		}
	}
	if rejectWebAuthnConfigPatch(s.Cfg, map[string]interface{}{"auth": map[string]interface{}{"session_timeout_hours": 12}}) {
		t.Fatal("ordinary auth settings blocked")
	}
	settings := s.Cfg.Auth.WebAuthn
	settings.Enabled = false
	changed := webAuthnTestRequest(t, handleWebAuthnSettings(s), http.MethodPut, "/api/auth/webauthn/settings", map[string]any{"settings": settings, "current_password": stepUpTestPassword}, owner)
	if changed.Code != 200 || s.Cfg.Auth.WebAuthn.Enabled {
		t.Fatalf("protected disable: %d %s", changed.Code, changed.Body.String())
	}
	deleted = webAuthnTestRequest(t, handleWebAuthnCredential(s), http.MethodDelete, path, map[string]string{"current_password": stepUpTestPassword}, owner)
	if deleted.Code != 200 {
		t.Fatalf("disabled removal: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestWebAuthnCorruptRegistryDoesNotBecomeAnEmptyAccount(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "passkey")
	for _, raw := range []string{"not json", `{"version":999}`, `{"version":1,"user_id":"bad"}`} {
		if err := s.Vault.WriteSecret(webAuthnVaultKey, raw); err != nil {
			t.Fatal(err)
		}
		if _, err := s.loadWebAuthnState(); err == nil {
			t.Fatal("corrupt registry accepted")
		}
		got, _ := s.Vault.ReadSecret(webAuthnVaultKey)
		if got != raw {
			t.Fatal("corrupt registry overwritten")
		}
	}
}
