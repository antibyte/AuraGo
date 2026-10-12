package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

const configListPatchTestYAML = `a2a:
  server:
    skills:
      - id: search
        name: Search
        description: Finds things
        tags: [search, web]
      - id: notes
        name: Notes
        description: Keeps notes
        tags: [notes]
  client:
    remote_agents:
      - id: alpha
        name: Alpha
        card_url: https://alpha.example/.well-known/agent-card.json
        enabled: true
      - id: beta
        name: Beta
        card_url: https://beta.example/.well-known/agent-card.json
        enabled: true
three_d_printers:
  enabled: true
  elegoo_centauri_carbon:
    enabled: true
    printers:
      - id: lab
        name: Lab
        url: ws://lab.local/websocket
        timeout_seconds: 10
  klipper:
    enabled: true
    printers:
      - id: voron
        name: Voron
        url: http://voron.local:7125
        timeout_seconds: 10
      - id: ender
        name: Ender
        url: http://ender.local:7125
        timeout_seconds: 10
`

var configListPatchTestSecrets = map[string]string{
	"a2a_remote_alpha_api_key":                         "alpha-key",
	"a2a_remote_alpha_bearer_token":                    "alpha-bearer",
	"a2a_remote_beta_api_key":                          "beta-key",
	config.ThreeDPrinterKlipperAPIKeyVaultKey("voron"): "voron-key",
	config.ThreeDPrinterKlipperAPIKeyVaultKey("ender"): "ender-key",
}

func newConfigListPatchTestServer(t *testing.T) (*Server, string, *security.Vault) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configListPatchTestYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault(strings.Repeat("77", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	for key, value := range configListPatchTestSecrets {
		if err := vault.WriteSecret(key, value); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded.ConfigPath = configPath
	loaded.ApplyVaultSecrets(vault)
	return &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}, configPath, vault
}

func putConfigListPatch(t *testing.T, server *Server, payload string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handleUpdateConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
	return recorder
}

func readVaultSecret(t *testing.T, vault *security.Vault, key string) string {
	t.Helper()
	value, err := vault.ReadSecret(key)
	if err != nil {
		return ""
	}
	return value
}

// The config UI once sent only the edited fields of a list entry. The server
// replaces these lists wholesale, so such a list deleted every other entry and
// every field the cards do not render (skill tags).
func TestHandleUpdateConfigRejectsIncompleteConfigLists(t *testing.T) {
	lists := []struct {
		path  string
		patch func(value string) string
	}{
		{"a2a.server.skills", func(v string) string { return `{"a2a":{"server":{"skills":` + v + `}}}` }},
		{"a2a.client.remote_agents", func(v string) string { return `{"a2a":{"client":{"remote_agents":` + v + `}}}` }},
		{"three_d_printers.elegoo_centauri_carbon.printers", func(v string) string {
			return `{"three_d_printers":{"elegoo_centauri_carbon":{"printers":` + v + `}}}`
		}},
		{"three_d_printers.klipper.printers", func(v string) string { return `{"three_d_printers":{"klipper":{"printers":` + v + `}}}` }},
	}
	values := []struct {
		name  string
		value string
	}{
		{"field edit as sparse list", `[null,{"name":"Renamed"}]`},
		{"field edit as partial entry", `[{"name":"Renamed"}]`},
		{"entry that is not an object", `["search"]`},
		{"empty entry", `[{}]`},
		{"null list", `null`},
		{"numeric map with a gap", `{"1":{"name":"Renamed"}}`},
	}
	for _, list := range lists {
		for _, tc := range values {
			t.Run(list.path+"/"+tc.name, func(t *testing.T) {
				server, configPath, vault := newConfigListPatchTestServer(t)
				before, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("read config: %v", err)
				}
				recorder := putConfigListPatch(t, server, list.patch(tc.value))

				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
				}
				if !strings.Contains(recorder.Body.String(), list.path) {
					t.Fatalf("rejection does not name %s: %s", list.path, recorder.Body.String())
				}
				after, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("read config after save: %v", err)
				}
				if !bytes.Equal(before, after) {
					t.Fatalf("rejected save changed config.yaml:\n%s", after)
				}
				for key, want := range configListPatchTestSecrets {
					if got := readVaultSecret(t, vault, key); got != want {
						t.Fatalf("vault %s = %q, want %q", key, got, want)
					}
				}
			})
		}
	}
}

