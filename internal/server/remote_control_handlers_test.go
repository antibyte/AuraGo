package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/remote"
)

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
