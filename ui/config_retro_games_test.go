package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var retroGamesConfigKeys = []string{
	"config.retro_games.title", "config.retro_games.help", "config.retro_games.enabled", "config.retro_games.agent_access",
	"config.retro_games.check", "config.retro_games.checking", "config.retro_games.status_ready", "config.retro_games.status_empty",
	"config.retro_games.unavailable", "config.retro_games.save_first", "config.retro_games.status_building",
}

func TestConfigRetroGamesBlockIsWired(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "virtual_desktop.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, marker := range []string{
		"html += vdCfgRetroGames();",
		"function vdCfgRetroGames()",
		"'retro_games.enabled'",
		"'retro_games.agent_access'",
		// A plain status request would start the first index download.
		"fetch('/api/desktop/retro-games/status?passive=1'",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("virtual_desktop.js lacks %q", marker)
		}
	}
	if strings.Contains(src, "retro-games/status'") || strings.Contains(src, "retro-games/status\"") {
		t.Error("virtual_desktop.js requests the Retro Games status without passive=1")
	}
	for _, key := range retroGamesConfigKeys {
		suffix := strings.TrimPrefix(key, "config.retro_games.")
		if !strings.Contains(src, "'"+suffix+"'") && !strings.Contains(src, "'"+key+"'") {
			t.Errorf("virtual_desktop.js never uses %s", key)
		}
	}
}

func TestConfigRetroGamesTranslations(t *testing.T) {
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var english map[string]string
	for _, lang := range langs {
		data, err := os.ReadFile(filepath.Join("lang", "config", "common", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if lang == "en" {
			english = m
		}
		for _, key := range retroGamesConfigKeys {
			v := strings.TrimSpace(m[key])
			if v == "" || strings.ContainsRune(v, '�') {
				t.Errorf("%s: %s missing or broken", lang, key)
			}
			if strings.Contains(m[key], "{count}") != strings.Contains(key, "status_ready") {
				t.Errorf("%s: %s placeholder mismatch", lang, key)
			}
		}
		if lang == "de" {
			for _, key := range retroGamesConfigKeys {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
					if strings.Contains(" "+m[key]+" ", formal) {
						t.Errorf("de: %s uses formal address", key)
					}
				}
			}
		}
	}
	for _, lang := range langs {
		if lang == "en" {
			continue
		}
		data, _ := os.ReadFile(filepath.Join("lang", "config", "common", lang+".json"))
		var m map[string]string
		_ = json.Unmarshal(data, &m)
		for _, key := range retroGamesConfigKeys {
			if key == "config.retro_games.title" {
				continue
			}
			if m[key] == english[key] {
				t.Errorf("%s: %s is untranslated", lang, key)
			}
		}
	}
}
