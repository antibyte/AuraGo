package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

// TestStorymakerConfigPatchOfTheConfigPageIsAcceptedAndRoundTrips sends the exact PUT the config page produces for the
// Storymaker block (data-path storymaker.enabled and storymaker.render.*), checks that the YAML on disk and the running
// configuration take the values, and that GET /api/config hands them back to the page. The rejecting paths are covered by
// TestStorymakerConfigValidationRejectsBadValuesBeforeWrite.
func TestStorymakerConfigPatchOfTheConfigPageIsAcceptedAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  host: 127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	if !cfg.Storymaker.Enabled || cfg.Storymaker.Render.TimeoutSeconds != 3600 || cfg.Storymaker.Render.MaxDurationSeconds != 900 ||
		cfg.Storymaker.Narration.MaxDailyChars != 100000 || cfg.Storymaker.Narration.MaxJobChars != 20000 {
		t.Fatalf("defaults of a config without the section: %+v", cfg.Storymaker)
	}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	patch := `{"storymaker":{"enabled":false,"render":{"timeout_seconds":7200,"max_duration_seconds":600,"browser_path":"","transport":"webp"},"narration":{"max_daily_chars":5000,"max_job_chars":1000}}}`
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body.String())
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"storymaker:", "enabled: false", "timeout_seconds: 7200", "max_duration_seconds: 600", "transport: webp", "max_daily_chars: 5000", "max_job_chars: 1000"} {
		if !strings.Contains(string(stored), want) {
			t.Errorf("config.yaml lacks %q:\n%s", want, stored)
		}
	}
	sm := s.ConfigSnapshot().Storymaker
	if sm.Enabled || sm.Render.TimeoutSeconds != 7200 || sm.Render.MaxDurationSeconds != 600 || sm.Render.Transport != "webp" ||
		sm.Narration.MaxDailyChars != 5000 || sm.Narration.MaxJobChars != 1000 {
		t.Errorf("the running configuration = %+v", sm)
	}

	// The page reads the section back with GET /api/config.
	rec = httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var got struct {
		Storymaker struct {
			Enabled bool `json:"enabled"`
			Render  struct {
				TimeoutSeconds     int    `json:"timeout_seconds"`
				MaxDurationSeconds int    `json:"max_duration_seconds"`
				Transport          string `json:"transport"`
			} `json:"render"`
			Narration struct {
				MaxDailyChars int `json:"max_daily_chars"`
				MaxJobChars   int `json:"max_job_chars"`
			} `json:"narration"`
		} `json:"storymaker"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %v\n%s", rec.Code, err, rec.Body.String())
	}
	if got.Storymaker.Enabled || got.Storymaker.Render.TimeoutSeconds != 7200 || got.Storymaker.Render.MaxDurationSeconds != 600 || got.Storymaker.Render.Transport != "webp" ||
		got.Storymaker.Narration.MaxDailyChars != 5000 || got.Storymaker.Narration.MaxJobChars != 1000 {
		t.Errorf("GET /api/config storymaker = %+v", got.Storymaker)
	}

	// The status endpoint follows the switch.
	off := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(off, httptest.NewRequest(http.MethodGet, storymakerAPIBase+"/status", nil))
	if off.Code != http.StatusOK || !strings.Contains(off.Body.String(), `"enabled":false`) {
		t.Errorf("status after the switch = %d %s", off.Code, off.Body.String())
	}
}

// 999 is one below the floor of max_daily_chars, so the page's PUT is refused before the file changes.
func TestStorymakerConfigRefusesANarrationCapOutsideItsRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := "server:\n  host: 127.0.0.1\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"storymaker":{"narration":{"max_daily_chars":999}}}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "max_daily_chars") {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body.String())
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != original {
		t.Fatalf("a refused patch changed the config file: %v\n%s", err, stored)
	}
}
