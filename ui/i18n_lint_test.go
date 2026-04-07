package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTranslations_MultimodalKeysAreTranslated(t *testing.T) {
	t.Parallel()

	type check struct {
		relPath string
		key     string
	}
	checks := []check{
		{relPath: filepath.Join("lang", "help"), key: "help.llm.multimodal"},
		{relPath: filepath.Join("lang", "help"), key: "help.llm.multimodal_provider_types_extra"},
		{relPath: filepath.Join("lang", "config", "misc"), key: "config.llm.multimodal_banner"},
	}

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

	for _, c := range checks {
		enFile := filepath.Join(c.relPath, "en.json")
		en, err := readJSONFileMap(enFile)
		if err != nil {
			t.Fatalf("read %s: %v", enFile, err)
		}
		enVal, ok := en[c.key].(string)
		if !ok || enVal == "" {
			t.Fatalf("missing or non-string key %q in %s", c.key, enFile)
		}

		for _, lang := range langs {
			f := filepath.Join(c.relPath, lang+".json")
			m, err := readJSONFileMap(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			vAny, ok := m[c.key]
			if !ok {
				t.Fatalf("missing key %q in %s", c.key, f)
			}
			v, ok := vAny.(string)
			if !ok || v == "" {
				t.Fatalf("key %q is not a non-empty string in %s", c.key, f)
			}
			if lang != "en" && v == enVal {
				t.Fatalf("key %q in %s is identical to English; needs translation", c.key, f)
			}
		}
	}
}

func readJSONFileMap(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

