package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGameMakerConfigSectionIsRegisteredAndPersistsPermissions(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`{ key: 'game_maker'`,
		`game_maker: { m: 'game_maker', fn: 'renderGameMakerSection' }`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing Game Maker marker %q", marker)
		}
	}

	module := normalizeAssetText(mustReadUIFile(t, "cfg/game_maker.js"))
	for _, path := range []string{
		"game_maker.enabled",
		"game_maker.readonly",
		"game_maker.allow_create",
		"game_maker.allow_edit",
		"game_maker.allow_delete",
		"game_maker.allow_media_generation",
		"game_maker.workspace_path",
		"game_maker.max_projects",
		"game_maker.max_files_per_project",
		"game_maker.max_file_size_kb",
		"game_maker.max_project_size_mb",
		"game_maker.job_timeout_seconds",
	} {
		if !strings.Contains(module, path) {
			t.Errorf("Game Maker config module missing %q", path)
		}
	}
	for _, marker := range []string{
		`window.AuraConfigState.set('game_maker.enabled', nextEnabled)`,
		`window.AuraConfigState.set(path, value)`,
		`config.game_maker.restart_note`,
	} {
		if !strings.Contains(module, marker) {
			t.Errorf("Game Maker config module missing persistence marker %q", marker)
		}
	}
	if strings.Contains(module, "alert(") || strings.Contains(module, "confirm(") {
		t.Fatal("Game Maker config must not use native browser dialogs")
	}
}

func TestGameMakerConfigTranslationsExist(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"config.section.game_maker.label",
		"config.section.game_maker.desc",
		"config.game_maker.enabled_label",
		"config.game_maker.disabled_notice",
		"config.game_maker.disabled_desc",
		"config.game_maker.restart_note",
		"config.game_maker.permissions_title",
		"config.game_maker.permissions_desc",
		"config.game_maker.readonly_label",
		"config.game_maker.allow_create_label",
		"config.game_maker.allow_edit_label",
		"config.game_maker.allow_delete_label",
		"config.game_maker.allow_media_generation_label",
		"config.game_maker.storage_title",
		"config.game_maker.workspace_path_label",
		"config.game_maker.limits_title",
		"config.game_maker.max_projects_label",
		"config.game_maker.max_files_per_project_label",
		"config.game_maker.max_file_size_kb_label",
		"config.game_maker.max_project_size_mb_label",
		"config.game_maker.job_timeout_seconds_label",
		"help.game_maker.enabled",
		"help.game_maker.readonly",
		"help.game_maker.allow_create",
		"help.game_maker.allow_edit",
		"help.game_maker.allow_delete",
		"help.game_maker.allow_media_generation",
	}
	for _, language := range languages {
		path := fmt.Sprintf("lang/config/game_maker/%s.json", language)
		var translations map[string]string
		if err := json.Unmarshal([]byte(mustReadUIFile(t, path)), &translations); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(translations[key]) == "" {
				t.Errorf("%s is missing translation %s", path, key)
			}
		}
	}
}
