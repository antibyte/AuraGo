package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"reflect"
	"testing"

	"aurago/internal/i18n"
	"aurago/ui"
)

func TestUII18NSectionsManifestCoversEveryPage(t *testing.T) {
	t.Parallel()

	want := map[string][]string{
		"config":      {"config", "chat", "help", "shared"},
		"dashboard":   {"dashboard", "chat", "config", "knowledge", "shared"},
		"desktop":     {"desktop", "chat", "cheater", "codeStudio", "config", "galaxa", "homepage_studio", "missions", "pixel", "shared", "viewer", "zipper"},
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
		"chat":        {"chat", "config", "plans", "pwa", "shared", "viewer"},
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

func TestDesktopTemplateDataIncludesAllDesktopAppI18NSections(t *testing.T) {
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

	for _, key := range []string{
		"chat.sse_thinking",
		"cheater.save",
		"codeStudio.refresh",
		"desktop.chess_flip",
		"galaxa.title",
		"homepage_studio.target_label",
		"missions.filter_all",
		"pixel.adjust",
		"viewer.loading",
		"zipper.open",
	} {
		if payload.I18N[key] == "" || payload.I18N[key] == key {
			t.Fatalf("desktop template data did not include translated key %s: %q", key, payload.I18N[key])
		}
	}
}
