package server

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestJellyfinStatusReportsClientInitializationDetails(t *testing.T) {
	s := &Server{
		Cfg: &config.Config{
			Jellyfin: config.JellyfinConfig{
				Enabled: true,
				Host:    "jellyfin.local",
				Port:    8096,
			},
		},
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jellyfin/status", nil)
	rec := httptest.NewRecorder()
	handleJellyfinStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("payload status = %#v, want error", payload["status"])
	}
	message, _ := payload["error"].(string)
	if !strings.Contains(message, "API key is required") {
		t.Fatalf("error = %q, want actionable API key detail", message)
	}
	if strings.Contains(message, "test-api-key") {
		t.Fatalf("error leaked API key value: %q", message)
	}
}

func TestJellyfinStatusDoesNotExposeReflectedAuthorization(t *testing.T) {
	const apiKey = "jellyfin-status-secret-token"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("reflected " + r.Header.Get("Authorization")))
	}))
	defer upstream.Close()

	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split upstream host: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse upstream port: %v", err)
	}

	s := &Server{
		Cfg: &config.Config{
			Jellyfin: config.JellyfinConfig{
				Enabled: true,
				Host:    host,
				Port:    port,
				APIKey:  apiKey,
			},
		},
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jellyfin/status", nil)
	rec := httptest.NewRecorder()
	handleJellyfinStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	message, _ := payload["error"].(string)
	if strings.Contains(message, apiKey) {
		t.Fatalf("status error leaked API key: %q", message)
	}
	if strings.Contains(message, "MediaBrowser Token") || strings.Contains(message, "reflected") {
		t.Fatalf("status error exposed upstream response body: %q", message)
	}
	if !strings.Contains(message, "HTTP 401") {
		t.Fatalf("status error = %q, want sanitized upstream status", message)
	}
}
