package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadCYDCreatorYAML(t *testing.T, content string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestCYDCreatorDefaultsWhenTheSectionIsAbsent(t *testing.T) {
	want := DefaultCYDCreatorConfig()
	if want.Enabled || want.BuilderImage != "ghcr.io/antibyte/aurago-cyd-builder:1" || want.BuildTimeoutSeconds != 300 ||
		want.ColdBuildTimeoutSeconds != 600 || want.MemoryMB != 2048 || want.CPUCores != 2 || want.IdleStopMinutes != 20 ||
		want.MaxRepairAttempts != 3 || want.JobTimeoutMinutes != 20 || want.Model != "" || want.KeepBuildsPerProject != 5 ||
		!want.OTA.Enabled || want.OTA.HeartbeatSeconds != 60 || want.OTA.PublicURL != "" {
		t.Fatalf("defaults differ from spec §15: %+v", want)
	}
	for name, content := range map[string]string{
		"empty file":    "{}\n",
		"null section":  "cyd_creator:\n",
		"empty section": "cyd_creator: {}\n",
		"only enabled":  "cyd_creator:\n  enabled: false\n",
	} {
		t.Run(name, func(t *testing.T) {
			cfg := loadCYDCreatorYAML(t, content)
			if cfg.CYDCreator != want {
				t.Fatalf("loaded = %+v, want %+v", cfg.CYDCreator, want)
			}
			if !strings.HasSuffix(filepath.ToSlash(cfg.SQLite.CYDCreatorPath), "/data/cyd_creator.db") || !filepath.IsAbs(cfg.SQLite.CYDCreatorPath) {
				t.Fatalf("sqlite.cyd_creator_path = %q, want an absolute data/cyd_creator.db", cfg.SQLite.CYDCreatorPath)
			}
		})
	}
}

func TestCYDCreatorLoadNormalizesValues(t *testing.T) {
	cfg := loadCYDCreatorYAML(t, `cyd_creator:
  enabled: true
  builder_image: "  registry.local:5000/me/cyd:dev  "
  build_timeout_seconds: 10
  cold_build_timeout_seconds: 20
  memory_mb: 99999
  cpu_cores: 0.1
  idle_stop_minutes: -5
  max_repair_attempts: 40
  job_timeout_minutes: 500
  keep_builds_per_project: 1000
  ota:
    enabled: false
    heartbeat_seconds: 5
    public_url: " https://aurago.example.com/ "
`).CYDCreator
	want := CYDCreatorConfig{Enabled: true, BuilderImage: "registry.local:5000/me/cyd:dev", BuildTimeoutSeconds: 60,
		ColdBuildTimeoutSeconds: 60, MemoryMB: 16384, CPUCores: 0.5, IdleStopMinutes: 1, MaxRepairAttempts: 10,
		JobTimeoutMinutes: 120, KeepBuildsPerProject: 50,
		OTA: CYDCreatorOTAConfig{Enabled: false, HeartbeatSeconds: 30, PublicURL: "https://aurago.example.com"}}
	if cfg != want {
		t.Fatalf("normalized = %+v, want %+v", cfg, want)
	}
	bad := loadCYDCreatorYAML(t, "cyd_creator:\n  builder_image: \"not an image!\"\n").CYDCreator
	if bad.BuilderImage != DefaultCYDCreatorBuilderImage {
		t.Fatalf("invalid image kept: %q", bad.BuilderImage)
	}
}

func TestValidateCYDCreatorConfig(t *testing.T) {
	if err := ValidateCYDCreatorConfig(DefaultCYDCreatorConfig()); err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}
	if err := ValidateCYDCreatorConfig(CYDCreatorConfig{}); err != nil {
		t.Fatalf("zero values (defaults) rejected: %v", err)
	}
	for name, mutate := range map[string]func(*CYDCreatorConfig){
		"image":       func(c *CYDCreatorConfig) { c.BuilderImage = "UPPER CASE" },
		"timeout":     func(c *CYDCreatorConfig) { c.BuildTimeoutSeconds = 30 },
		"cold < warm": func(c *CYDCreatorConfig) { c.BuildTimeoutSeconds, c.ColdBuildTimeoutSeconds = 900, 600 },
		"memory":      func(c *CYDCreatorConfig) { c.MemoryMB = 100 },
		"cpu":         func(c *CYDCreatorConfig) { c.CPUCores = 32 },
		"idle":        func(c *CYDCreatorConfig) { c.IdleStopMinutes = 5000 },
		"repairs":     func(c *CYDCreatorConfig) { c.MaxRepairAttempts = -1 },
		"job":         func(c *CYDCreatorConfig) { c.JobTimeoutMinutes = 1 },
		"keep":        func(c *CYDCreatorConfig) { c.KeepBuildsPerProject = 99 },
		"heartbeat":   func(c *CYDCreatorConfig) { c.OTA.HeartbeatSeconds = 10 },
		"url scheme":  func(c *CYDCreatorConfig) { c.OTA.PublicURL = "ftp://x" },
		"url user":    func(c *CYDCreatorConfig) { c.OTA.PublicURL = "https://u:p@x" },
	} {
		c := DefaultCYDCreatorConfig()
		mutate(&c)
		if err := ValidateCYDCreatorConfig(c); err == nil {
			t.Errorf("%s: invalid value accepted: %+v", name, c)
		}
	}
	ok := DefaultCYDCreatorConfig()
	ok.BuilderImage = "ghcr.io/antibyte/aurago-cyd-builder@sha256:" + strings.Repeat("a", 64)
	ok.OTA.PublicURL = "http://192.168.1.10:8088"
	if err := ValidateCYDCreatorConfig(ok); err != nil {
		t.Fatalf("valid digest image / http URL rejected: %v", err)
	}
}

// TestCYDCreatorTemplateMatchesLoadDefaults pins the cyd_creator block of
// config_template.yaml against DefaultCYDCreatorConfig.
func TestCYDCreatorTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	node, ok := doc["cyd_creator"]
	if !ok {
		t.Fatal("config_template.yaml has no cyd_creator: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl CYDCreatorConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("decode cyd_creator block: %v", err)
	}
	if tpl != DefaultCYDCreatorConfig() {
		t.Fatalf("template = %+v, want %+v", tpl, DefaultCYDCreatorConfig())
	}
	var keys map[string]any
	_ = yaml.Unmarshal(raw, &keys)
	got := make([]string, 0, len(keys))
	for k := range keys {
		got = append(got, k)
	}
	sort.Strings(got)
	if want := "build_timeout_seconds,builder_image,cold_build_timeout_seconds,cpu_cores,enabled,idle_stop_minutes,job_timeout_minutes,keep_builds_per_project,max_repair_attempts,memory_mb,model,ota"; strings.Join(got, ",") != want {
		t.Fatalf("template keys = %s, want %s", strings.Join(got, ","), want)
	}
	sqlite := doc["sqlite"]
	var paths map[string]string
	if err := sqlite.Decode(&paths); err != nil || paths["cyd_creator_path"] != "./data/cyd_creator.db" {
		t.Fatalf("sqlite.cyd_creator_path in the template = %q, %v", paths["cyd_creator_path"], err)
	}
}

func TestSQLiteDatabasePathsIncludesCYDCreator(t *testing.T) {
	cfg := &Config{}
	cfg.SQLite.CYDCreatorPath = filepath.Join(t.TempDir(), "cyd_creator.db")
	if !containsPath(SQLiteDatabasePaths(cfg), cfg.SQLite.CYDCreatorPath) {
		t.Fatalf("SQLiteDatabasePaths lacks %s", cfg.SQLite.CYDCreatorPath)
	}
}
