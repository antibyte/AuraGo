package ui

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The outline, search, problems and output panels all render their empty state
// with the shared .cs-empty class. Without a horizontal padding that text sits
// flush against the panel border, so the rule has to keep a visible gutter.
func TestCodeStudioEmptyStateKeepsPanelGutter(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/code-studio.css")
	rule := regexp.MustCompile(`(?s)\.cs-empty\s*\{(.*?)\}`).FindStringSubmatch(css)
	if len(rule) != 2 {
		t.Fatal("code studio stylesheet is missing the .cs-empty rule")
	}
	padding := regexp.MustCompile(`padding:\s*([^;]+);`).FindStringSubmatch(rule[1])
	if len(padding) != 2 {
		t.Fatal(".cs-empty must declare a padding so the empty text keeps a gutter")
	}
	fields := strings.Fields(padding[1])
	if len(fields) < 2 {
		t.Fatalf(".cs-empty padding %q must set a horizontal gutter", padding[1])
	}
	left, err := strconv.Atoi(strings.TrimSuffix(fields[0], "px"))
	if err != nil {
		t.Fatalf(".cs-empty padding %q must use pixel values: %v", padding[1], err)
	}
	if left < 8 {
		t.Fatalf(".cs-empty left padding is %dpx, want at least 8px of gutter", left)
	}

	for _, path := range []string{
		filepath.Join("js", "desktop", "apps", "code-studio", "outline.js"),
		filepath.Join("js", "desktop", "apps", "code-studio", "search.js"),
		filepath.Join("js", "desktop", "apps", "code-studio", "problems.js"),
		filepath.Join("js", "desktop", "apps", "code-studio", "panel.js"),
	} {
		if !strings.Contains(readDesktopAssetText(t, path), `class="cs-empty"`) {
			t.Fatalf("%s must render its empty state with the shared .cs-empty class", path)
		}
	}
}
