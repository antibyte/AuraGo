package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeStudioStatusAndSearchCap(t *testing.T) {
	coreJS, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "code-studio", "core.js"))
	if err != nil {
		t.Fatal(err)
	}
	searchJS, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "code-studio", "search.js"))
	if err != nil {
		t.Fatal(err)
	}
	core := string(coreJS)
	search := string(searchJS)
	for _, marker := range []string{
		`data-network="none"`,
		"codeStudio.networkOff",
		"codeStudio.readOnly",
		"codeStudio.idleUntil",
		"containerWaitMessage",
		"rememberContainerStatus",
	} {
		if !strings.Contains(core, marker) {
			t.Errorf("core.js lacks %q", marker)
		}
	}
	for _, marker := range []string{"codeStudio.resultsTruncated", "searchTruncated"} {
		if !strings.Contains(search, marker) {
			t.Errorf("search.js lacks %q", marker)
		}
	}

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"codeStudio.readOnly",
		"codeStudio.networkOff",
		"codeStudio.resultsTruncated",
		"codeStudio.idleUntil",
	}
	var english map[string]string
	for _, lang := range langs {
		raw, err := os.ReadFile(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if lang == "en" {
			english = values
		}
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, '\uFFFD') {
				t.Errorf("%s: %s missing or broken", lang, key)
			}
		}
		if !strings.Contains(values["codeStudio.idleUntil"], "{{time}}") {
			t.Errorf("%s: idleUntil drops the time placeholder", lang)
		}
		if lang == "de" {
			for _, key := range keys {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
					if strings.Contains(" "+values[key]+" ", formal) {
						t.Errorf("de: %s uses formal address", key)
					}
				}
			}
		}
		if lang != "en" && english != nil {
			if values["codeStudio.networkOff"] == english["codeStudio.networkOff"] ||
				values["codeStudio.resultsTruncated"] == english["codeStudio.resultsTruncated"] {
				t.Errorf("%s copies English status text", lang)
			}
		}
	}
}
