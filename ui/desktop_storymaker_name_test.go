package ui

import (
	"strings"
	"testing"
)

// The sixteen languages of the AuraGo UI; every Storymaker locale directory has one file per language.
var storymakerLanguages = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func TestStorymakerAppNameExistsInAllLanguages(t *testing.T) {
	t.Parallel()
	for _, lang := range storymakerLanguages {
		if strings.TrimSpace(readMergedLangMap(t, lang)["desktop.app_storymaker"]) == "" {
			t.Errorf("%s lacks desktop.app_storymaker", lang)
		}
	}
}
