package ui

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranslations_NoConflictingCrossSectionValues(t *testing.T) {
	t.Parallel()
	type definition struct {
		value string
		path  string
	}
	known := map[string]map[string]definition{}
	err := filepath.WalkDir("lang", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		lang := strings.TrimSuffix(entry.Name(), ".json")
		if len(lang) != 2 {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var values map[string]string
		if err := json.Unmarshal(content, &values); err != nil {
			return err
		}
		if known[lang] == nil {
			known[lang] = map[string]definition{}
		}
		for key, value := range values {
			if earlier, exists := known[lang][key]; exists && earlier.value != value {
				t.Errorf("%s %s differs between %s and %s", lang, key, earlier.path, path)
			}
			known[lang][key] = definition{value, path}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
