package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
