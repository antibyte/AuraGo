package ui

import (
	"strings"
	"testing"
)

func TestConfigAgentMailRelayCheatsheetDropdownContract(t *testing.T) {
	t.Parallel()

	agentmailJS := readDesktopAssetText(t, "cfg/agentmail.js")
	for _, marker := range []string{
		"agentMailCheatsheetSelect(",
		`data-path="agentmail.relay_cheatsheet_id"`,
		"/api/cheatsheets",
		"config.agentmail.relay_cheatsheet_label",
		"config.agentmail.relay_cheatsheet_none",
	} {
		if !strings.Contains(agentmailJS, marker) {
			t.Fatalf("AgentMail config UI missing relay cheatsheet marker %q", marker)
		}
	}
	if strings.Contains(agentmailJS, "alert(") {
		t.Fatal("AgentMail config UI must use modals/toasts instead of alert()")
	}
}
