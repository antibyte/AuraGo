package server

import (
	"encoding/json"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/detective"
)

// The configured path wins, and the legacy location next to the Game Maker database only
// survives while that file is already there (audit 4.20).
func TestDetectivePathDefaultsAndOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.SQLite.DetectivePath = "./data/detective.db"
	if got := detectiveStorePath(cfg); got != "./data/detective.db" {
		t.Fatalf("configured path = %q", got)
	}
	cfg.SQLite.DetectivePath = "  ./custom/research.db  "
	if got := detectiveStorePath(cfg); got != "./custom/research.db" {
		t.Fatalf("padded path = %q", got)
	}
}

func TestDetectivePathKeepsLegacyLocation(t *testing.T) {
	dir := t.TempDir()
	gameMaker := filepath.Join(dir, "game_maker.db")
	legacy := filepath.Join(dir, "detective.db")
	cfg := &config.Config{}
	cfg.SQLite.GameMakerPath = gameMaker

	// Without the legacy file the service uses the data directory.
	if got := detectiveStorePath(cfg); got != "data/detective.db" {
		t.Fatalf("fresh install path = %q", got)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectiveStorePath(cfg); got != legacy {
		t.Fatalf("legacy path = %q, want %q", got, legacy)
	}
}

// A store failure becomes a stable code; the raw error stays out of the API.
func TestDetectiveInitializationErrorCodes(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{nil, ""},
		{detective.ErrSchemaTooNew, "schema_too_new"},
		{fs.ErrPermission, "permission_denied"},
		{fs.ErrNotExist, "path_missing"},
		{errString("disk I/O error"), "store_failed"},
	}
	for _, c := range cases {
		if got := detectiveInitCode(c.err); got != c.code {
			t.Errorf("%v: code %q, want %q", c.err, got, c.code)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// The allowlist preview names the tools and the configured operations; it works for an
// administrator even while the integration is disabled (audit 4.16).
func TestDetectiveReadOperationsEndpoint(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = false
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Detective.ExtraReadOperations = map[string][]string{"mcp_call": {"archive/read"}}
	s := &Server{Cfg: cfg}
	read := func() map[string]any {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/desktop/detective/read-operations", nil)
		w := httptest.NewRecorder()
		s.handleDetective(w, r)
		if w.Code != 200 {
			t.Fatalf("read-operations: %d %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	body := read()
	tools, _ := body["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("no tools: %v", body)
	}
	found := false
	for _, name := range tools {
		if name == "detective_report" {
			found = true
		}
	}
	if !found {
		t.Fatalf("detective_report missing: %v", tools)
	}
	freeForm, _ := body["free_form"].([]any)
	if len(freeForm) != 2 {
		t.Fatalf("free_form = %v", freeForm)
	}
	configured, _ := body["configured"].(map[string]any)
	if _, ok := configured["mcp_call"]; !ok {
		t.Fatalf("configured = %v", configured)
	}
	// Disabling the integration keeps the preview readable.
	cfg.Detective.Enabled = false
	if body = read(); body["tools"] == nil {
		t.Fatalf("disabled integration lost the tools: %v", body)
	}
}

// The default path only falls back to the legacy store when nothing would be lost: the
// configured store does not exist yet and an older one sits beside Game Maker.
func TestDetectivePathKeepsLegacyAfterUpgrade(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "detective.db")
	cfg := &config.Config{}
	cfg.SQLite.GameMakerPath = filepath.Join(dir, "game_maker.db")
	cfg.SQLite.DetectivePath = filepath.Join(dir, "data", "detective.db")
	cfg.SQLite.DetectivePathDefaulted = true
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectiveStorePath(cfg); got != legacy {
		t.Fatalf("upgrade path = %q, want %q", got, legacy)
	}
	// Once the new store exists the default stays authoritative.
	if err := os.MkdirAll(filepath.Dir(cfg.SQLite.DetectivePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.SQLite.DetectivePath, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectiveStorePath(cfg); got != cfg.SQLite.DetectivePath {
		t.Fatalf("existing store path = %q", got)
	}
	// An explicit choice never redirects, even while the legacy file exists.
	cfg.SQLite.DetectivePathDefaulted = false
	os.Remove(cfg.SQLite.DetectivePath)
	if got := detectiveStorePath(cfg); got != cfg.SQLite.DetectivePath {
		t.Fatalf("explicit path = %q", got)
	}
}

// A Config save normalizes the allowlist before it is written to YAML.
func TestNormalizeDetectiveYAMLSection(t *testing.T) {
	raw := map[string]interface{}{"detective": map[string]interface{}{
		"extra_read_operations": map[string]interface{}{
			" mcp_call ":  []interface{}{" archive/read ", "", "archive/read"},
			"empty_tool":  []interface{}{},
			"   ":         []interface{}{"orphan"},
			"web_scraper": []interface{}{"read"},
		},
	}}
	normalizeDetectiveYAMLSection(raw)
	section := raw["detective"].(map[string]interface{})
	entries := section["extra_read_operations"].(map[string]interface{})
	if len(entries) != 2 {
		t.Fatalf("tools = %v", entries)
	}
	if ops := entries["mcp_call"].([]interface{}); len(ops) != 1 || ops[0] != "archive/read" {
		t.Fatalf("mcp_call = %v", ops)
	}
	// A malformed value becomes an empty map instead of blocking the save.
	raw["detective"] = map[string]interface{}{"extra_read_operations": "not-a-map"}
	normalizeDetectiveYAMLSection(raw)
	if got := raw["detective"].(map[string]interface{})["extra_read_operations"].(map[string]interface{}); len(got) != 0 {
		t.Fatalf("malformed value = %v", got)
	}
}
