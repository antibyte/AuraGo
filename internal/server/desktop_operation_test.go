package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

func TestDesktopReadonlyOperationMatrix(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("desktop admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	for _, tc := range []struct {
		name, token, scope string
		op                 desktopOperation
		allowed            bool
	}{
		{"read token read", readToken, desktopScopeRead, desktopRead, true},
		{"read token write", readToken, desktopScopeWrite, desktopWrite, false},
		{"write token write", writeToken, desktopScopeWrite, desktopWrite, false},
		{"admin read", adminToken, desktopScopeAdmin, desktopRead, true},
		{"admin write", adminToken, desktopScopeAdmin, desktopWrite, false},
		{"admin execute", adminToken, desktopScopeAdmin, desktopExecute, false},
		{"admin stop", adminToken, desktopScopeAdmin, desktopStop, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/desktop/test", nil)
			r.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()
			if got := requireDesktopOperation(s, w, r, tc.scope, tc.op); got != tc.allowed {
				t.Fatalf("allowed=%v status=%d %s", got, w.Code, w.Body.String())
			}
			if !tc.allowed && w.Code != http.StatusForbidden {
				t.Fatal(w.Code)
			}
			if !tc.allowed && tc.token != readToken && !strings.Contains(w.Body.String(), `"code":"desktop_readonly"`) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestDesktopReadonlyAlsoAppliesToAuthenticatedBrowser(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.ReadOnly = true
	r := httptest.NewRequest(http.MethodPost, "/api/desktop/chat", nil)
	cookies := httptest.NewRecorder()
	SetSessionCookie(cookies, r, s.Cfg.Auth.SessionSecret, time.Hour)
	for _, cookie := range cookies.Result().Cookies() {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	if requireDesktopOperation(s, w, r, desktopScopeAdmin, desktopExecute) || w.Code != 403 || !strings.Contains(w.Body.String(), "desktop_readonly") {
		t.Fatalf("browser bypass: %d %s", w.Code, w.Body.String())
	}
}

func TestDesktopRevocationCancelsRunsAndRejectsLatePublication(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	ctx, done, err := s.beginDesktopRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	dir := t.TempDir()
	path := filepath.Join(dir, "original.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.ReadOnly = true
	s.revokeDesktopRuns()
	s.CfgMu.Unlock()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("run survived revocation")
	}
	if err := fileutil.WriteFileContext(ctx, path, []byte("late"), 0600); !errors.Is(err, context.Canceled) {
		t.Fatalf("late write: %v", err)
	}
	called := false
	if err := publishDesktopResult(ctx, func() error { called = true; return nil }); !errors.Is(err, context.Canceled) || called {
		t.Fatalf("late result published: %v", err)
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	if err := publishDesktopResult(ctx, func() error { called = true; return nil }); err == nil || called {
		t.Fatal("old generation revived")
	}
	if got, _ := os.ReadFile(path); string(got) != "original" {
		t.Fatal("original changed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
}

func TestDesktopBackgroundRunOutlivesRequestButNotOwner(t *testing.T) {
	owner, cancel := context.WithCancel(context.Background())
	s := &Server{Cfg: &config.Config{}, integrationCtx: owner}
	ctx, done, err := s.beginDesktopBackgroundRun(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("owner shutdown did not cancel run")
	}
}

func TestDesktopReadonlyConfigPublicationKeepsServiceAndRevokesRun(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, hub, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, done, err := s.beginDesktopRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	next := *s.Cfg
	next.VirtualDesktop.ReadOnly = true
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(&next)
	s.CfgMu.Unlock()
	if ctx.Err() == nil || !svc.Config().ReadOnly {
		t.Fatal("policy publication did not revoke run and service writes")
	}
	after, afterHub, err := s.getDesktopService(context.Background())
	if err != nil || after != svc || afterHub != hub {
		t.Fatalf("readonly reopened the service: %v", err)
	}
}
