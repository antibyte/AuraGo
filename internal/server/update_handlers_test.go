package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleUpdateInstallRequiresSelfUpdatePermission(t *testing.T) {
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.ConfigPath = t.TempDir() + "/config.yaml"
	s.Cfg.Agent.AllowSelfUpdate = false

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/updates/install", nil)

	handleUpdateInstall(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("response missing error: %#v", body)
	}
}

func TestHandleUpdateInstallKeepsMissingScriptPathWhenAllowed(t *testing.T) {
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.ConfigPath = t.TempDir() + "/config.yaml"
	s.Cfg.Agent.AllowSelfUpdate = true

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/updates/install", nil)

	handleUpdateInstall(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleUpdateInstallBlocksDockerRuntime(t *testing.T) {
	dir := t.TempDir()
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Cfg.Agent.AllowSelfUpdate = true
	s.Cfg.Runtime.IsDocker = true

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/updates/install", nil)

	handleUpdateInstall(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if !strings.Contains(body["error"], "Docker") {
		t.Fatalf("expected Docker block error, got %#v", body)
	}
}

func TestHandleUpdateInstallBlocksMissingBash(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "update.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write update.sh: %v", err)
	}
	oldGOOS := updateGOOS
	oldLookPath := updateLookPath
	updateGOOS = "linux"
	updateLookPath = func(name string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() {
		updateGOOS = oldGOOS
		updateLookPath = oldLookPath
	})

	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.ConfigPath = filepath.Join(dir, "config.yaml")
	s.Cfg.Agent.AllowSelfUpdate = true

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/updates/install", nil)

	handleUpdateInstall(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "bash") {
		t.Fatalf("expected missing bash error, got %s", rec.Body.String())
	}
}
