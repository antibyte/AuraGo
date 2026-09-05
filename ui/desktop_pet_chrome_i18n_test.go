package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPetChromeI18n(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/pet-runtime.js")
	if !strings.Contains(runtime, "spriteEl.setAttribute('aria-label', t('desktop.pet_aria_label'))") {
		t.Fatal("pet runtime fallback aria-label must use desktop.pet_aria_label")
	}
	if strings.Contains(runtime, `aria-label="Desktop pet"`) {
		t.Fatal("pet runtime still hardcodes Desktop pet")
	}

	picker := readDesktopAssetText(t, "js/desktop/apps/pet-picker.js")
	for _, want := range []string{
		"function formatScale(value)",
		"t('desktop.pet_scale_value', { value: Number(value).toFixed(1) })",
		"formatScale(1)",
		"formatScale(scaleInput.value)",
	} {
		if !strings.Contains(picker, want) {
			t.Fatalf("pet picker scale i18n missing marker %q", want)
		}
	}
	if strings.Count(picker, "formatScale(scaleInput.value)") < 2 {
		t.Fatal("pet picker must format scale on load and on input")
	}
	for _, forbidden := range []string{
		">1.0x<",
		"+ 'x'",
	} {
		if strings.Contains(picker, forbidden) {
			t.Fatalf("pet picker still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(values["desktop.pet_aria_label"]) == "" {
			t.Fatalf("%s missing non-empty desktop.pet_aria_label", path)
		}
		got := values["desktop.pet_scale_value"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.pet_scale_value", path)
		}
		if !strings.Contains(got, "{{value}}") {
			t.Fatalf("%s desktop.pet_scale_value must keep {{value}}", path)
		}
		if lang == "de" {
			if values["desktop.pet_aria_label"] == "Desktop pet" {
				t.Fatalf("%s must not copy the English pet aria-label", path)
			}
			if got == "{{value}}x" {
				t.Fatalf("%s must not copy the English pet scale suffix", path)
			}
		}
	}
}