// Complete lists, as the config UI sends them, keep unrendered fields (skill
// tags) and never write the masked credential placeholders the server reports
// for remote agents into config.yaml or the Vault.
func TestHandleUpdateConfigSavesCompleteA2ALists(t *testing.T) {
	server, configPath, vault := newConfigListPatchTestServer(t)
	payload := `{"a2a":{"server":{"skills":[` +
		`{"id":"notes","name":"Notebook","description":"Keeps notes","tags":["notes"]},` +
		`{"id":"lights","name":"Lights","description":"","tags":[]}]},` +
		`"client":{"remote_agents":[` +
		`{"id":"alpha","name":"Alpha","card_url":"https://alpha.example/card.json","enabled":true,"api_key":"••••••••","bearer_token":"••••••••"},` +
		`{"id":"beta","name":"Beta","card_url":"https://beta.example/.well-known/agent-card.json","enabled":false,"api_key":"beta-rotated","bearer_token":""}]}}}`
	recorder := putConfigListPatch(t, server, payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, leaked := range []string{"••••••••", "beta-rotated", "api_key", "bearer_token"} {
		if strings.Contains(string(data), leaked) {
			t.Fatalf("config.yaml contains %q:\n%s", leaked, data)
		}
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	var skills []string
	for _, skill := range saved.A2A.Server.Skills {
		skills = append(skills, fmt.Sprintf("%s=%s%v", skill.ID, skill.Name, skill.Tags))
	}
	if got, want := strings.Join(skills, ","), "notes=Notebook[notes],lights=Lights[]"; got != want {
		t.Fatalf("saved skills = %s, want %s", got, want)
	}
	if got := len(saved.A2A.Client.RemoteAgents); got != 2 || saved.A2A.Client.RemoteAgents[1].Enabled {
		t.Fatalf("saved remote agents = %#v", saved.A2A.Client.RemoteAgents)
	}
	for key, want := range map[string]string{
		"a2a_remote_alpha_api_key":      "alpha-key",
		"a2a_remote_alpha_bearer_token": "alpha-bearer",
		"a2a_remote_beta_api_key":       "beta-rotated",
	} {
		if got := readVaultSecret(t, vault, key); got != want {
			t.Fatalf("vault %s = %q, want %q", key, got, want)
		}
	}
}

// Removing the first Klipper printer moves the others up one index. That is
// not a rename: each remaining printer keeps its own API key.
func TestHandleUpdateConfigKeepsKlipperKeysWhenPrinterRemoved(t *testing.T) {
	server, configPath, vault := newConfigListPatchTestServer(t)
	payload := `{"three_d_printers":{"klipper":{"printers":[` +
		`{"id":"ender","name":"Ender","url":"http://ender.local:7125","api_key":"••••••••","timeout_seconds":10,"webcam_name":""}]}}}`
	recorder := putConfigListPatch(t, server, payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	if got := readVaultSecret(t, vault, config.ThreeDPrinterKlipperAPIKeyVaultKey("ender")); got != "ender-key" {
		t.Fatalf("ender API key = %q, want ender-key", got)
	}
	if got := readVaultSecret(t, vault, config.ThreeDPrinterKlipperAPIKeyVaultKey("voron")); got != "" {
		t.Fatalf("removed voron API key still stored: %q", got)
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if got := len(saved.ThreeDPrinters.Klipper.Printers); got != 1 || saved.ThreeDPrinters.Klipper.Printers[0].ID != "ender" {
		t.Fatalf("saved klipper printers = %#v", saved.ThreeDPrinters.Klipper.Printers)
	}
}

// Renaming a printer in place still carries its stored API key to the new id.
func TestHandleUpdateConfigCarriesKlipperKeyToRenamedPrinter(t *testing.T) {
	server, _, vault := newConfigListPatchTestServer(t)
	payload := `{"three_d_printers":{"klipper":{"printers":[` +
		`{"id":"voron-24","name":"Voron","url":"http://voron.local:7125","api_key":"••••••••","timeout_seconds":10},` +
		`{"id":"ender","name":"Ender","url":"http://ender.local:7125","api_key":"••••••••","timeout_seconds":10}]}}}`
	recorder := putConfigListPatch(t, server, payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	for id, want := range map[string]string{"voron-24": "voron-key", "ender": "ender-key"} {
		if got := readVaultSecret(t, vault, config.ThreeDPrinterKlipperAPIKeyVaultKey(id)); got != want {
			t.Fatalf("%s API key = %q, want %q", id, got, want)
		}
	}
}
