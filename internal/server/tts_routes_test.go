package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMainTTSRouteServesSupertonicAudioContentTypes(t *testing.T) {
	dataDir := t.TempDir()
	ttsDir := filepath.Join(dataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		t.Fatalf("mkdir tts dir: %v", err)
	}
	for _, filename := range []string{"voice.ogg", "voice.flac"} {
		if err := os.WriteFile(filepath.Join(ttsDir, filename), []byte("audio-data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	if _, err := s.registerUIRoutes(mux, make(chan struct{})); err != nil {
		t.Fatalf("registerUIRoutes: %v", err)
	}

	tests := []struct {
		path        string
		contentType string
	}{
		{path: "/tts/voice.ogg", contentType: "audio/ogg"},
		{path: "/tts/voice.flac", contentType: "audio/flac"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q; want 200", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, tt.contentType) {
				t.Fatalf("Content-Type = %q, want %s", ct, tt.contentType)
			}
		})
	}
}

func TestMainCastMediaRouteServesVideoContentType(t *testing.T) {
	dataDir := t.TempDir()
	castMediaDir := filepath.Join(dataDir, "cast_media")
	if err := os.MkdirAll(castMediaDir, 0o755); err != nil {
		t.Fatalf("mkdir cast media dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(castMediaDir, "clip.mp4"), []byte("video-data"), 0o644); err != nil {
		t.Fatalf("write clip.mp4: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	if _, err := s.registerUIRoutes(mux, make(chan struct{})); err != nil {
		t.Fatalf("registerUIRoutes: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/cast-media/clip.mp4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q; want 200", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "video/mp4") {
		t.Fatalf("Content-Type = %q, want video/mp4", ct)
	}
}

func TestDedicatedChromecastMediaServerServesTTSAndCastMedia(t *testing.T) {
	dataDir := t.TempDir()
	ttsDir := filepath.Join(dataDir, "tts")
	castMediaDir := filepath.Join(dataDir, "cast_media")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		t.Fatalf("mkdir tts dir: %v", err)
	}
	if err := os.MkdirAll(castMediaDir, 0o755); err != nil {
		t.Fatalf("mkdir cast media dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ttsDir, "voice.mp3"), []byte("audio-data"), 0o644); err != nil {
		t.Fatalf("write voice.mp3: %v", err)
	}
	if err := os.WriteFile(filepath.Join(castMediaDir, "clip.mp4"), []byte("video-data"), 0o644); err != nil {
		t.Fatalf("write clip.mp4: %v", err)
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Server.Host = "127.0.0.1"
	cfg.Chromecast.Enabled = true
	cfg.Chromecast.TTSPort = port
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	mediaServer, err := s.registerUIRoutes(mux, make(chan struct{}))
	if err != nil {
		t.Fatalf("registerUIRoutes: %v", err)
	}
	if mediaServer == nil {
		t.Fatal("expected dedicated Chromecast media server")
	}
	defer mediaServer.Shutdown(context.Background())

	client := &http.Client{Timeout: 500 * time.Millisecond}
	assertDedicatedMedia := func(path, wantContentType string) {
		t.Helper()
		url := "http://127.0.0.1:" + mediaServer.Addr[strings.LastIndex(mediaServer.Addr, ":")+1:] + path
		var resp *http.Response
		var err error
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
			resp, err = client.Get(url)
			if err == nil {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, wantContentType) {
			t.Fatalf("GET %s Content-Type = %q, want %s", path, ct, wantContentType)
		}
	}

	assertDedicatedMedia("/tts/voice.mp3", "audio/mpeg")
	assertDedicatedMedia("/cast-media/clip.mp4", "video/mp4")
}

func TestChromecastMediaServerBindHostUsesLANCapableDefault(t *testing.T) {
	tests := map[string]string{
		"":             "0.0.0.0",
		"0.0.0.0":      "0.0.0.0",
		"127.0.0.1":    "0.0.0.0",
		"localhost":    "0.0.0.0",
		"::1":          "0.0.0.0",
		"192.168.1.20": "192.168.1.20",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := chromecastMediaServerBindHost(input); got != want {
				t.Fatalf("chromecastMediaServerBindHost(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
