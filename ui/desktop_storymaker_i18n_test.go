package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var storymakerPlaceholder = regexp.MustCompile(`\{\{?\w+\}\}?`)

func storymakerLocale(t *testing.T, dir, lang string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("lang", dir, lang+".json"))
	if err != nil {
		t.Fatalf("%s/%s.json: %v", dir, lang, err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("%s/%s.json must be a flat map of strings: %v", dir, lang, err)
	}
	return values
}

func storymakerPlaceholders(text string) string {
	found := storymakerPlaceholder.FindAllString(text, -1)
	sort.Strings(found)
	return strings.Join(found, " ")
}

func TestStorymakerLocalesCoverAllSixteenLanguages(t *testing.T) {
	t.Parallel()
	english := map[string]map[string]string{
		"storymaker":         storymakerLocale(t, "storymaker", "en"),
		"storymaker_catalog": storymakerLocale(t, "storymaker_catalog", "en"),
	}
	if len(english["storymaker"]) < 150 {
		t.Fatalf("lang/storymaker/en.json holds %d keys, the app needs far more", len(english["storymaker"]))
	}
	for _, lang := range storymakerLanguages {
		for dir, base := range english {
			values := storymakerLocale(t, dir, lang)
			for key := range base {
				text := strings.TrimSpace(values[key])
				if text == "" {
					t.Errorf("%s/%s.json lacks %s", dir, lang, key)
					continue
				}
				if storymakerPlaceholders(text) != storymakerPlaceholders(base[key]) {
					t.Errorf("%s/%s.json %s has the placeholders %q, English has %q", dir, lang, key, storymakerPlaceholders(text), storymakerPlaceholders(base[key]))
				}
				if strings.ContainsRune(text, '�') {
					t.Errorf("%s/%s.json %s holds a broken character", dir, lang, key)
				}
			}
			for key := range values {
				if _, ok := base[key]; !ok {
					t.Errorf("%s/%s.json has %s, which English lacks", dir, lang, key)
				}
			}
			// The directories keep to their own keys: the catalog owns storymaker.catalog.*, the app everything else.
			for key := range values {
				isCatalog := strings.HasPrefix(key, "storymaker.catalog.")
				if (dir == "storymaker_catalog") != isCatalog || !strings.HasPrefix(key, "storymaker.") {
					t.Errorf("%s/%s.json holds %s, which belongs to the other directory or to no storymaker section", dir, lang, key)
				}
			}
		}
	}
}

// A translation that is the English text is a missed translation, except for the names and words that really are the same
// (the product, FFmpeg, Pause or Code in a few languages, the format name). The list is exact: a new key that a language copies
// from English fails here.
func TestStorymakerAppTranslationsAreTranslated(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "storymaker", "en")
	same := map[string]bool{
		"storymaker.title": true, "storymaker.tab_stage": true, "storymaker.strip.duration_s": true, "storymaker.export.cap_ffmpeg": true,
		"storymaker.pause": true, "storymaker.side_scene": true, "storymaker.projects.new_format": true, "storymaker.strip.scene_n": true,
		"storymaker.export.cap_browser": true, "storymaker.tab_code": true, "storymaker.code.title": true, "storymaker.aspect_9_16": true,
	}
	for _, lang := range storymakerLanguages {
		if lang == "en" {
			continue
		}
		values := storymakerLocale(t, "storymaker", lang)
		for key, text := range english {
			if values[key] == text && !same[key] {
				t.Errorf("%s keeps the English text of %s: %q", lang, key, text)
			}
		}
	}
}
