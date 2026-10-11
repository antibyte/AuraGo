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

func loadRetroGamesYAML(t *testing.T, content string) *Config {
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

func TestRetroGamesDefaultsOn(t *testing.T) {
	for name, content := range map[string]string{
		"empty file":   "{}\n",
		"null section": "retro_games:\n",
	} {
		cfg := loadRetroGamesYAML(t, content)
		if !cfg.RetroGames.Enabled || !cfg.RetroGames.AgentAccess {
			t.Fatalf("%s: retro_games = %+v, want both on", name, cfg.RetroGames)
		}
	}
	cfg := loadRetroGamesYAML(t, "retro_games:\n  enabled: false\n  agent_access: false\n")
	if cfg.RetroGames.Enabled || cfg.RetroGames.AgentAccess {
		t.Fatalf("explicit off ignored: %+v", cfg.RetroGames)
	}
}

func TestRetroGamesTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	node, ok := doc["retro_games"]
	if !ok {
		t.Fatal("config_template.yaml has no retro_games: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl RetroGamesConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("decode retro_games block: %v", err)
	}
	var keys map[string]interface{}
	if err := yaml.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(keys))
	for k := range keys {
		got = append(got, k)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "agent_access,enabled" {
		t.Fatalf("template keys = %v", got)
	}
	if loaded := loadRetroGamesYAML(t, "{}\n").RetroGames; tpl != loaded {
		t.Fatalf("template = %+v, defaults = %+v", tpl, loaded)
	}
}
