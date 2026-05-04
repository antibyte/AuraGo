package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopPapirusAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	license, err := Content.ReadFile("img/papirus/LICENSE-Papirus.txt")
	if err != nil {
		t.Fatalf("Papirus license asset missing from embedded UI: %v", err)
	}
	if !strings.Contains(string(license), "GNU GENERAL PUBLIC LICENSE") {
		t.Fatal("Papirus license asset does not contain the GPL license text")
	}

	data, err := Content.ReadFile("img/papirus/manifest.json")
	if err != nil {
		t.Fatalf("Papirus manifest missing from embedded UI: %v", err)
	}
	var manifest struct {
		Name         string            `json:"name"`
		Version      int               `json:"version"`
		Source       string            `json:"source"`
		License      string            `json:"license"`
		DefaultTheme string            `json:"default_theme"`
		Themes       []string          `json:"themes"`
		Icons        map[string]string `json:"icons"`
		Aliases      map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse Papirus manifest: %v", err)
	}
	if manifest.Name != "Papirus Icon Theme" || manifest.Version != 1 {
		t.Fatalf("unexpected Papirus manifest identity: name=%q version=%d", manifest.Name, manifest.Version)
	}
	if manifest.Source != "https://github.com/PapirusDevelopmentTeam/papirus-icon-theme" {
		t.Fatalf("unexpected Papirus manifest source %q", manifest.Source)
	}
	if manifest.License != "GPL-3.0" {
		t.Fatalf("unexpected Papirus manifest license %q", manifest.License)
	}
	if manifest.DefaultTheme != "Papirus" {
		t.Fatalf("unexpected Papirus default theme %q", manifest.DefaultTheme)
	}
	for _, want := range []string{"Papirus", "Papirus-Dark", "Papirus-Light"} {
		if !containsString(manifest.Themes, want) {
			t.Fatalf("Papirus manifest missing theme %q", want)
		}
	}

	for _, key := range []string{
		"analytics", "apps", "archive", "audio", "backup", "book", "browser", "calendar",
		"calculator", "camera", "chat", "cloud", "code", "css", "database", "desktop",
		"documents", "downloads", "editor", "folder", "forms", "globe", "go", "help",
		"home", "html", "image", "javascript", "json", "key", "mail", "markdown",
		"map", "monitor", "network", "notes", "package", "pdf", "phone", "printer",
		"python", "server", "settings", "spreadsheet", "terminal", "text", "tools",
		"trash", "video", "weather", "workflow", "xml", "yaml",
		"arrow-up", "check-square", "chevron-down", "chevron-left", "chevron-right",
		"chevron-up", "clipboard", "copy", "download", "file-plus", "folder-open",
		"folder-plus", "grid", "info", "list", "refresh", "run", "save", "scissors",
		"search", "sort", "stop", "upload", "x",
	} {
		path, ok := manifest.Icons[key]
		if !ok {
			t.Fatalf("Papirus manifest missing required icon %q", key)
		}
		if !strings.HasPrefix(path, "img/papirus/icons/") || !strings.HasSuffix(path, ".svg") {
			t.Fatalf("Papirus icon %q has invalid path %q", key, path)
		}
		svg, err := Content.ReadFile(path)
		if err != nil {
			t.Fatalf("Papirus icon %q not embedded at %s: %v", key, path, err)
		}
		if !strings.Contains(string(svg), "<svg") {
			t.Fatalf("Papirus icon %q is not an SVG asset", key)
		}
	}

	for alias, target := range manifest.Aliases {
		if _, ok := manifest.Icons[target]; !ok {
			t.Fatalf("Papirus alias %q targets missing icon %q", alias, target)
		}
	}
}

func TestDesktopPapirusManifestMatchesBackendIconCatalog(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("img/papirus/manifest.json")
	if err != nil {
		t.Fatalf("Papirus manifest missing from embedded UI: %v", err)
	}
	var manifest struct {
		Icons   map[string]string `json:"icons"`
		Aliases map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse Papirus manifest: %v", err)
	}

	catalog := desktop.DesktopIconCatalog(map[string]string{"appearance.icon_theme": "papirus"})
	for _, icon := range catalog.Preferred {
		if _, ok := manifest.Icons[icon]; !ok {
			t.Fatalf("backend icon catalog preferred icon %q is missing from Papirus manifest", icon)
		}
	}
	for icon := range manifest.Icons {
		if !containsString(catalog.Preferred, icon) {
			t.Fatalf("Papirus manifest icon %q is missing from backend icon catalog", icon)
		}
	}
	for alias, target := range catalog.Aliases {
		if got := manifest.Aliases[alias]; got != target {
			t.Fatalf("Papirus manifest alias %q = %q, want backend target %q", alias, got, target)
		}
	}
	for alias, target := range manifest.Aliases {
		if got := catalog.Aliases[alias]; got != target {
			t.Fatalf("backend icon catalog alias %q = %q, want Papirus target %q", alias, got, target)
		}
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
