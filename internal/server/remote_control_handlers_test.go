package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/remote"
	"aurago/internal/security"
)

func TestRemoteEnrollmentCreateReturnsOneTimeToken(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, nil)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/remote/enroll", strings.NewReader(`{"device_name":"agodesk-desktop"}`))
	req.Header.Set("Content-Type", "application/json")
	handleRemoteEnrollmentCreate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		EnrollmentID string `json:"enrollment_id"`
		Token        string `json:"token"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.EnrollmentID == "" || payload.Token == "" || payload.ExpiresAt == "" {
		t.Fatalf("payload missing required fields: %+v", payload)
	}
	enrollment := assertRemoteTokenStoredAsLookupHash(t, s, payload.Token)
	if enrollment.ID != payload.EnrollmentID || enrollment.DeviceName != "agodesk-desktop" || enrollment.Used {
		t.Fatalf("stored enrollment = %+v, response = %+v", enrollment, payload)
	}
}

// assertRemoteTokenStoredAsLookupHash checks that remote_enrollments holds only
// the lookup hash of token and the vault holds its MAC key.
func assertRemoteTokenStoredAsLookupHash(t *testing.T, s *Server, token string) remote.EnrollmentRecord {
	t.Helper()
	enrollment, err := remote.GetEnrollmentByTokenHash(s.RemoteHub.DB(), remote.DeriveEnrollmentLookupHash(token))
	if err != nil {
		t.Fatalf("enrollment not stored under its lookup hash: %v", err)
	}
	for name, value := range map[string]string{
		"raw token":  token,
		"plain hash": hashSHA256(token),
		"MAC key":    remote.DeriveEnrollmentAuthKey(token),
	} {
		var count int
		if err := s.RemoteHub.DB().QueryRow(`SELECT COUNT(*) FROM remote_enrollments WHERE token_hash = ?`, value).Scan(&count); err != nil {
			t.Fatalf("query %s count: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("remote_enrollments must not hold the %s", name)
		}
	}
	key, err := s.Vault.ReadSecret("remote_enroll_key_" + enrollment.ID)
	if err != nil || key != remote.DeriveEnrollmentAuthKey(token) {
		t.Fatalf("vault MAC key for enrollment %s: %v", enrollment.ID, err)
	}
	return enrollment
}

func TestRemoteEnrollmentCreateFailsWhenMACKeyCannotBeStored(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, nil)
	defer cleanup()
	brokenVault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "missing", "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vault = brokenVault
	s.RemoteHub = remote.NewRemoteHub(s.RemoteHub.DB(), brokenVault, slog.Default())

	for name, serve := range map[string]func() *httptest.ResponseRecorder{
		"create": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			handleRemoteEnrollmentCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/remote/enroll", strings.NewReader(`{"device_name":"x"}`)))
			return rec
		},
		"download": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
			req.Host = "localhost:8090"
			handleRemoteDownload(s).ServeHTTP(rec, req)
			return rec
		},
	} {
		if rec := serve(); rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status = %d, want 500; body=%s", name, rec.Code, rec.Body.String())
		}
	}
	var count int
	if err := s.RemoteHub.DB().QueryRow(`SELECT COUNT(*) FROM remote_enrollments`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("a token without a stored MAC key must not be created, got %d rows", count)
	}
}

func TestRemoteDownloadTrailerCarriesRawTokenAndStoresLookupHash(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, nil)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
	req.Host = "localhost:8090"
	handleRemoteDownload(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("parse personalized binary trailer: %v", err)
	}
	if !strings.HasPrefix(trailer.EnrollToken, "remote_") || trailer.DeviceName != "nas" {
		t.Fatalf("trailer must carry the raw token and device name: %+v", trailer)
	}
	enrollment := assertRemoteTokenStoredAsLookupHash(t, s, trailer.EnrollToken)
	if enrollment.DeviceName != "nas" || enrollment.Used {
		t.Fatalf("stored enrollment = %+v", enrollment)
	}
}

func TestRemoteDownloadUsesTailscaleSupervisorURL(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, func(cfg *config.Config) {
		cfg.Server.Port = 8090
		cfg.RemoteControl.ConnectionMode = "tailscale"
		cfg.RemoteControl.TailscaleAddress = "aurago.tailnet.ts.net"
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
	req.Host = "localhost:8090"
	handleRemoteDownload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("parse personalized binary trailer: %v", err)
	}
	if trailer.SupervisorURL != "wss://aurago.tailnet.ts.net/api/remote/ws" {
		t.Fatalf("supervisor_url = %q", trailer.SupervisorURL)
	}
}

func TestRemoteDownloadUsesManualSupervisorURL(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, func(cfg *config.Config) {
		cfg.RemoteControl.ConnectionMode = "manual"
		cfg.RemoteControl.SupervisorURL = "https://remote.example.com/custom/ws"
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
	req.Host = "localhost:8090"
	handleRemoteDownload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("parse personalized binary trailer: %v", err)
	}
	if trailer.SupervisorURL != "wss://remote.example.com/custom/ws" {
		t.Fatalf("supervisor_url = %q", trailer.SupervisorURL)
	}
}

func TestRemoteDownloadPrefersWSSWhenServerTLSEnabled(t *testing.T) {
	cases := []struct {
		name           string
		httpsEnabled   bool
		forwardedProto string
		want           string
	}{
		// The TLS listener binds server.https.https_port; server.port is only
		// the loopback listener then.
		{name: "server TLS, plain request", httpsEnabled: true, want: "wss://aurago.lan:8443/api/remote/ws"},
		{name: "no server TLS, plain request", want: "ws://aurago.lan:8090/api/remote/ws"},
		{name: "no server TLS, forwarded https", forwardedProto: "https", want: "wss://aurago.lan:8090/api/remote/ws"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, cleanup := newRemoteDownloadTestServer(t, func(cfg *config.Config) {
				cfg.Server.Port = 8090
				cfg.Server.HTTPS.Enabled = tc.httpsEnabled
				cfg.Server.HTTPS.HTTPSPort = 8443
			})
			defer cleanup()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
			req.Host = "aurago.lan:8090"
			if tc.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}
			handleRemoteDownload(s).ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
			if err != nil {
				t.Fatalf("parse personalized binary trailer: %v", err)
			}
			if trailer.SupervisorURL != tc.want {
				t.Fatalf("supervisor_url = %q, want %q", trailer.SupervisorURL, tc.want)
			}
		})
	}

	// Without a request host the fallback also prefers the TLS listener; an
	// unset https_port means the 443 default.
	cfg := &config.Config{}
	cfg.Server.Port = 8090
	cfg.Server.HTTPS.Enabled = true
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64", nil)
	req.Host = ""
	if got := autoRemoteDownloadSupervisorURL(&Server{Cfg: cfg}, req); got != "wss://localhost:443/api/remote/ws" {
		t.Fatalf("fallback supervisor_url = %q, want wss://localhost:443/api/remote/ws", got)
	}
}

func newRemoteDownloadTestServer(t *testing.T, mutate func(*config.Config)) (*Server, func()) {
	t.Helper()

	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.MkdirAll("deploy", 0o755); err != nil {
		t.Fatalf("MkdirAll deploy: %v", err)
	}
	generic, err := remote.BuildPersonalizedBinary([]byte("binary"), remote.BinaryConfig{SupervisorURL: "ws://placeholder", EnrollToken: "placeholder"})
	if err != nil {
		t.Fatalf("BuildPersonalizedBinary fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join("deploy", "aurago-remote_linux_amd64"), generic, 0o644); err != nil {
		t.Fatalf("WriteFile binary: %v", err)
	}

	db, err := remote.InitDB(filepath.Join(tmp, "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tmp, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}

	cfg := &config.Config{}
	cfg.Server.Port = 8090
	if mutate != nil {
		mutate(cfg)
	}
	s := &Server{
		Cfg:       cfg,
		Logger:    slog.Default(),
		Vault:     vault,
		RemoteHub: remote.NewRemoteHub(db, vault, slog.Default()),
	}
	cleanup := func() {
		_ = os.Chdir(oldWD)
	}
	return s, cleanup
}
