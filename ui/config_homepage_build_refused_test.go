package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageConfigShowsRefusedImageBuild(t *testing.T) {
	module := string(mustReadUIFile(t, "cfg/homepage.js"))
	for _, required := range []string{
		"st.image_build_refused",
		"t('config.homepage.image_build_refused')",
		"t('config.homepage.image_build_refused_desc')",
		"escapeHtml(st.image_build_command",
	} {
		if !strings.Contains(module, required) {
			t.Fatalf("cfg/homepage.js is missing %q", required)
		}
	}
}

func TestHomepageBuildRefusedKeysAreTranslated(t *testing.T) {
	read := func(locale string) map[string]string {
		raw, err := os.ReadFile(filepath.Join("lang", "config", "homepage", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("decode %s: %v", locale, err)
		}
		return values
	}
	english := read("en")
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := read(locale)
		for _, key := range []string{"config.homepage.image_build_refused", "config.homepage.image_build_refused_desc"} {
			value := strings.TrimSpace(values[key])
			if value == "" {
				t.Fatalf("%s: %s is missing", locale, key)
			}
			if locale != "en" && value == english[key] {
				t.Fatalf("%s: %s copies the English text", locale, key)
			}
		}
	}
}
