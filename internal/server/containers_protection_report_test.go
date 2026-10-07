package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

type containerProtectionReport struct {
	Status            string `json:"status"`
	Owner             string `json:"owner"`
	Protected         bool   `json:"protected"`
	UpdateUnsupported bool   `json:"update_unsupported"`
	ReadOnly          bool   `json:"read_only"`
	Message           string `json:"message"`
}

func getContainerProtectionReport(t *testing.T, s *Server, method string) (int, containerProtectionReport) {
	t.Helper()
	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(method, "/api/containers/demo/protection", nil))
	var report containerProtectionReport
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, report
}

// TestContainerProtectionReportMatchesTheTerminalRules: the report answers what
// a terminal handshake would need, without an exec and without an upgrade, so
// the page can explain a refused handshake (K12 review, M7).
func TestContainerProtectionReportMatchesTheTerminalRules(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: true}
	t.Cleanup(replaceContainerTerminalBackend(fake))
	for _, tc := range []struct {
		name        string
		p           containerProtection
		owner       string
		protected   bool
		unsupported bool
	}{
		{"unprotected", containerProtection{}, "", false, false},
		{"managed", containerProtection{Owner: "go2rtc"}, "go2rtc", true, false},
		{"unverified", containerProtection{Unverified: true}, "unverified", true, false},
		{"self", containerProtection{Owner: dockerutil.AppOwner, Self: true}, "self", true, true},
		{"docker endpoint", containerProtection{DockerEndpoint: true}, "docker-endpoint", true, true},
		{"shared network", containerProtection{SharedNetwork: true}, "shared-network", true, false},
	} {
		restore := replaceContainerProtection(tc.p)
		code, report := getContainerProtectionReport(t, s, http.MethodGet)
		restore()
		if code != http.StatusOK || report.Status != "ok" || report.Owner != tc.owner || report.Protected != tc.protected || report.UpdateUnsupported != tc.unsupported || report.ReadOnly {
			t.Fatalf("%s: %d %+v", tc.name, code, report)
		}
		if tc.protected != strings.Contains(report.Message, "confirm=protected") {
			t.Fatalf("%s: message %q must carry the 409 text exactly when protected", tc.name, report.Message)
		}
	}
	if fake.createCalls != 0 {
		t.Fatal("the protection report must never start a shell")
	}

	t.Cleanup(replaceContainerProtection(containerProtection{Owner: "go2rtc"}))
	s.Cfg.Docker.ReadOnly = true
	if code, report := getContainerProtectionReport(t, s, http.MethodGet); code != http.StatusOK || !report.ReadOnly {
		t.Fatalf("read-only report = %d %+v, want read_only so the page offers no pointless confirmation", code, report)
	}
	if code, _ := getContainerProtectionReport(t, s, http.MethodPost); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST protection = %d, want 405", code)
	}
	s.Cfg.Docker.Enabled = false
	if code, _ := getContainerProtectionReport(t, s, http.MethodGet); code != http.StatusServiceUnavailable {
		t.Fatalf("disabled protection = %d, want 503", code)
	}
}

// TestContainerProtectionReportStaysAdminOnly: the new route is in the same
// admin Bearer catch-all as every other container route
// (TestContainerRoutesStayInTheAdminBearerCatchAll).
func TestContainerProtectionReportStaysAdminOnly(t *testing.T) {
	s, _, adminToken, desktopToken, readToken := newContainerAuthChain(t)
	const path = "/api/containers/demo/protection"
	if isDesktopScopedAPIPath(path) || isAuthBypassed(path) || isAllowedWithoutPassword(path) || isDesktopEmbedResourcePath(path) {
		t.Fatalf("%s must stay out of desktop, bypass, lockdown and embed lists", path)
	}
	if !validRouteBearer(s, adminToken, path, http.MethodGet) {
		t.Fatalf("GET %s: admin token refused", path)
	}
	for scope, raw := range map[string]string{"read": readToken, desktopScopeAdmin: desktopToken} {
		if validRouteBearer(s, raw, path, http.MethodGet) {
			t.Fatalf("GET %s: a %q token is admitted", path, scope)
		}
	}
}
