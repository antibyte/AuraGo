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
	enrollment, err := remote.GetEnrollmentByTokenHash(s.RemoteHub.DB(), hashSHA256(payload.Token))
	if err != nil {
		t.Fatalf("GetEnrollmentByTokenHash: %v", err)
	}
	if enrollment.ID != payload.EnrollmentID || enrollment.DeviceName != "agodesk-desktop" || enrollment.Used {
		t.Fatalf("stored enrollment = %+v, response = %+v", enrollment, payload)
	}
	var rawCount int
	if err := s.RemoteHub.DB().QueryRow(`SELECT COUNT(*) FROM remote_enrollments WHERE token_hash = ?`, payload.Token).Scan(&rawCount); err != nil {
		t.Fatalf("query raw token count: %v", err)
	}
	if rawCount != 0 {
		t.Fatal("raw enrollment token must not be stored in remote_enrollments")
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

	cfg := &config.Config{}
	cfg.Server.Port = 8090
	if mutate != nil {
		mutate(cfg)
	}
	s := &Server{
		Cfg:       cfg,
		Logger:    slog.Default(),
		RemoteHub: remote.NewRemoteHub(db, nil, slog.Default()),
	}
	cleanup := func() {
		_ = os.Chdir(oldWD)
	}
	return s, cleanup
}
