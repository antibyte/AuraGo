package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// confirmBeforeAction asserts that, within the code following marker, a
// showConfirm call precedes the first destructive token.
func confirmBeforeAction(t *testing.T, file, marker string, stops ...string) {
	t.Helper()
	src := string(mustReadUIFile(t, "cfg/"+file))
	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatalf("%s: marker %q not found", file, marker)
	}
	end := start + 1600
	if end > len(src) {
		end = len(src)
	}
	body := src[start:end]
	confirm := strings.Index(body, "showConfirm(")
	if confirm < 0 {
		t.Fatalf("%s: %q does not call showConfirm", file, marker)
	}
	for _, stop := range stops {
		if i := strings.Index(body, stop); i >= 0 && i < confirm {
			t.Errorf("%s: %q runs %q before showConfirm", file, marker, stop)
		}
	}
}

func TestConfigWave0DestructiveActionsConfirmFirst(t *testing.T) {
	confirmBeforeAction(t, "backup.js", "async function backupImport()", "fetch(", "btn.disabled = true")
	confirmBeforeAction(t, "security_proxy.js", "async function _proxyAction(", "fetch(", "disabled = true")
	confirmBeforeAction(t, "dograh.js", "async function dograhAction(", "fetch(", "btn.disabled = true")
	confirmBeforeAction(t, "local_llm.js", "async function localLLMAction(", "fetch(", "_localLLMActionBusy = true")
	confirmBeforeAction(t, "space_agent.js", "async function spaceAgentRecreate()", "fetch(", "btn.disabled = true")
	confirmBeforeAction(t, "server.js", "async function _srvRegenCert()", "fetch(", "disabled = true")
	confirmBeforeAction(t, "go2rtc.js", "function go2rtcRemoveStream(", "splice(", "go2rtcSetStreams(")
	confirmBeforeAction(t, "gog.js", "remove.addEventListener('click'", "splice(")
	confirmBeforeAction(t, "gog.js", "function button(parent, label, work, confirmKeys)", "action(b, work)")
	confirmBeforeAction(t, "google_workspace.js", "async function gwOAuthDisconnect()", "fetch(", "disabled = true")
	confirmBeforeAction(t, "realtime_speech.js", "document.querySelectorAll('[data-rs-delete]')", "splice(")
	confirmBeforeAction(t, "three_d_printers.js", "async function threeDPrinterRemove(", "splice(")
	confirmBeforeAction(t, "a2a.js", "async function a2aRemoveSkill(", "splice(")
	confirmBeforeAction(t, "a2a.js", "async function a2aRemoveRemoteAgent(", "splice(")

	gog := string(mustReadUIFile(t, "cfg/gog.js"))
	if !strings.Contains(gog, "'confirm_disconnect_title', 'confirm_disconnect_message'") {
		t.Error("gog disconnect does not use its confirm keys")
	}
	if strings.Contains(gog, "authLink.focus(); });") {
		t.Error("gog connect must return the login-opened message instead of reporting success")
	}
}

func TestConfigWave0BackupEscapesServerMessage(t *testing.T) {
	src := string(mustReadUIFile(t, "cfg/backup.js"))
	if strings.Contains(src, "${data.message}") {
		t.Error("backup import inserts the server message unescaped")
	}
	if strings.Contains(src, "'Unknown error'") || strings.Contains(src, "title=\"Toggle visibility\"") {
		t.Error("backup still has hard-coded English texts")
	}
}

func TestConfigWave0RealtimeSpeechExposesUnsavedHook(t *testing.T) {
	src := string(mustReadUIFile(t, "cfg/realtime_speech.js"))
	for _, want := range []string{
		"window.realtimeSpeechHasUnsavedChanges",
		"rsMarkClean();",
		"config.realtime_speech.save_note",
		"data-rs-test=\"${index}\"",
		"data-rs-refresh-voices=\"${index}\"",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("realtime_speech.js misses %q", want)
		}
	}
}

func TestConfigWave0ConfirmLocalesHaveNewKeys(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	files := map[string][]string{
		"lang/config/backup/%s.json":           {"config.backup.confirm_import_title", "config.backup.confirm_import_message", "config.backup.unknown_error", "config.backup.toggle_visibility"},
		"lang/config/security_proxy/%s.json":   {"config.security_proxy.confirm_destroy_title", "config.security_proxy.confirm_destroy_message"},
		"lang/config/%s.json":                  {"config.dograh.confirm_recreate_title", "config.dograh.confirm_recreate_message", "config.local_llm.confirm_recreate_title", "config.local_llm.confirm_recreate_message", "config.local_llm.result_ok", "config.realtime_speech.confirm_delete_title", "config.realtime_speech.confirm_delete_message", "config.realtime_speech.save_note"},
		"lang/config/sections/%s.json":         {"config.space_agent.confirm_recreate_title", "config.space_agent.confirm_recreate_message"},
		"lang/config/server/%s.json":           {"config.server.confirm_regen_title", "config.server.confirm_regen_message"},
		"lang/config/go2rtc/%s.json":           {"config.go2rtc.confirm_remove_title", "config.go2rtc.confirm_remove_message"},
		"lang/config/gog/%s.json":              {"config.gog.confirm_disconnect_title", "config.gog.confirm_disconnect_message", "config.gog.confirm_remove_title", "config.gog.confirm_remove_message", "config.gog.login_opened", "config.gog.login_not_connected"},
		"lang/config/google_workspace/%s.json": {"config.google_workspace.confirm_disconnect_title", "config.google_workspace.confirm_disconnect_message", "config.google_workspace.disconnecting"},
		"lang/config/three_d_printers/%s.json": {"config.three_d_printers.confirm_remove_title", "config.three_d_printers.confirm_remove_message"},
		"lang/config/a2a/%s.json":              {"config.a2a.confirm_remove_skill_title", "config.a2a.confirm_remove_skill_message", "config.a2a.confirm_remove_agent_title", "config.a2a.confirm_remove_agent_message"},
	}
	for pattern, keys := range files {
		for _, locale := range locales {
			path := fmt.Sprintf(pattern, locale)
			var values map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, path), &values); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(values[key]) == "" {
					t.Errorf("%s misses %s", path, key)
				}
			}
		}
	}
}
