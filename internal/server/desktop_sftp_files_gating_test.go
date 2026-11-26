package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func TestSFTPRoutesGateRDPFileAccessToStrongCallers(t *testing.T) {
	s, tokens := rdpGatingServer(t)
	s.Cfg.Auth.MaxLoginAttempts = 100
	cookie := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))}
	filesOpts := desktop.RemoteFilesOptions{AllowStoredRDPSecret: func(r *http.Request) bool { return desktopRemoteStrongCaller(s, r) }, Audit: desktopRDPAudit(s)}
	handler := withDesktopRemoteGuard(s, "desktop_sftp_list", "", desktop.HandleSFTPList(s.InventoryDB, s.Vault, s.Logger, filesOpts))

	call := func(mutate func(*http.Request)) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/desktop/sftp/list?device_id=rdp-a&path=/", nil)
		req.RemoteAddr = "203.0.113.31:1000"
		mutate(req)
		rec := httptest.NewRecorder()
		handler(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	// Scoped tokens are never strong callers: the handler refuses them, or the
	// guard does before it gets there.
	for _, name := range []string{"write", "device", "tag", "remote"} {
		code, body := call(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tokens[name]) })
		if code == http.StatusOK {
			t.Errorf("token %s listed an RDP device: %v", name, body)
		}
		if code == http.StatusForbidden && body != nil && body["code"] != nil && body["code"] != "file_access_admin_required" {
			t.Errorf("token %s: unexpected code %v", name, body["code"])
		}
	}
	// A session passes the gate; the device has no file access row, so it is off.
	code, body := call(func(r *http.Request) { r.AddCookie(cookie) })
	if code != http.StatusForbidden || body["code"] != "file_access_disabled" {
		t.Fatalf("cookie session: %d %v", code, body)
	}
	code, body = call(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tokens["admin"]) })
	if code != http.StatusForbidden || body["code"] != "file_access_disabled" {
		t.Fatalf("admin token: %d %v", code, body)
	}
	// Auth disabled: nobody is a strong caller.
	s.Cfg.Auth.Enabled = false
	code, body = call(func(r *http.Request) {})
	if code != http.StatusForbidden || body["code"] != "file_access_admin_required" {
		t.Fatalf("auth disabled: %d %v", code, body)
	}
}

func TestSFTPHostKeyTrustActionIsGuardedAsExecute(t *testing.T) {
	s, tokens := rdpGatingServer(t)
	s.Cfg.Auth.MaxLoginAttempts = 100
	reached := false
	handler := withDesktopRemoteGuard(s, "desktop_sftp_hostkey_trust", "", func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/sftp/hostkey?device_id=rdp-a", nil)
	req.Header.Set("Authorization", "Bearer "+tokens["device"])
	req.RemoteAddr = "203.0.113.32:1000"
	rec := httptest.NewRecorder()
	handler(rec, req)
	// The device-scoped remote token carries execute rights only; a write-mapped
	// action would have been refused by the guard.
	if !reached || rec.Code != http.StatusNoContent {
		t.Fatalf("execute-scoped token must reach the trust handler: reached=%v status=%d %s", reached, rec.Code, rec.Body.String())
	}
}
