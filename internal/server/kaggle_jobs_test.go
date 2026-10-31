package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/kaggle"
)

func seedKaggleUnknownJob(t *testing.T, s *Server, id, state string) {
	t.Helper()
	if _, err := s.ensureKaggleServices(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(s.Cfg.Directories.DataDir, config.KaggleDBFilename))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO jobs(id,account,domain,operation,status,progress,result,error,created,updated,request,hash) VALUES(?,?,?,?,?,0,?,?,?,?,'{}','fixture-hash')`, id, "owner", "notebooks", "save_kernel", state, `{"ref":"owner/book"}`, "response lost", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
}

func TestKaggleUnknownResolutionHTTPAuthorizationAndConfirmation(t *testing.T) {
	s, read, write := testDesktopPermissionServer(t)
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.Kaggle = config.DefaultKaggleConfig()
	// Resolution is cleanup: disabled integrations, readonly and no network still allow it.
	s.Cfg.VirtualDesktop.ReadOnly = true
	t.Cleanup(s.closeKaggleServices)
	admin, _, err := s.TokenManager.Create("resolution admin", []string{"admin", desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminRead, _, err := s.TokenManager.Create("read-only admin", []string{"admin", desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	desktopAdmin, _, err := s.TokenManager.Create("desktop admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 32)
	seedKaggleUnknownJob(t, s, id, "unknown")
	endpoint := "/api/kaggle/jobs/" + id + "/resolve-unknown"
	call := func(method, path, token, body, origin string, session bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if session {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		}
		w := httptest.NewRecorder()
		s.handleKaggle(w, r)
		return w
	}
	for _, tc := range []struct {
		name, token, body, origin string
		session                   bool
		want                      int
	}{
		{"anonymous", "", `{"confirmed_inactive":true}`, "", false, 401},
		{"read", read, `{"confirmed_inactive":true}`, "", false, 403},
		{"write", write, `{"confirmed_inactive":true}`, "", false, 403},
		{"desktop-admin", desktopAdmin, `{"confirmed_inactive":true}`, "", false, 403},
		{"admin-without-desktop-write", adminRead, `{"confirmed_inactive":true}`, "", false, 403},
		{"missing-confirmation", admin, `{}`, "", false, 400},
		{"false-confirmation", admin, `{"confirmed_inactive":false}`, "", false, 400},
		{"cross-origin", "", `{"confirmed_inactive":true}`, "https://attacker.invalid", true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := call("POST", endpoint, tc.token, tc.body, tc.origin, tc.session)
			if w.Code != tc.want {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			job, _ := s.kaggleServices.Platform.Store.Get(t.Context(), id)
			if job.Status != "unknown" {
				t.Fatal("rejected request changed job")
			}
		})
	}
	for _, token := range []string{read, desktopAdmin, adminRead, admin} {
		w := call("GET", "/api/kaggle/status", token, "", "", false)
		var status struct {
			CanResolve bool `json:"can_resolve_unknown"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &status) != nil || status.CanResolve != (token == admin) {
			t.Fatalf("incorrect capability: %d %s", w.Code, w.Body.String())
		}
	}
	w := call("POST", endpoint, admin, `{"confirmed_inactive":true}`, "", false)
	var resolved kaggle.Job
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &resolved) != nil || resolved.Status != "resolved_unknown" || resolved.Error != "response lost" {
		t.Fatalf("resolution: %d %s", w.Code, w.Body.String())
	}
	if again := call("POST", endpoint, admin, `{"confirmed_inactive":true}`, "", false); again.Code != 200 || again.Body.String() != w.Body.String() {
		t.Fatal("non-idempotent repeat", again.Code)
	}
	other := strings.Repeat("b", 32)
	seedKaggleUnknownJob(t, s, other, "remote_running")
	if got := call("POST", strings.Replace(endpoint, id, other, 1), admin, `{"confirmed_inactive":true}`, "", false); got.Code != 409 {
		t.Fatal("active job resolved", got.Code)
	}
	if got := call("POST", strings.Replace(endpoint, id, strings.Repeat("c", 32), 1), admin, `{"confirmed_inactive":true}`, "", false); got.Code != 404 {
		t.Fatal("missing job", got.Code)
	}
	if got := call("GET", endpoint, read, "", "", false); got.Code != 404 {
		t.Fatal("resolution exposed through GET", got.Code)
	}
	owner := strings.Repeat("d", 32)
	seedKaggleUnknownJob(t, s, owner, "unknown")
	if got := call("POST", strings.Replace(endpoint, id, owner, 1), "", `{"confirmed_inactive":true}`, "http://example.com", true); got.Code != 200 {
		t.Fatal("same-origin owner blocked", got.Code, got.Body.String())
	}
}
