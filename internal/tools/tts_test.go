package tools

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTTSSynthesizeCachedHitCleansExpiredFiles(t *testing.T) {
	dataDir := t.TempDir()
	ttsDir := filepath.Join(dataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		t.Fatalf("mkdir tts dir: %v", err)
	}

	cfg := TTSConfig{
		Provider:            "google",
		Language:            "en",
		DataDir:             dataDir,
		CacheRetentionHours: 1,
	}
	filename := fmt.Sprintf("%x.mp3", md5.Sum([]byte(cfg.Provider+cfg.Language+"hello")))
	if err := os.WriteFile(filepath.Join(ttsDir, filename), []byte("cached"), 0o644); err != nil {
		t.Fatalf("write cached file: %v", err)
	}
	stalePath := filepath.Join(ttsDir, "stale.mp3")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stalePath, old, old); err != nil {
		t.Fatalf("set stale mtime: %v", err)
	}

	got, err := TTSSynthesize(cfg, "hello")
	if err != nil {
		t.Fatalf("TTSSynthesize: %v", err)
	}
	if got != filename {
		t.Fatalf("filename = %q, want %q", got, filename)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed, stat err = %v", err)
	}
}

func TestCleanupTTSCachePrunesOldestFilesBeyondLimit(t *testing.T) {
	dataDir := t.TempDir()
	ttsDir := filepath.Join(dataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		t.Fatalf("mkdir tts dir: %v", err)
	}

	now := time.Now()
	files := []struct {
		name string
		when time.Time
	}{
		{name: "oldest.mp3", when: now.Add(-3 * time.Hour)},
		{name: "middle.mp3", when: now.Add(-2 * time.Hour)},
		{name: "keep.mp3", when: now.Add(-1 * time.Hour)},
	}
	for _, file := range files {
		path := filepath.Join(ttsDir, file.name)
		if err := os.WriteFile(path, []byte(file.name), 0o644); err != nil {
			t.Fatalf("write %s: %v", file.name, err)
		}
		if err := os.Chtimes(path, file.when, file.when); err != nil {
			t.Fatalf("set mtime %s: %v", file.name, err)
		}
	}

	err := cleanupTTSCache(TTSConfig{DataDir: dataDir, CacheMaxFiles: 2}, "keep.mp3", now)
	if err != nil {
		t.Fatalf("cleanupTTSCache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ttsDir, "oldest.mp3")); !os.IsNotExist(err) {
		t.Fatalf("expected oldest file to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(ttsDir, "middle.mp3")); err != nil {
		t.Fatalf("expected middle file to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ttsDir, "keep.mp3")); err != nil {
		t.Fatalf("expected keep file to remain: %v", err)
	}
}

func TestTTSGoogleRejectsOversizedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	withTTSTestClient(t, server, func() {
		_, err := ttsGoogle("hello", "en")
		if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
			t.Fatalf("expected oversized response error, got %v", err)
		}
	})
}

func TestTTSElevenLabsRejectsOversizedErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("b", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	withTTSTestClient(t, server, func() {
		_, err := ttsElevenLabs(TTSConfig{ElevenLabs: struct {
			APIKey  string
			VoiceID string
			ModelID string
		}{APIKey: "test-key"}}, "hello")
		if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
			t.Fatalf("expected oversized error response, got %v", err)
		}
	})
}

func withTTSTestClient(t *testing.T, server *httptest.Server, fn func()) {
	t.Helper()
	oldClient := ttsHTTPClient
	t.Cleanup(func() { ttsHTTPClient = oldClient })

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}

	client := server.Client()
	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = serverURL.Scheme
		clone.URL.Host = serverURL.Host
		clone.Host = serverURL.Host
		return baseTransport.RoundTrip(clone)
	})
	ttsHTTPClient = client
	fn()
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
