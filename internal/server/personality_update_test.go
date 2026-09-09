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
	"gopkg.in/yaml.v3"
)

func TestPersonalityUpdatePublishesOnlyAfterSave(t *testing.T) {
	for _, persist := range []bool{false, true} {
		name := "save-failure"
		if persist {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if persist {
				if err := os.WriteFile(path, []byte("personality:\n  core_personality: friend\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cfg := &config.Config{ConfigPath: path}
			cfg.Directories.PromptsDir = t.TempDir()
			cfg.Personality.CorePersonality = "friend"
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			s.initConfigSnapshot()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/personality", strings.NewReader(`{"id":"punk"}`))
			handleUpdatePersonality(s)(rec, req)
			if cfg.Personality.CorePersonality != "friend" {
				t.Fatal("mutated the previous runtime snapshot")
			}
			if !persist {
				if rec.Code != http.StatusInternalServerError || s.ConfigSnapshot() != cfg {
					t.Fatalf("failed save published a persona: status=%d", rec.Code)
				}
				return
			}
			if rec.Code != http.StatusOK || s.ConfigSnapshot().Personality.CorePersonality != "punk" || s.Cfg != s.ConfigSnapshot() {
				t.Fatalf("saved persona was not published: status=%d", rec.Code)
			}
			var response map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response["active"] != "punk" {
				t.Fatalf("unexpected response: %s", rec.Body.String())
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var saved config.Config
			if err := yaml.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Personality.CorePersonality != "punk" {
				t.Fatal("saved persona differs from runtime")
			}
		})
	}
}

// Test both first-time creation and rejection without destroying a working profile.
func TestPersonalityFileSaveValidation(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		existing      bool
		status        int
	}{
		{"first-profile", "---\nid: custom\nmeta:\n  volatility: 0\n---\nA distinctive custom voice.", false, http.StatusOK},
		{"plain-profile", "A plain custom voice.", false, http.StatusOK},
		{"malformed-frontmatter", "---\nid: [broken\n---\nBroken override", true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Directories.PromptsDir = t.TempDir()
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			path := filepath.Join(cfg.Directories.PromptsDir, "personalities", "custom.md")
			const original = "Keep this working profile."
			if tc.existing {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(original), 0644); err != nil {
					t.Fatal(err)
				}
			}
			payload, err := json.Marshal(map[string]string{"name": "custom", "content": tc.content})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			handleSavePersonalityFile(s)(rec, httptest.NewRequest(http.MethodPost, "/api/config/personality-files", strings.NewReader(string(payload))))
			if rec.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.content
			if tc.existing {
				want = original
			}
			if string(data) != want {
				t.Fatal("profile contents differ from the accepted version")
			}
		})
	}
}
