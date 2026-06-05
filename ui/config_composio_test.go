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
