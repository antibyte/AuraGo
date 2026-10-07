package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvasionPlaintextHintHasALocalizedTitleOnly(t *testing.T) {
	const title = "security.hint.invasion_docker_remote_plaintext.title"
	const description = "security.hint.invasion_docker_remote_plaintext.description"
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	en, err := readJSONFileMap(filepath.Join("lang", "config", "security", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range langs {
		m, err := readJSONFileMap(filepath.Join("lang", "config", "security", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		v, _ := m[title].(string)
		if strings.TrimSpace(v) == "" || (lang != "en" && v == en[title]) {
			t.Fatalf("%s: %s missing or English", lang, title)
		}
	}
	_ = filepath.WalkDir("lang", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			if raw, _ := os.ReadFile(path); strings.Contains(string(raw), description) {
				t.Fatalf("%s translates the description; it must stay server text with the nest names", path)
			}
		}
		return nil
	})
}
