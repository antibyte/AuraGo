package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/injectionguard"
)

func TestInjectionGuardAdminAndPassiveStatus(t *testing.T) {
	s, admin, desktop, read := newBearerSchemeTestServer(t)
	root := t.TempDir()
	s.InjectionGuard = injectionguard.NewManager(context.Background(), root, "")
	t.Cleanup(s.InjectionGuard.Close)
	for _, token := range []string{admin, desktop, read, ""} {
		for _, operation := range []string{"status", "cancel", "test"} {
			method := http.MethodPost
			if operation == "status" {
				method = http.MethodGet
			}
			r := httptest.NewRequest(method, "/api/guardian/injection-guard/"+operation, nil)
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			authMiddleware(s, requireAdmin(s, handleInjectionGuard(s, operation))).ServeHTTP(w, r)
			if token != admin {
				if w.Code != 401 && w.Code != 403 {
					t.Fatalf("non-admin %s: %d", operation, w.Code)
				}
			} else if operation == "test" {
				if w.Code != http.StatusConflict {
					t.Fatalf("missing model test: %d", w.Code)
				}
			} else if w.Code != 200 {
				t.Fatalf("admin %s: %d %s", operation, w.Code, w.Body.String())
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("passive requests created runtime files")
	}
	w := httptest.NewRecorder()
	handleInjectionGuard(s, "download")(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET download accepted")
	}
	cfg := &config.Config{}
	cfg.Guardian.InjectionGuard.Enabled = true
	if validateInjectionGuardActivation(s, cfg) == nil {
		t.Fatal("unverified model enabled")
	}
	cfg.Guardian.InjectionGuard.Enabled = false
	if err := validateInjectionGuardActivation(s, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInjectionGuardMailProvenanceRequiresLoopbackToken(t *testing.T) {
	s := &Server{internalToken: "fixture-token"}
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-External-Source", "email")
	r.Header.Set("X-Internal-FollowUp", "true")
	if trustedMailIngress(r, s) {
		t.Fatal("forged source accepted")
	}
	r.Header.Set("X-Internal-Token", s.internalToken)
	if !trustedMailIngress(r, s) {
		t.Fatal("trusted relay rejected")
	}
	r.RemoteAddr = "192.0.2.8:1234"
	if trustedMailIngress(r, s) {
		t.Fatal("non-loopback source accepted")
	}
}

func TestUntrustedProvenanceHeaderRequiresLoopbackToken(t *testing.T) {
	s := &Server{internalToken: "fixture-token"}
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Internal-FollowUp", "true")
	r.Header.Set("X-Untrusted-Provenance", "true")
	if trustedUntrustedProvenance(r, s) {
		t.Fatal("forged provenance header accepted")
	}
	r.Header.Set("X-Internal-Token", s.internalToken)
	if !trustedUntrustedProvenance(r, s) {
		t.Fatal("authenticated provenance header rejected")
	}
	r.Header.Del("X-Untrusted-Provenance")
	if trustedUntrustedProvenance(r, s) {
		t.Fatal("absent header read as untrusted")
	}
}
