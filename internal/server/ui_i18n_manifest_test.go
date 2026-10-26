package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/i18n"
	"aurago/ui"
)

func TestUII18NSectionsManifestCoversEveryPage(t *testing.T) {
	t.Parallel()

	want := map[string][]string{
		"config":      {"config", "chat", "help", "security", "shared", "ruview", "personality_dynamics", "personality_legacy"},
		"dashboard":   {"dashboard", "chat", "config", "knowledge", "shared", "personality_dynamics"},
		"desktop":     {"desktop"},
		"plans":       {"plans", "chat", "config", "shared"},
		"missions":    {"missions", "chat", "config", "shared"},
		"cheatsheets": {"cheatsheets", "chat", "config", "shared"},
		"media":       {"media", "gallery", "chat", "config", "shared"},
		"knowledge":   {"knowledge", "chat", "config", "shared"},
		"containers":  {"containers", "chat", "config", "shared"},
		"truenas":     {"truenas", "chat", "config", "shared"},
		"skills":      {"skills", "chat", "config", "shared"},
		"invasion":    {"invasion", "chat", "config", "shared"},
		"setup":       {"setup", "chat", "config", "shared"},
		"chat":        {"chat", "config", "plans", "pwa", "shared", "viewer", "personality_legacy"},
		"404":         {"notfound"},
	}

	if len(uiI18NSections) != len(want) {
		t.Fatalf("manifest has %d pages, want %d", len(uiI18NSections), len(want))
	}
	for page, sections := range want {
		if got := uiI18NSections[page]; !reflect.DeepEqual(got, sections) {
			t.Errorf("uiI18NSections[%q] = %#v, want %#v", page, got, sections)
		}
	}
}

func TestPersonalityTemplateDataIncludesTranslations(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	pages := map[string][]string{
		"config":    {"personality_dynamics.", "personality_legacy."},
		"dashboard": {"personality_dynamics."},
		"chat":      {"personality_legacy."},
	}
	for _, lang := range strings.Fields("cs da de el en es fr hi it ja nl no pl pt sv zh") {
		common, err := fs.ReadFile(ui.Content, "lang/common/"+lang+".json")
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(common, &labels); err != nil {
			t.Fatal(err)
		}
		for page, prefixes := range pages {
			t.Run(lang+"/"+page, func(t *testing.T) {
				data := uiTemplateData(lang, page)
				var payload struct {
					I18N map[string]string `json:"i18n"`
				}
				if err := json.Unmarshal([]byte(fmt.Sprint(data["TemplateDataJSON"])), &payload); err != nil {
					t.Fatal(err)
				}
				for _, prefix := range prefixes {
					found := false
					for key, want := range labels {
						if strings.HasPrefix(key, prefix) {
							found = true
							if want == "" || payload.I18N[key] != want {
								t.Errorf("page payload missing localized key %s", key)
							}
						}
					}
					if !found {
						t.Errorf("locale missing namespace %s", prefix)
					}
				}
			})
		}
	}
}

func TestDesktopTemplateDataIsShellOnlyI18N(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal(err)
	}
	i18n.Load(uiFS, slog.Default())

	data := uiTemplateData("de", "desktop")
	var payload struct {
		I18N map[string]string `json:"i18n"`
	}
	if err := json.Unmarshal([]byte(fmt.Sprint(data["TemplateDataJSON"])), &payload); err != nil {
		t.Fatalf("parse desktop template data: %v", err)
	}

	// Shell must still include desktop + common keys.
	for _, key := range []string{
		"desktop.chess_flip",
		"common.skip_to_content",
	} {
		if payload.I18N[key] == "" || payload.I18N[key] == key {
			t.Fatalf("desktop shell missing key %s: %q", key, payload.I18N[key])
		}
	}

	// App / config prefixes must not bloat the initial desktop HTML payload.
	forbiddenPrefixes := []string{
		"config.", "chat.", "cheater.", "codeStudio.", "galaxa.",
		"homepage_studio.", "missions.", "pixel.", "viewer.", "zipper.",
		"dashboard.", "help.",
	}
	for key := range payload.I18N {
		for _, prefix := range forbiddenPrefixes {
			if strings.HasPrefix(key, prefix) {
				t.Fatalf("desktop shell must not embed %s keys, found %q", prefix, key)
			}
		}
	}
}
