package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigManusRequiresModalWhenDisablingReadOnly(t *testing.T) {
	t.Parallel()

	manusJS := readDesktopAssetText(t, "cfg/manus.js")
	for _, marker := range []string{
		"async function manusToggleReadOnly",
		"config.manus.read_only_disable_confirm_title",
		"config.manus.read_only_disable_confirm",
		"showConfirm(",
		"el.classList.contains('on')",
	} {
		if !strings.Contains(manusJS, marker) {
			t.Fatalf("Manus read-only modal contract missing %q", marker)
		}
	}
}

func TestConfigManusLoadsCachesAndDeduplicatesProjectSkills(t *testing.T) {
	t.Parallel()

	manusJS := readDesktopAssetText(t, "cfg/manus.js")
	for _, marker := range []string{
		"projectSkills: {}",
		"async function manusLoadProjectSkills",
		"project_id=' + encodeURIComponent(projectID)",
		"await manusLoadProjectSkills(cfg.allowed_project_ids",
		"if (kind === 'projects' && !wasSelected)",
		"new Map()",
	} {
		if !strings.Contains(manusJS, marker) {
			t.Fatalf("Manus project skill contract missing %q", marker)
		}
	}
}

func TestConfigManusDisablesRemoteActionsUntilStatusConfirmsEnabled(t *testing.T) {
	t.Parallel()

	manusJS := readDesktopAssetText(t, "cfg/manus.js")
	for _, marker := range []string{
		"actionsEnabled: false",
		"const actionsDisabled = manusCatalogState.actionsEnabled ? '' : ' disabled'",
		`id="manus-load-catalogs-btn"`,
		"function manusSetActionAvailability",
		"manusSetActionAvailability(data.enabled === true)",
		"manusSetActionAvailability(false)",
		"btn.disabled = !manusCatalogState.actionsEnabled",
	} {
		if !strings.Contains(manusJS, marker) {
			t.Fatalf("Manus disabled-action contract missing %q", marker)
		}
	}
}

func TestConfigManusFindingTranslationsAreComplete(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, lang := range languages {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/config/manus/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse Manus %s translations: %v", lang, err)
		}
		for _, key := range []string{
			"config.manus.read_only_disable_confirm_title",
			"config.manus.read_only_disable_confirm",
			"help.manus.allowed_skills",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("Manus %s translation missing %q", lang, key)
			}
		}
	}
}
