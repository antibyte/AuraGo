package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTTSUnknownProviderMakesNoHTTPCall(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()

	withTTSTestClient(t, server, func() {
		_, err := TTSSynthesize(TTSConfig{Provider: "not-a-provider", DataDir: t.TempDir(), Language: "de"}, "hallo")
		if err == nil || !strings.Contains(err.Error(), "unsupported TTS provider") {
			t.Fatalf("error = %v", err)
		}
		_, _, err = TTSSynthesizeInMemoryContext(t.Context(), TTSConfig{Provider: "not-a-provider", Language: "de"}, "hallo")
		if err == nil || !strings.Contains(err.Error(), "unsupported TTS provider") {
			t.Fatalf("memory error = %v", err)
		}
	})
	if calls.Load() != 0 {
		t.Fatalf("http calls = %d", calls.Load())
	}
}

func TestTTSEmptyAudioIsNotCached(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	dataDir := t.TempDir()
	cfg := TTSConfig{Provider: "google", DataDir: dataDir, Language: "de"}
	withTTSTestClient(t, server, func() {
		for i := 0; i < 2; i++ {
			_, err := TTSSynthesize(cfg, "hallo")
			if err == nil || !strings.Contains(err.Error(), "TTS provider returned empty audio") {
				t.Fatalf("call %d error = %v", i, err)
			}
		}
	})
	if calls.Load() != 2 {
		t.Fatalf("http calls = %d, want 2", calls.Load())
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "tts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cache entries = %d", len(entries))
	}
}

func TestTTSEmptyCacheFileIsSynthesizedAgain(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "audio")
	}))
	defer server.Close()
	dataDir := t.TempDir()
	cfg := TTSConfig{Provider: "google", DataDir: dataDir, Language: "de"}
	withTTSTestClient(t, server, func() {
		filename, err := TTSSynthesize(cfg, "hallo")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dataDir, "tts", filename)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		second, err := TTSSynthesize(cfg, "hallo")
		if err != nil || second != filename {
			t.Fatalf("second = %q, err = %v", second, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("cached file = %v, err = %v", info, err)
		}
	})
	if calls.Load() != 2 {
		t.Fatalf("http calls = %d, want 2", calls.Load())
	}
}

func TestTTSGoogleFilePathSends200Runes(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("q")
		_, _ = io.WriteString(w, "audio")
	}))
	defer server.Close()
	text := strings.Repeat("ä", 500)
	var meta TTSSynthesisMeta
	withTTSTestClient(t, server, func() {
		_, err := TTSSynthesize(TTSConfig{Provider: "google", DataDir: t.TempDir(), Language: "de", Result: &meta}, text)
		if err != nil {
			t.Fatal(err)
		}
	})
	if got := len([]rune(query)); got != 200 {
		t.Fatalf("google query runes = %d, want 200", got)
	}
	if meta.Truncated {
		t.Fatal("exactly 500 runes must not set truncated")
	}
}

func TestTTSMemoryPathKeeps600Runes(t *testing.T) {
	var got int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		got = len([]rune(body.Text))
		_, _ = io.WriteString(w, "audio")
	}))
	defer server.Close()
	cfg := TTSConfig{Provider: "supertonic", Language: "de"}
	cfg.Supertonic.URL = server.URL
	text := strings.Repeat("ä", 600)
	data, _, err := TTSSynthesizeInMemoryContext(t.Context(), cfg, text)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || got != 600 {
		t.Fatalf("audio=%d runes=%d", len(data), got)
	}
}
