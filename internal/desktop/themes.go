package desktop

// DefaultDesktopTheme is the theme of a fresh desktop.
const DefaultDesktopTheme = "standard"

// DesktopThemeIDs returns the selectable Virtual Desktop themes in picker order. DESKTOP_THEMES in
// ui/js/desktop/core/theme-runtime.js must list the same ids in the same order (ui/desktop_theme_registry_test.go).
func DesktopThemeIDs() []string {
	return []string{"standard", "fruity", "amiga", "cyberpunk", "black_gloss", "crazy", "lollipop"}
}
