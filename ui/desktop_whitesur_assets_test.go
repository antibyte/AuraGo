package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopWhiteSurAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	license, err := Content.ReadFile("img/whitesur/LICENSE-WhiteSur.txt")
	if err != nil {
		t.Fatalf("WhiteSur license asset missing from embedded UI: %v", err)
	}
	if !strings.Contains(string(license), "GNU GENERAL PUBLIC LICENSE") {
		t.Fatal("WhiteSur license asset does not contain the GPL license text")
	}

	data, err := Content.ReadFile("img/whitesur/manifest.json")
	if err != nil {
		t.Fatalf("WhiteSur manifest missing from embedded UI: %v", err)
	}
	var manifest struct {
		Name           string            `json:"name"`
		Version        int               `json:"version"`
		Source         string            `json:"source"`
		SourceRevision string            `json:"source_revision"`
		License        string            `json:"license"`
		LicenseFile    string            `json:"license_file"`
		DefaultTheme   string            `json:"default_theme"`
		Themes         []string          `json:"themes"`
		Icons          map[string]string `json:"icons"`
		Aliases        map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse WhiteSur manifest: %v", err)
	}
	if manifest.Name != "WhiteSur Icon Theme" || manifest.Version != 1 {
		t.Fatalf("unexpected WhiteSur manifest identity: name=%q version=%d", manifest.Name, manifest.Version)
	}
	if manifest.Source != "https://github.com/vinceliuice/WhiteSur-icon-theme" {
		t.Fatalf("unexpected WhiteSur manifest source %q", manifest.Source)
	}
	if strings.TrimSpace(manifest.SourceRevision) == "" {
		t.Fatal("WhiteSur manifest source revision is empty")
	}
	if manifest.License != "GPL-3.0" || manifest.LicenseFile != "img/whitesur/LICENSE-WhiteSur.txt" {
		t.Fatalf("unexpected WhiteSur license metadata: license=%q file=%q", manifest.License, manifest.LicenseFile)
	}
	if manifest.DefaultTheme != "WhiteSur" || !containsString(manifest.Themes, "WhiteSur") {
		t.Fatalf("unexpected WhiteSur theme metadata: default=%q themes=%+v", manifest.DefaultTheme, manifest.Themes)
	}

	for _, key := range desktop.DesktopIconCatalog(map[string]string{"appearance.icon_theme": "whitesur"}).Preferred {
		path, ok := manifest.Icons[key]
		if !ok {
			t.Fatalf("WhiteSur manifest missing required icon %q", key)
		}
		if !strings.HasPrefix(path, "img/whitesur/icons/") || !strings.HasSuffix(path, ".svg") {
			t.Fatalf("WhiteSur icon %q has invalid path %q", key, path)
		}
		svg, err := Content.ReadFile(path)
		if err != nil {
			t.Fatalf("WhiteSur icon %q not embedded at %s: %v", key, path, err)
		}
		if !strings.Contains(string(svg), "<svg") {
			t.Fatalf("WhiteSur icon %q is not an SVG asset", key)
		}
	}

	for alias, target := range manifest.Aliases {
		if _, ok := manifest.Icons[target]; !ok {
			t.Fatalf("WhiteSur alias %q targets missing icon %q", alias, target)
		}
	}
}

func TestDesktopWhiteSurManifestMatchesBackendIconCatalog(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("img/whitesur/manifest.json")
	if err != nil {
		t.Fatalf("WhiteSur manifest missing from embedded UI: %v", err)
	}
	var manifest struct {
		Icons   map[string]string `json:"icons"`
		Aliases map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse WhiteSur manifest: %v", err)
	}

	catalog := desktop.DesktopIconCatalog(map[string]string{"appearance.icon_theme": "whitesur"})
	if catalog.Theme != "whitesur" {
		t.Fatalf("catalog theme = %q, want whitesur", catalog.Theme)
	}
	for _, icon := range catalog.Preferred {
		if _, ok := manifest.Icons[icon]; !ok {
			t.Fatalf("backend icon catalog preferred icon %q is missing from WhiteSur manifest", icon)
		}
	}
	for icon := range manifest.Icons {
		if !containsString(catalog.Preferred, icon) {
			t.Fatalf("WhiteSur manifest icon %q is missing from backend icon catalog", icon)
		}
	}
	for alias, target := range catalog.Aliases {
		if got := manifest.Aliases[alias]; got != target {
			t.Fatalf("WhiteSur manifest alias %q = %q, want backend target %q", alias, got, target)
		}
	}
	for alias, target := range manifest.Aliases {
		if got := catalog.Aliases[alias]; got != target {
			t.Fatalf("backend icon catalog alias %q = %q, want WhiteSur target %q", alias, got, target)
		}
	}
}
