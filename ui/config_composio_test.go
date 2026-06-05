package ui

import (
	"strings"
	"testing"
)

func TestConfigComposioAPIKeyParticipatesInNormalConfigSave(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"async function renderComposioSection",
		`data-path="composio.api_key"`,
		`key: 'composio_api_key'`,
		"/api/composio/status",
		"/api/vault/secrets",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio config module missing marker %q", marker)
		}
	}
	if strings.Contains(composioJS, "alert(") {
		t.Fatal("composio config module must use modals/toasts instead of alert()")
	}
}

func TestConfigComposioPickerUsesOpaqueModalSurfaces(t *testing.T) {
	t.Parallel()

	configCSS := strings.ReplaceAll(readDesktopAssetText(t, "css/config.css"), "\r\n", "\n")
	for _, marker := range []string{
		".cmp-modal-overlay",
		"background: rgba(2, 6, 23, 0.86);",
		"backdrop-filter: blur(8px);",
		".cmp-modal {\n",
		"background: var(--bg-primary);",
		".cmp-list-panel {\n    border-right: 1px solid var(--border-subtle);\n    background: var(--bg-secondary);",
		".cmp-detail-panel {\n    padding: 1.1rem;\n    background: var(--bg-primary);",
	} {
		if !strings.Contains(configCSS, marker) {
			t.Fatalf("composio modal CSS missing opaque surface marker %q", marker)
		}
	}
}
