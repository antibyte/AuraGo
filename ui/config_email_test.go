package ui

import (
	"strings"
	"testing"
)

func TestConfigEmailRelayCheatsheetDropdownContract(t *testing.T) {
	t.Parallel()

	emailJS := readDesktopAssetText(t, "cfg/email.js")
	for _, marker := range []string{
		"emailCheatsheetSelect(",
		`data-path="email.relay_cheatsheet_id"`,
		"/api/cheatsheets",
		"config.email.relay_cheatsheet_label",
		"config.email.relay_cheatsheet_none",
	} {
		if !strings.Contains(emailJS, marker) {
			t.Fatalf("Email config UI missing relay cheatsheet marker %q", marker)
		}
	}
	if strings.Contains(emailJS, "alert(") {
		t.Fatal("Email config UI must use modals/toasts instead of alert()")
	}
}
