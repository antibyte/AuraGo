package server

import (
	"reflect"
	"testing"
)

func TestUII18NSectionsManifestCoversEveryPage(t *testing.T) {
	t.Parallel()

	want := map[string][]string{
		"config":      {"config", "chat", "shared"},
		"dashboard":   {"dashboard", "chat", "config", "knowledge", "shared"},
		"desktop":     {"desktop", "config", "shared"},
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
