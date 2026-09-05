package ui

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestConfigMeshCoreTranslationsAndBoundaries(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	module := read("cfg/meshcore.js")
	main := read("js/config/main.js")
	if !strings.Contains(main, "meshcore: { m: 'meshcore', fn: 'renderMeshCoreSection' }") {
		t.Fatal("lazy renderer missing")
	}
	for _, wanted := range []string{"window.AuraConfigState.get('meshcore')", "window.AuraConfigState.isDirty()", "AbortController", "aurago:config-saved", "text.textContent = msg.text", "pin.value = ''", "set('identity_key', runtime.status.identity_key)", "binding: channel.binding", "data-type=\"array-lines\""} {
		if !strings.Contains(module, wanted) {
			t.Fatalf("UI contract missing %s", wanted)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt(", "/send", "localStorage", "sessionStorage"} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("unsafe setup action: %s", forbidden)
		}
	}
	var en map[string]string
	if err := json.Unmarshal([]byte(read("lang/config/meshcore/en.json")), &en); err != nil {
		t.Fatal(err)
	}
	for _, match := range regexp.MustCompile(`\btr\('([^']+)'\)`).FindAllStringSubmatch(module, -1) {
		if en["config.meshcore."+match[1]] == "" {
			t.Fatalf("missing key %s", match[1])
		}
	}
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var entries map[string]string
		if err := json.Unmarshal([]byte(read("lang/config/meshcore/"+lang+".json")), &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries) != len(en) {
			t.Fatalf("%s: translation count", lang)
		}
		for k, v := range en {
			if strings.TrimSpace(entries[k]) == "" {
				t.Fatalf("%s missing %s", lang, k)
			}
			if entries[k] == v && (k == "config.meshcore.security" || k == "config.section.meshcore.desc") {
				t.Fatalf("%s untranslated %s", lang, k)
			}
		}
	}
}
