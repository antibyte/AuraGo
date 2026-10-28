package ui

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every locale carries the same docker_manager.* keys as English, none empty, with the same {{placeholders}}.
func TestDockerManagerLocalesAreComplete(t *testing.T) {
	t.Parallel()

	placeholder := regexp.MustCompile(`\{\{(\w+)\}\}`)
	tokens := func(text string) string {
		var out []string
		for _, match := range placeholder.FindAllStringSubmatch(text, -1) {
			out = append(out, match[1])
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	load := func(lang string) map[string]string {
		values := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse %s: %v", lang, err)
		}
		return values
	}
	owned := func(key string) bool {
		return strings.HasPrefix(key, "docker_manager.") || key == "desktop.app_docker_manager"
	}

	english := load("en")
	var keys []string
	for key := range english {
		if owned(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) < 150 {
		t.Fatalf("English has only %d Docker Manager keys", len(keys))
	}
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := load(lang)
		count := 0
		for key := range values {
			if owned(key) {
				count++
			}
		}
		if count != len(keys) {
			t.Errorf("%s has %d Docker Manager keys, English has %d", lang, count, len(keys))
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing %s", lang, key)
			} else if tokens(values[key]) != tokens(english[key]) {
				t.Errorf("%s changes the placeholders of %s: %q", lang, key, values[key])
			}
		}
	}
}
