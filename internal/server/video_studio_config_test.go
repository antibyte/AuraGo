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

func TestVideoStudioConfigRejectsInvalidLimitsBeforeWrite(t *testing.T) {
	for _, setup := range []bool{false, true} {
		name := "config"
		if setup {
			name = "setup"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			original := []byte("server:\n  host: 127.0.0.1\nvideo_studio:\n  enabled: false\n")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConfigPath = path
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			if setup {
				_, err := applyConfigPatch(s, map[string]interface{}{"video_studio": map[string]interface{}{"max_asset_size_mb": -1}})
				if err == nil || !strings.Contains(err.Error(), "video_studio:") {
					t.Fatalf("invalid limit validation: %v", err)
				}
			} else {
				rec := httptest.NewRecorder()
				handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"video_studio":{"max_asset_size_mb":-1}}`)))
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "video_studio:") {
					t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
			}
			stored, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(stored) != string(original) {
				t.Fatal("invalid config changed persisted file")
			}
		})
	}
}
