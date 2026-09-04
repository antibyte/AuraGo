package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopAppErrorFallbackI18n(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "t('desktop.app_error_fallback')") {
		t.Fatal("renderAppError must translate the fallback message")
	}
	if !strings.Contains(foundation, "t('desktop.app_error_title')") {
		t.Fatal("renderAppError must keep the translated title")
	}
	if strings.Contains(foundation, "err || 'Error'") {
		t.Fatal("renderAppError still hardcodes English Error")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(values["desktop.app_error_fallback"]) == "" {
			t.Fatalf("%s missing non-empty desktop.app_error_fallback", path)
		}
		if lang == "de" && values["desktop.app_error_fallback"] != "Fehler" {
			t.Fatalf("%s German app error fallback must be Fehler", path)
		}
	}
}

func TestDesktopAppRendererMissingI18n(t *testing.T) {
	t.Parallel()

	menus := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"t('desktop.app_error_renderer_missing', { app: t('desktop.app_agent_chat') })",
		"t('desktop.app_error_renderer_missing', { app: t('desktop.app_live_speech') })",
	} {
		if !strings.Contains(menus, want) {
			t.Fatalf("renderer-missing i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"Agent chat renderer is not loaded",
		"Live Speech renderer is not loaded",
	} {
		if strings.Contains(menus, forbidden) {
			t.Fatalf("menus-and-routing still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.app_error_renderer_missing"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.app_error_renderer_missing", path)
		}
		if !strings.Contains(got, "{{app}}") {
			t.Fatalf("%s renderer-missing must keep {{app}}", path)
		}
		if lang != "en" && got == "{{app}} renderer is not loaded" {
			t.Fatalf("%s must not copy the English renderer-missing string", path)
		}
	}
}
