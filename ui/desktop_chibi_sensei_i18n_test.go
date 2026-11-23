package ui

import (
	"strings"
	"testing"
)

// Every app key exists in all sixteen languages with the same placeholders, and no language carries a key English lacks.
func TestChibiSenseiLocalesCoverAllSixteenLanguages(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "chibi_sensei", "en")
	if len(english) < 250 {
		t.Fatalf("lang/chibi_sensei/en.json holds %d keys, the app needs far more", len(english))
	}
	for key := range english {
		if !strings.HasPrefix(key, "chibi_sensei.") {
			t.Errorf("en.json holds %s, which is not a chibi_sensei key", key)
		}
	}
	for _, lang := range storymakerLanguages {
		values := storymakerLocale(t, "chibi_sensei", lang)
		for key, base := range english {
			text := strings.TrimSpace(values[key])
			if text == "" {
				t.Errorf("chibi_sensei/%s.json lacks %s", lang, key)
				continue
			}
			if storymakerPlaceholders(text) != storymakerPlaceholders(base) {
				t.Errorf("chibi_sensei/%s.json %s has the placeholders %q, English has %q", lang, key, storymakerPlaceholders(text), storymakerPlaceholders(base))
			}
			if strings.ContainsRune(text, '�') {
				t.Errorf("chibi_sensei/%s.json %s holds a broken character", lang, key)
			}
		}
		for key := range values {
			if _, ok := english[key]; !ok {
				t.Errorf("chibi_sensei/%s.json has %s, which English lacks", lang, key)
			}
		}
	}
}

// A translation that is the English text is a missed translation, except for product, format and method names. The list is
// exact: a new key that a language copies from English fails here.
func TestChibiSenseiTranslationsAreTranslated(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "chibi_sensei", "en")
	same := map[string]bool{
		"chibi_sensei.title": true, "chibi_sensei.panel.title": true, "chibi_sensei.data.hf": true, "chibi_sensei.source.hf": true,
		"chibi_sensei.import.format_jsonl": true, "chibi_sensei.import.format_csv": true,
		"chibi_sensei.table.col_index": true, "chibi_sensei.training.method_lora": true,
		// Tokens and symbols that read the same in every language.
		"chibi_sensei.export.filter": true, "chibi_sensei.training.lora": true, "chibi_sensei.rail.step_label": true,
		"chibi_sensei.source.sensei": true, "chibi_sensei.checks.level_ok": true,
	}
	// Words that really are the same in one language: German shares Text, Training, Format, Name and more with English, and
	// the loan word Prompt stays Prompt in Polish, Portuguese and Swedish. The list is exact per language.
	sameIn := map[string]map[string]bool{
		"de": {
			"chibi_sensei.review.role_text": true, "chibi_sensei.split.train": true, "chibi_sensei.table.filter_flag": true,
			"chibi_sensei.editor.text_label": true, "chibi_sensei.import.format": true, "chibi_sensei.step.training": true,
			"chibi_sensei.create.name_label": true, "chibi_sensei.goal.labels_label": true, "chibi_sensei.flag.other": true,
			"chibi_sensei.training.batch": true, "chibi_sensei.footer.train": true, "chibi_sensei.step.test": true,
			"chibi_sensei.editor.role_system": true, "chibi_sensei.table.col_text": true, "chibi_sensei.editor.tags_label": true,
		},
		// Danish uses the English words for a download and an upload.
		"da": {"chibi_sensei.run.download": true, "chibi_sensei.phase.upload": true},
		// "Error" is the Spanish word too.
		"es": {"chibi_sensei.event.error": true},
		// "Preset" is the established word in Italian and Dutch; Dutch also says "epochs".
		"it": {"chibi_sensei.training.preset_title": true},
		"nl": {"chibi_sensei.training.preset_title": true, "chibi_sensei.overrides.epochs": true},
		// Chinese keeps Kaggle's own product term.
		"zh": {"chibi_sensei.leftover.notebook": true},
		"pl": {"chibi_sensei.table.col_prompt": true},
		"pt": {"chibi_sensei.table.col_prompt": true},
		"sv": {"chibi_sensei.table.col_prompt": true},
	}
	for _, lang := range storymakerLanguages {
		if lang == "en" {
			continue
		}
		values := storymakerLocale(t, "chibi_sensei", lang)
		for key, text := range english {
			if values[key] == text && !same[key] && !sameIn[lang][key] {
				t.Errorf("%s keeps the English text of %s: %q", lang, key, text)
			}
		}
	}
}

// The German copy addresses the user as "Du".
func TestChibiSenseiGermanUsesDu(t *testing.T) {
	t.Parallel()
	for key, text := range storymakerLocale(t, "chibi_sensei", "de") {
		padded := " " + text + " "
		if strings.Contains(padded, " Sie ") || strings.Contains(padded, " Ihre ") || strings.Contains(padded, " Ihnen ") {
			t.Errorf("de: %s uses formal address", key)
		}
	}
}
