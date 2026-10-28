package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestConfigAgentMailAllowedSendersFieldAndTranslations(t *testing.T) {
	t.Parallel()

	agentmailJS := readDesktopAssetText(t, "cfg/agentmail.js")
	for _, marker := range []string{
		"agentMailAllowedSendersField(data.allowed_senders)",
		"path: 'agentmail.allowed_senders'",
		`data-type="array-lines"`,
		"config.agentmail.allowed_senders_label",
		"help.agentmail.allowed_senders",
	} {
		if !strings.Contains(agentmailJS, marker) {
			t.Fatalf("AgentMail config UI missing allowed-senders marker %q", marker)
		}
	}

	files, err := filepath.Glob(filepath.Join("lang", "config", "agentmail", "*.json"))
	if err != nil {
		t.Fatalf("glob AgentMail locales: %v", err)
	}
	if len(files) != 16 {
		t.Fatalf("AgentMail locale files = %d, want all 16", len(files))
	}
	var english string
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, key := range []string{"config.agentmail.allowed_senders_label", "help.agentmail.allowed_senders"} {
			if strings.TrimSpace(entries[key]) == "" {
				t.Fatalf("%s missing non-empty %s", file, key)
			}
		}
		help := entries["help.agentmail.allowed_senders"]
		for _, example := range []string{"anna@example.com", "@example.com"} {
			if !strings.Contains(help, example) {
				t.Fatalf("%s help must show the entry format %q: %s", file, example, help)
			}
		}
		if filepath.Base(file) == "en.json" {
			english = help
		}
	}
	for _, file := range files {
		if filepath.Base(file) == "en.json" {
			continue
		}
		raw, _ := os.ReadFile(file)
		var entries map[string]string
		_ = json.Unmarshal(raw, &entries)
		if entries["help.agentmail.allowed_senders"] == english {
			t.Fatalf("%s still carries the English help text", file)
		}
	}
}
