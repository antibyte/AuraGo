package persondetect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var alertLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

// alertLocaleTexts reads the four alert keys of one ui/lang/desktop locale file.
func alertLocaleTexts(t *testing.T, lang string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "ui", "lang", "desktop", lang+".json"))
	if err != nil {
		t.Fatalf("read %s: %v", lang, err)
	}
	var all map[string]any
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatalf("parse %s: %v", lang, err)
	}
	out := map[string]string{}
	for key := range alertDefaultTexts {
		if s, ok := all[key].(string); ok {
			out[key] = s
		}
	}
	return out
}

// Every locale has the four alert texts with their placeholders, the non-English ones are
// real translations, and the built-in English defaults equal en.json.
func TestAlertTextsExistInEveryDesktopLocale(t *testing.T) {
	required := map[string][]string{
		AlertTextTitle:       {"{camera}"},
		AlertTextMessage:     {"{camera}", "{zones}", "{time}", "{score}"},
		AlertTextAllZones:    {},
		AlertTextAgentPrompt: {"{event_id}", "go2rtc", "analyze_detection_event"},
	}
	en := alertLocaleTexts(t, "en")
	for key, want := range alertDefaultTexts {
		if en[key] != want {
			t.Errorf("en.json %s = %q, want the built-in default %q", key, en[key], want)
		}
	}
	for _, lang := range alertLocales {
		texts := alertLocaleTexts(t, lang)
		for key, placeholders := range required {
			value := strings.TrimSpace(texts[key])
			if value == "" {
				t.Errorf("%s.json misses %s", lang, key)
				continue
			}
			for _, p := range placeholders {
				if !strings.Contains(value, p) {
					t.Errorf("%s.json %s lacks %s: %q", lang, key, p, value)
				}
			}
			// A camera wake-up has no chat: the prompt must not offer show_detection_event,
			// which cannot show the picture to anyone there.
			if key == AlertTextAgentPrompt && strings.Contains(value, "show_detection_event") {
				t.Errorf("%s.json %s points to show_detection_event: %q", lang, key, value)
			}
			if key == AlertTextAllZones && strings.ContainsAny(value, "{}") {
				t.Errorf("%s.json %s must not hold placeholders: %q", lang, key, value)
			}
			if lang != "en" && key != AlertTextMessage && value == en[key] {
				t.Errorf("%s.json %s is the English text", lang, key)
			}
		}
	}
}
