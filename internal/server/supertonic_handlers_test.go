package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestHandleSupertonicStatusMapsHealthOKToRunning(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Fatalf("path = %q, want /v1/health", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","model":"supertonic-3","voices_loaded":10}`))
	}))
	defer backend.Close()

	cfg := &config.Config{}
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = backend.URL
	cfg.TTS.Supertonic.AutoStart = true
	cfg.TTS.Supertonic.ContainerName = "aurago-supertonic-tts"
	cfg.TTS.Supertonic.Image = "ghcr.io/antibyte/aurago-supertonic:latest"
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	rec := httptest.NewRecorder()
	handleSupertonicStatus(s)(rec, httptest.NewRequest(http.MethodGet, "/api/supertonic/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["status"] != "running" {
		t.Fatalf("status = %#v, want running; body=%s", got["status"], rec.Body.String())
	}
	if got["upstream_status"] != "ok" {
		t.Fatalf("upstream_status = %#v, want ok", got["upstream_status"])
	}
	if got["auto_start"] != true {
		t.Fatalf("auto_start = %#v, want true", got["auto_start"])
	}
}

func TestHandleSupertonicStylesReturnsStructuredStartingRetry(t *testing.T) {
	oldList := supertonicListStyles
	oldLifecycle := supertonicLifecycleStatus
	t.Cleanup(func() {
		supertonicListStyles = oldList
		supertonicLifecycleStatus = oldLifecycle
	})
	supertonicListStyles = func(string) ([]tools.SupertonicStyle, error) {
		return nil, errors.New("connection refused")
	}
	supertonicLifecycleStatus = func() tools.SupertonicLifecycleStatus {
		return tools.SupertonicLifecycleStatus{State: "pulling", RetryAfterSeconds: 3}
	}
	s := &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	handleSupertonicStyles(s)(rec, httptest.NewRequest(http.MethodGet, "/api/supertonic/styles", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "3" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "supertonic_starting" || got["runtime_state"] != "pulling" {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleSupertonicStylesReturnsStyles(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/styles" {
			t.Fatalf("path = %q, want /v1/styles", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"styles":[{"name":"M1","kind":"builtin"},{"name":"studio","kind":"custom","path":"/root/.cache/supertonic3/custom_styles/studio.json"}]}`))
	}))
	defer backend.Close()

	cfg := &config.Config{}
	cfg.TTS.Supertonic.URL = backend.URL
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	rec := httptest.NewRecorder()
	handleSupertonicStyles(s)(rec, httptest.NewRequest(http.MethodGet, "/api/supertonic/styles", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var got struct {
		Styles []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"styles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Styles) != 2 || got.Styles[0].Name != "M1" || got.Styles[1].Kind != "custom" {
		t.Fatalf("styles = %#v", got.Styles)
	}
}
