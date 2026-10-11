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
	"aurago/internal/security"

	"gopkg.in/yaml.v3"
)

// The config UI must show the effective virtual_desktop.retronet_enabled: a
// config without the key loads Retro-Net on, and showing the absence as "off"
// would make the next Virtual Desktop save write false. Written booleans stay.
func TestInjectRetroNetDefaultExposesEffectiveGrant(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		want       bool
	}{
		{"key missing", "virtual_desktop:\n  enabled: true\n", true},
		{"section missing", "server:\n  port: 8088\n", true},
		{"null section", "virtual_desktop:\n", true},
		{"null key", "virtual_desktop:\n  enabled: true\n  retronet_enabled:\n", true},
		{"explicit false", "virtual_desktop:\n  enabled: true\n  retronet_enabled: false\n", false},
		{"explicit true", "virtual_desktop:\n  enabled: true\n  retronet_enabled: true\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			var raw map[string]interface{}
			if err := yaml.Unmarshal([]byte(tc.yaml), &raw); err != nil {
				t.Fatal(err)
			}
			injectRetroNetDefault(raw, cfg)
			section, ok := raw["virtual_desktop"].(map[string]interface{})
			if !ok {
				t.Fatalf("virtual_desktop section = %#v", raw["virtual_desktop"])
			}
			if got, ok := section["retronet_enabled"].(bool); !ok || got != tc.want || got != cfg.VirtualDesktop.RetroNetEnabled {
				t.Fatalf("virtual_desktop.retronet_enabled = %#v, want %v (loaded %v)", section["retronet_enabled"], tc.want, cfg.VirtualDesktop.RetroNetEnabled)
			}
			if strings.Contains(tc.yaml, "enabled: true\n") && section["enabled"] != true {
				t.Fatalf("inject dropped existing virtual_desktop keys: %#v", section)
			}
		})
	}
}

// GET shows the effective value and PUT round-trips keep it: a missing key
// stays on through unrelated saves, a Virtual Desktop save and a save that
// echoes the shown value; an explicit false stays off through the same saves.
func TestConfigRetroNetDefaultSurvivesGetAndPut(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		want       bool
	}{
		{"key missing", "virtual_desktop:\n  enabled: true\n  max_ws_clients: 8\nbudget: {enabled: false, daily_limit_usd: 5}\n", true},
		{"explicit false", "virtual_desktop:\n  enabled: true\n  max_ws_clients: 8\n  retronet_enabled: false\nbudget: {enabled: false, daily_limit_usd: 5}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConfigPath = path
			vault, err := security.NewVault(strings.Repeat("5a", 32), filepath.Join(root, "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}

			shown := func(step string) {
				t.Helper()
				rec := httptest.NewRecorder()
				handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: GET status %d: %s", step, rec.Code, rec.Body.String())
				}
				var body struct {
					VirtualDesktop map[string]interface{} `json:"virtual_desktop"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.VirtualDesktop["retronet_enabled"] != tc.want {
					t.Fatalf("%s: GET virtual_desktop.retronet_enabled = %#v, want %v", step, body.VirtualDesktop["retronet_enabled"], tc.want)
				}
			}
			shown("initial")

			echo := `{"virtual_desktop":{"retronet_enabled":false}}`
			if tc.want {
				echo = `{"virtual_desktop":{"retronet_enabled":true}}`
			}
			for _, patch := range []string{
				`{"budget":{"daily_limit_usd":7}}`,
				`{"virtual_desktop":{"max_ws_clients":9}}`,
				echo,
			} {
				put := httptest.NewRecorder()
				handleUpdateConfig(s).ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
				if put.Code != http.StatusOK {
					t.Fatalf("PUT %s: %d %s", patch, put.Code, put.Body.String())
				}
				after, err := config.Load(path)
				if err != nil {
					t.Fatal(err)
				}
				if after.VirtualDesktop.RetroNetEnabled != tc.want {
					t.Fatalf("PUT %s: file loads retronet_enabled=%v, want %v", patch, after.VirtualDesktop.RetroNetEnabled, tc.want)
				}
				if live := s.ConfigSnapshot().VirtualDesktop.RetroNetEnabled; live != tc.want {
					t.Fatalf("PUT %s: live config retronet_enabled=%v, want %v", patch, live, tc.want)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if tc.want && strings.Contains(string(data), "retronet_enabled: false") {
					t.Fatalf("PUT %s wrote retronet_enabled: false:\n%s", patch, data)
				}
				if !tc.want && !strings.Contains(string(data), "retronet_enabled: false") {
					t.Fatalf("PUT %s dropped the explicit false:\n%s", patch, data)
				}
				shown("after PUT " + patch)
			}
			after, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if after.VirtualDesktop.MaxWSClients != 9 || after.Budget.DailyLimitUSD != 7 {
				t.Fatalf("saves not applied: max_ws_clients=%d daily_limit=%v", after.VirtualDesktop.MaxWSClients, after.Budget.DailyLimitUSD)
			}
		})
	}
}
