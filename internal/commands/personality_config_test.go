package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestPersonalityCommandSavesOnlyValidatedProfile(t *testing.T) {
	root := t.TempDir()
	profiles := filepath.Join(root, "prompts", "personalities")
	if err := os.MkdirAll(profiles, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "calm.md"), []byte("calm"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte("personality:\n  core_personality: old\nserver:\n  ui_language: de\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: path}
	cfg.Personality.CorePersonality = "old"
	cfg.Server.UILanguage = "fr"
	command := &PersonalityCommand{}
	ctx := Context{Cfg: cfg, PromptsDir: filepath.Join(root, "prompts"), Lang: "en"}
	if _, err := command.Execute([]string{"../calm"}, ctx); err != nil {
		t.Fatal(err)
	}
	if cfg.Personality.CorePersonality != "old" {
		t.Fatal("invalid profile changed runtime config")
	}
	if _, err := command.Execute([]string{"calm"}, ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "core_personality: calm") || !strings.Contains(string(data), "ui_language: de") {
		t.Fatalf("config = %q, %v", data, err)
	}
	if cfg.Personality.CorePersonality != "calm" {
		t.Fatal("successful save did not update runtime profile")
	}
}
