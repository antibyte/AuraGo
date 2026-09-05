package ui

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

func TestDesktopMeshCoreTranslationsAndIntegration(t *testing.T) {
	b, err := Content.ReadFile("lang/desktop/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline map[string]string
	if err = json.Unmarshal(b, &baseline); err != nil {
		t.Fatal(err)
	}
	files, _ := fs.Glob(Content, "lang/desktop/*.json")
	if len(files) != 16 {
		t.Fatalf("locales: %d", len(files))
	}
	for _, file := range files {
		b, _ := Content.ReadFile(file)
		var values map[string]string
		if err = json.Unmarshal(b, &values); err != nil {
			t.Fatal(err)
		}
		for k := range baseline {
			if (strings.HasPrefix(k, "desktop.meshcore_") || k == "desktop.app_meshcore") && strings.TrimSpace(values[k]) == "" {
				t.Errorf("%s missing %s", file, k)
			}
		}
	}
	for file, want := range map[string]string{
		"js/desktop/core/module-loader.js":        "apps/meshcore.js",
		"js/desktop/core/desktop-foundation.js":   "MeshCoreApp",
		"js/desktop/core/menus-and-routing.js":    "MeshCoreApp.render",
		"js/desktop/core/session-runtime.js":      "conversation_id",
		"js/desktop/core/sdk-events-bootstrap.js": "meshcore_changed",
		"js/desktop/core/shell-chrome-runtime.js": "entry?.context",
	} {
		b, err := Content.ReadFile(file)
		if err != nil || !strings.Contains(string(b), want) {
			t.Errorf("%s missing %s", file, want)
		}
	}
}
