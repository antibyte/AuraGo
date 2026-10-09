package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/security"
)

func TestValidateLocalWikipediaSettings(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{Cfg: cfg}
	system := "/etc/wikipedia"
	if runtime.GOOS == "windows" {
		system = `C:\Windows\wikipedia`
	}
	for name, tc := range map[string]struct {
		dir     string
		docker  bool
		wantErr bool
	}{
		"empty uses the default": {dir: ""},
		"absolute directory":     {dir: filepath.Join(t.TempDir(), "wiki")},
		"relative directory":     {dir: "wiki", wantErr: true},
		"data directory root":    {dir: cfg.Directories.DataDir, wantErr: true},
		"system directory":       {dir: system, wantErr: true},
		"docker ignores the dir": {dir: "wiki", docker: true},
	} {
		err := validateLocalWikipediaSettings(s, config.LocalWikipediaConfig{DataDir: tc.dir}, config.Runtime{IsDocker: tc.docker})
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: validateLocalWikipediaSettings = %v, wantErr %v", name, err, tc.wantErr)
		}
	}
}

func TestInjectLocalWikipediaDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.LocalWikipedia = config.LocalWikipediaConfig{AgentAccess: true, Variant: "nopic", UpdateCheck: true}
	raw := map[string]interface{}{}
	injectLocalWikipediaDefaults(raw, cfg)
	section := raw["local_wikipedia"].(map[string]interface{})
	if section["enabled"] != false || section["agent_access"] != true || section["update_check"] != true ||
		section["variant"] != "nopic" || section["language"] != "" || section["data_dir"] != "" {
		t.Fatalf("injected defaults = %#v", section)
	}
	raw = map[string]interface{}{"local_wikipedia": map[string]interface{}{"agent_access": false, "variant": "maxi"}}
	injectLocalWikipediaDefaults(raw, cfg)
	section = raw["local_wikipedia"].(map[string]interface{})
	if section["agent_access"] != false || section["variant"] != "maxi" {
		t.Fatalf("explicit values were overwritten: %#v", section)
	}
}

func TestNewServerFromOptionsConstructsLocalWikipediaManager(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Agent.SystemLanguage = "Deutsch"
	srv := newServerFromOptions(StartOptions{Cfg: cfg, Logger: slog.Default(), AccessLogger: slog.Default(), ShutdownCh: make(chan struct{})})
	if srv.LocalWiki == nil {
		t.Fatal("newServerFromOptions did not construct the Local Wikipedia manager")
	}
	settings := srv.LocalWiki.Settings()
	if settings.DataDir != filepath.Join(cfg.Directories.DataDir, "wikipedia") || settings.Language != "de" {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestHandleUpdateConfigReconfiguresLocalWikipedia(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  system_language: Deutsch\nlocal_wikipedia:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(loaded))
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault, LocalWiki: manager}
	save := func(payload string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
		return recorder
	}

	if got := save(`{"local_wikipedia":{"enabled":true,"language":"fr","variant":"maxi"}}`); got.Code != http.StatusOK {
		t.Fatalf("save = %d %s", got.Code, got.Body.String())
	}
	settings := manager.Settings()
	if !settings.Enabled || settings.Language != "fr" || settings.Variant != localwiki.VariantMaxi || settings.SystemLanguage != "de" {
		t.Fatalf("manager not reconfigured: %+v", settings)
	}
	for _, payload := range []string{
		`{"local_wikipedia":{"language":"xx"}}`,
		`{"local_wikipedia":{"variant":"mini"}}`,
		`{"local_wikipedia":{"data_dir":"relative/wiki"}}`,
	} {
		if got := save(payload); got.Code != http.StatusBadRequest {
			t.Fatalf("save %s = %d, want 400", payload, got.Code)
		}
	}
}
