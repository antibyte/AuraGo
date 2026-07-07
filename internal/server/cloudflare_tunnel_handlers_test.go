package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestBuildTunnelConfigIncludesRuntimeFields(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Docker.Enabled = true
	s.Cfg.Docker.Host = "tcp://docker.example.test:2375"
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.Server.Port = 8080
	s.Cfg.Server.HTTPS.Enabled = true
	s.Cfg.Server.HTTPS.HTTPSPort = 9443
	s.Cfg.Homepage.WebServerPort = 3000
	s.Cfg.CloudflareTunnel.Enabled = true
	s.Cfg.CloudflareTunnel.TunnelID = "tunnel-uuid"
	s.Cfg.CloudflareTunnel.LoopbackPort = 18448
	s.Cfg.CloudflareTunnel.MetricsPort = -1
	s.Cfg.CloudflareTunnel.LogLevel = ""

	got := s.buildTunnelConfig()
	if got.Mode != "auto" {
		t.Fatalf("Mode = %q, want auto", got.Mode)
	}
	if got.AuthMethod != "token" {
		t.Fatalf("AuthMethod = %q, want token", got.AuthMethod)
	}
	if got.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", got.LogLevel)
	}
	if got.MetricsPort != 0 {
		t.Fatalf("MetricsPort = %d, want normalized 0", got.MetricsPort)
	}
	if got.TunnelID != "tunnel-uuid" {
		t.Fatalf("TunnelID = %q, want tunnel-uuid", got.TunnelID)
	}
	if got.LoopbackPort != 18448 {
		t.Fatalf("LoopbackPort = %d, want 18448", got.LoopbackPort)
	}
	if got.DockerHost != "tcp://docker.example.test:2375" {
		t.Fatalf("DockerHost = %q", got.DockerHost)
	}
	if got.DataDir == "" {
		t.Fatal("DataDir is empty")
	}
	if got.WebUIPort != 8080 {
		t.Fatalf("WebUIPort = %d, want 8080", got.WebUIPort)
	}
	if got.HomepagePort != 3000 {
		t.Fatalf("HomepagePort = %d, want 3000", got.HomepagePort)
	}
	if !got.HTTPSEnabled || got.HTTPSPort != 9443 {
		t.Fatalf("HTTPS = %v:%d, want true:9443", got.HTTPSEnabled, got.HTTPSPort)
	}
}

func TestHandleCloudflareTunnelRestartPreservesToolError(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.CloudflareTunnel.Enabled = true
	s.Cfg.CloudflareTunnel.ReadOnly = true

	req := httptest.NewRequest(http.MethodPost, "/api/cloudflare-tunnel/restart", nil)
	rec := httptest.NewRecorder()
	handleCloudflareTunnelRestart(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body["status"] == "ok" {
		t.Fatalf("status = ok, want error preserved; body=%#v", body)
	}
	msg := strings.ToLower(firstNonEmptyTunnelTestString(body["error"], body["message"]))
	if !strings.Contains(msg, "read-only") {
		t.Fatalf("error/message = %q, want read-only diagnostic; body=%#v", msg, body)
	}
}

func firstNonEmptyTunnelTestString(values ...interface{}) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
