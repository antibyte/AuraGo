package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/inventory"
)

func rdpGatingServer(t *testing.T) (*Server, map[string]string) {
	t.Helper()
	s, _ := testDesktopRemoteGuardServer(t, inventory.DeviceRecord{
		ID: "rdp-a", Name: "rdp-a", Type: "server", Protocol: "rdp", IPAddress: "127.0.0.1",
		Username: `CORP\bob`, VaultSecretID: "rdp-a-pw", Tags: []string{"lab"},
	}, nil)
	if err := s.Vault.WriteSecret("rdp-a-pw", "vault-password"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	tokens := map[string]string{}
	for name, scopes := range map[string][]string{
		"admin":  {"desktop:admin"},
		"write":  {"desktop:write", "desktop:remote:device:rdp-a"},
		"device": {"desktop:remote:device:rdp-a"},
		"tag":    {"desktop:remote:tag:lab"},
		"remote": {"desktop:remote"},
	} {
		raw, _, err := s.TokenManager.Create(name, scopes, nil)
		if err != nil {
			t.Fatalf("create token %s: %v", name, err)
		}
		tokens[name] = raw
	}
	return s, tokens
}

func TestDesktopRemoteStrongCaller(t *testing.T) {
	s, tokens := rdpGatingServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))}

	newReq := func() *http.Request { return httptest.NewRequest(http.MethodPost, "/x?device_id=rdp-a", nil) }

	req := newReq()
	req.AddCookie(cookie)
	if !desktopRemoteStrongCaller(s, req) {
		t.Fatal("valid cookie session must qualify")
	}
	if desktopRemoteStrongCaller(s, newReq()) {
		t.Fatal("no credentials must not qualify")
	}
	bad := newReq()
	bad.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "forged"})
	if desktopRemoteStrongCaller(s, bad) {
		t.Fatal("forged cookie must not qualify")
	}

	for name, want := range map[string]bool{"admin": true, "write": false, "device": false, "tag": false, "remote": false} {
		req := newReq()
		req.Header.Set("Authorization", "Bearer "+tokens[name])
		if got := desktopRemoteStrongCaller(s, req); got != want {
			t.Errorf("token %s: got %v, want %v", name, got, want)
		}
	}
	unknown := newReq()
	unknown.Header.Set("Authorization", "Bearer not-a-token")
	if desktopRemoteStrongCaller(s, unknown) {
		t.Fatal("unknown bearer must not qualify")
	}

	s.Cfg.Auth.Enabled = false
	req = newReq()
	req.AddCookie(cookie)
	if desktopRemoteStrongCaller(s, req) {
		t.Fatal("with auth disabled even a well-formed cookie must not qualify")
	}
	if desktopRemoteStrongCaller(s, newReq()) {
		t.Fatal("with auth disabled nobody qualifies")
	}
}

func TestRDPTicketRouteReleasesPasswordOnlyToStrongCallers(t *testing.T) {
	s, tokens := rdpGatingServer(t)
	s.Cfg.Auth.MaxLoginAttempts = 100
	cookie := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))}
	store := desktop.NewRDPTicketStore()
	handler := withDesktopRemoteGuard(s, "desktop_rdp_ticket", "rdp",
		desktop.HandleRDPTicket(s.InventoryDB, s.Vault, s.Logger, store, func(r *http.Request) bool { return desktopRemoteStrongCaller(s, r) }, desktopRDPAudit(s)))

	call := func(mutate func(*http.Request)) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/desktop/rdp/ticket?device_id=rdp-a", nil)
		req.RemoteAddr = "203.0.113.30:1000"
		mutate(req)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}

	if b := call(func(r *http.Request) { r.AddCookie(cookie) }); b["password"] != "vault-password" || b["has_password"] != true {
		t.Fatalf("session must get the password: %v", b)
	}
	if b := call(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tokens["admin"]) }); b["password"] != "vault-password" {
		t.Fatalf("desktop:admin token must get the password: %v", b)
	}
	for _, name := range []string{"write", "device", "tag", "remote"} {
		b := call(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tokens[name]) })
		if b["password"] != "" || b["has_password"] != false || b["ticket"] == "" {
			t.Fatalf("token %s must get a ticket without password: %v", name, b)
		}
	}

	// Auth disabled: the guard lets the request through, the password stays home.
	s.Cfg.Auth.Enabled = false
	if b := call(func(r *http.Request) {}); b["password"] != "" || b["has_password"] != false || b["ticket"] == "" {
		t.Fatalf("auth disabled must not release the password: %v", b)
	}
}

func TestRDPPinResetRouteRequiresStrongCaller(t *testing.T) {
	s, tokens := rdpGatingServer(t)
	s.Cfg.Auth.MaxLoginAttempts = 100
	if err := inventory.SetRDPCertPin(s.InventoryDB, "rdp-a", "aa"); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))}
	handler := withDesktopRemoteGuard(s, "desktop_rdp_pin_reset", "rdp",
		desktop.HandleRDPPinReset(s.InventoryDB, s.Logger, func(r *http.Request) bool { return desktopRemoteStrongCaller(s, r) }, desktopRDPAudit(s)))
	do := func(mutate func(*http.Request)) int {
		req := httptest.NewRequest(http.MethodPost, "/api/desktop/rdp/pin/reset?device_id=rdp-a", nil)
		req.RemoteAddr = "203.0.113.31:1000"
		mutate(req)
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Code
	}
	if c := do(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tokens["device"]) }); c != http.StatusForbidden {
		t.Fatalf("device-scoped token: %d", c)
	}
	if _, found, _ := inventory.GetRDPCertPin(s.InventoryDB, "rdp-a"); !found {
		t.Fatal("refused reset must keep the pin")
	}
	if c := do(func(r *http.Request) { r.AddCookie(cookie) }); c != http.StatusOK {
		t.Fatalf("session: %d", c)
	}
	if _, found, _ := inventory.GetRDPCertPin(s.InventoryDB, "rdp-a"); found {
		t.Fatal("pin must be gone")
	}
}
