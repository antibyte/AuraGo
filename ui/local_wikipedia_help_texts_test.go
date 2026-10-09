package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var localWikipediaHelpFields = []string{"enabled", "agent_access", "language", "variant", "data_dir", "update_check"}

// localWikipediaLocales is declared in config_local_wikipedia_test.go.

func TestLocalWikipediaConfigHelpTextsCoverAllLocales(t *testing.T) {
	module, err := os.ReadFile(filepath.Join("cfg", "local_wikipedia.js"))
	if err != nil {
		t.Fatalf("read cfg/local_wikipedia.js: %v", err)
	}
	js := string(module)
	// Most help texts are rendered through the field helpers:
	// localWikiToggle/localWikiSelect call t('help.local_wikipedia.' + key).
	dynamicHelp := strings.Contains(js, "t('help.local_wikipedia.' + key)")
	for _, field := range localWikipediaHelpFields {
		key := "help.local_wikipedia." + field
		literal := strings.Contains(js, "'"+key+"'") || strings.Contains(js, `"`+key+`"`) || strings.Contains(js, "helpTexts['local_wikipedia."+field+"']")
		viaHelper := dynamicHelp && (strings.Contains(js, "localWikiToggle('"+field+"'") || strings.Contains(js, "localWikiSelect('"+field+"'"))
		if !literal && !viaHelper {
			t.Errorf("cfg/local_wikipedia.js renders no help text %s", key)
		}
	}
	bundles := map[string]map[string]string{}
	for _, locale := range localWikipediaLocales {
		bundles[locale] = localWikipediaHelpBundle(t, locale)
	}
	for _, field := range localWikipediaHelpFields {
		key := "help.local_wikipedia." + field
		english := bundles["en"][key]
		if strings.TrimSpace(english) == "" {
			t.Errorf("en is missing %s", key)
			continue
		}
		for _, locale := range localWikipediaLocales {
			value := bundles[locale][key]
			switch {
			case strings.TrimSpace(value) == "":
				t.Errorf("%s is missing %s", locale, key)
			case locale != "en" && value == english:
				t.Errorf("%s %s is still English", locale, key)
			}
		}
		if strings.Contains(" "+bundles["de"][key]+" ", " Sie ") {
			t.Errorf("German %s uses the formal Sie", key)
		}
	}
}

func localWikipediaHelpBundle(t *testing.T, locale string) map[string]string {
	t.Helper()
	bundle := map[string]string{}
	err := filepath.WalkDir("lang", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Name() != locale+".json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var values map[string]interface{}
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for key, value := range values {
			if text, ok := value.(string); ok && strings.HasPrefix(key, "help.local_wikipedia.") {
				bundle[key] = text
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk lang for %s: %v", locale, err)
	}
	return bundle
}
