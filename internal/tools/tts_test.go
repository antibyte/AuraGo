package tools

import (
	"aurago/internal/testutil"
	"crypto/md5"
	"encoding/json"
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
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestTTSSupertonicSendsNativeRequest(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotPayload map[string]interface{}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("RIFF"))
	}))
	defer server.Close()

	data, err := ttsSupertonic(TTSConfig{
		Language: "de",
		Supertonic: struct {
			URL            string
			Model          string
			Voice          string
			Speed          float64
			Steps          int
			ResponseFormat string
		}{
			URL:            server.URL,
			Model:          "supertonic-3",
			Voice:          "F1",
			Speed:          1.2,
			Steps:          10,
			ResponseFormat: "wav",
		},
	}, "Hallo")
	if err != nil {
		t.Fatalf("ttsSupertonic: %v", err)
	}
	if string(data) != "RIFF" {
		t.Fatalf("audio data = %q, want RIFF", string(data))
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/tts" {
		t.Fatalf("path = %q, want /v1/tts", gotPath)
	}
	want := map[string]interface{}{
		"text":            "Hallo",
		"voice":           "F1",
		"lang":            "de",
		"speed":           1.2,
		"steps":           float64(10),
		"response_format": "wav",
	}
	for key, expected := range want {
		if gotPayload[key] != expected {
			t.Fatalf("payload[%s] = %#v, want %#v (payload: %#v)", key, gotPayload[key], expected, gotPayload)
		}
	}
}

func TestTTSSupertonicParsesErrorEnvelope(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"unknown voice","type":"invalid_request_error","code":"unknown_voice"}}`))
	}))
	defer server.Close()

	_, err := ttsSupertonic(TTSConfig{
		Supertonic: struct {
			URL            string
			Model          string
			Voice          string
			Speed          float64
			Steps          int
			ResponseFormat string
		}{URL: server.URL},
	}, "hello")
	if err == nil || !strings.Contains(err.Error(), "unknown_voice") || !strings.Contains(err.Error(), "unknown voice") {
		t.Fatalf("expected Supertonic envelope error, got %v", err)
	}
}

func TestTTSSynthesizeSupertonicUsesFormatExtensionAndVoiceCacheKey(t *testing.T) {
	dataDir := t.TempDir()
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Voice string `json:"voice"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "audio/ogg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio-" + payload.Voice))
	}))
	defer server.Close()

	cfg := TTSConfig{
		Provider: "supertonic",
		Language: "en",
		DataDir:  dataDir,
		Supertonic: struct {
			URL            string
			Model          string
			Voice          string
			Speed          float64
			Steps          int
			ResponseFormat string
		}{
			URL:            server.URL,
			Model:          "supertonic-3",
			Voice:          "M1",
			Speed:          1,
			Steps:          8,
			ResponseFormat: "ogg",
		},
	}

	first, err := TTSSynthesize(cfg, "same text")
	if err != nil {
		t.Fatalf("first TTSSynthesize: %v", err)
	}
	if !strings.HasSuffix(first, ".ogg") {
		t.Fatalf("first filename = %q, want .ogg suffix", first)
	}
	firstData, err := os.ReadFile(filepath.Join(dataDir, "tts", first))
	if err != nil {
		t.Fatalf("read first file: %v", err)
	}
	if string(firstData) != "audio-M1" {
		t.Fatalf("first audio = %q, want audio-M1", string(firstData))
	}

	cfg.Supertonic.Voice = "F1"
	second, err := TTSSynthesize(cfg, "same text")
	if err != nil {
		t.Fatalf("second TTSSynthesize: %v", err)
	}
	if second == first {
		t.Fatalf("voice change reused cache filename %q", first)
	}
	secondData, err := os.ReadFile(filepath.Join(dataDir, "tts", second))
	if err != nil {
		t.Fatalf("read second file: %v", err)
	}
	if string(secondData) != "audio-F1" {
		t.Fatalf("second audio = %q, want audio-F1", string(secondData))
	}
}

func TestMiniMaxTTSModelIDNormalizesLegacySpeech02HD(t *testing.T) {
	got := miniMaxTTSModelForAPI("speech-02-hd")
	if got != "speech-2.8-hd" {
		t.Fatalf("miniMaxTTSModelForAPI = %q, want speech-2.8-hd", got)
	}
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
