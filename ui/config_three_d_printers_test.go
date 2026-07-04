package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThreeDPrintersTranslationsKeepNativeDiacritics(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("lang", "config", "three_d_printers", "*.json"))
	if err != nil {
		t.Fatalf("glob three_d_printers translations: %v", err)
	}
	if len(files) != 16 {
		t.Fatalf("three_d_printers translations = %d files, want 16", len(files))
	}

	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
	}

	for lang, badValues := range map[string][]string{
		"cs": {"Pridat", "tiskarnu", "klic", "Volitelny", "prazdne", "duveryhodne"},
		"da": {"Tilfoj"},
		"de": {"Schlussel", "Fur", "uber"},
		"es": {"Anadir", "Anade", "Dejala", "vacia", "integracion", "impresion", "estandar"},
		"fr": {"Cle API", "approuves", "l integration", "l etat", "controle d impression", "l API", "l hote"},
		"no": {"nokkel", "La sta"},
	} {
		raw, err := os.ReadFile(filepath.Join("lang", "config", "three_d_printers", lang+".json"))
		if err != nil {
			t.Fatalf("read %s translations: %v", lang, err)
		}
		text := string(raw)
		for _, bad := range badValues {
			if strings.Contains(text, bad) {
				t.Fatalf("%s.json contains ASCII replacement %q:\n%s", lang, bad, text)
			}
		}
	}
}
