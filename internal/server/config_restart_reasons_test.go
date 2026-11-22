package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func newConfigSaveTestServer(t *testing.T, yaml string) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	return &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}
}

func saveConfigPatch(t *testing.T, s *Server, patch string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch))
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body=%s", err, rec.Body.String())
	}
	return resp
}

func jsonStrings(t *testing.T, resp map[string]any, field string) []string {
	t.Helper()
	raw, ok := resp[field].([]any)
	if !ok {
		t.Fatalf("%s = %#v, want a JSON array", field, resp[field])
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("%s contains %#v, want strings only", field, v)
		}
		out = append(out, s)
	}
	return out
}

func TestHandleUpdateConfigReportsRestartReasonIDs(t *testing.T) {
	s := newConfigSaveTestServer(t, "agent:\n  max_concurrent_loops: 4\ntools:\n  skill_manager:\n    enabled: false\n")

	resp := saveConfigPatch(t, s, `{"agent":{"max_concurrent_loops":6},"tools":{"skill_manager":{"enabled":true}}}`)

	if resp["needs_restart"] != true {
		t.Fatalf("needs_restart = %v, want true; resp=%v", resp["needs_restart"], resp)
	}
	if got, want := jsonStrings(t, resp, "restart_reasons"), []string{"agent_concurrency", "skill_manager"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("restart_reasons = %v, want %v", got, want)
	}
	// restart_reason keeps its legacy shape: English labels in the same order.
	if got, want := jsonStrings(t, resp, "restart_reason"), []string{"Agent concurrency", "Skill Manager"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("restart_reason = %v, want %v", got, want)
	}
	msg, _ := resp["message"].(string)
	if msg != "Saved. Restart required for: Agent concurrency, Skill Manager" {
		t.Fatalf("message = %q, want the English fallback naming both reasons", msg)
	}
}

func TestHandleUpdateConfigHotReloadAnswerIsEnglishAndHasNoReasons(t *testing.T) {
	s := newConfigSaveTestServer(t, "discord:\n  enabled: false\n")

	resp := saveConfigPatch(t, s, `{"discord":{"enabled":true,"allowed_user_id":"1234","default_channel_id":"5678"}}`)

	if resp["needs_restart"] != false {
		t.Fatalf("needs_restart = %v, want false; resp=%v", resp["needs_restart"], resp)
	}
	if _, ok := resp["restart_reasons"]; ok {
		t.Fatalf("restart_reasons present on a hot reload: %v", resp)
	}
	if msg, _ := resp["message"].(string); msg != "Configuration saved and applied." {
		t.Fatalf("message = %q, want the English fallback", msg)
	}
}

func TestConfigRestartReasonsAreStableAndTranslated(t *testing.T) {
	if len(configRestartReasons) == 0 {
		t.Fatal("no restart reasons registered")
	}
	idPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	seen := map[string]bool{}
	required := []string{
		"config.save_bar.saved_applied",
		"config.save_bar.restart_needed_for",
	}
	for _, reason := range configRestartReasons {
		if !idPattern.MatchString(reason.ID) {
			t.Fatalf("restart reason id %q is not a stable snake_case id", reason.ID)
		}
		if seen[reason.ID] {
			t.Fatalf("restart reason id %q registered twice", reason.ID)
		}
		seen[reason.ID] = true
		if strings.TrimSpace(reason.Label) == "" {
			t.Fatalf("restart reason %q has no English label", reason.ID)
		}
		required = append(required, "config.save_bar.restart_reason."+reason.ID)
	}
	commonDir := filepath.Join("..", "..", "ui", "lang", "config", "common")
	assertLocaleKeysExist(t, commonDir, required)

	// The sentence must keep its placeholder in every locale.
	entries, err := os.ReadDir(commonDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", commonDir, err)
	}
	locales := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		locales++
		data, err := os.ReadFile(filepath.Join(commonDir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", entry.Name(), err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("Unmarshal(%s): %v", entry.Name(), err)
		}
		if !strings.Contains(values["config.save_bar.restart_needed_for"], "{{reasons}}") {
			t.Fatalf("%s: config.save_bar.restart_needed_for lacks {{reasons}}", entry.Name())
		}
	}
	if locales != 16 {
		t.Fatalf("found %d config/common locales, want 16", locales)
	}
}
