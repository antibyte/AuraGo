package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Contract test (string level, no browser) for the staged-restore UI of the
// backup page: result rendering, restart gating, status polling, i18n.
func TestBackupRestoreStagedUIContract(t *testing.T) {
	src := string(mustReadUIFile(t, "cfg/backup.js"))
	for _, want := range []string{
		"/api/backup/restore-status",
		"data.restart_required === true",
		"outcome.state",
		"data.warnings",
		"li.textContent = String(w)",
		"msg.textContent = String(data.message)",
		"config.backup.error_http",
		"config.backup.restore_pending_since",
		"config.backup.last_restore_",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("backup.js misses %q", want)
		}
	}
	if strings.Contains(src, "innerHTML = `<div class=\"backup-import-success-box\"") || strings.Contains(src, "result.innerHTML") {
		t.Error("backup import result must not be built with innerHTML")
	}
	// The restart button belongs only to the restart_required branch.
	idx := strings.Index(src, "function backupRenderImportResult")
	if idx < 0 || strings.Index(src[idx:], "backupRestartButton()") < strings.Index(src[idx:], "restart_required") {
		t.Error("restart button is not gated by restart_required")
	}
}

func TestBackupRestoreStagedLocales(t *testing.T) {
	keys := []string{"import_staged", "import_partial", "restore_pending_note", "error_http", "restore_pending_since",
		"last_restore_applied", "last_restore_rolled_back", "last_restore_failed"}
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	en := map[string]string{}
	for _, loc := range locales {
		var m map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/config/backup/%s.json", loc)), &m); err != nil {
			t.Fatalf("%s: %v", loc, err)
		}
		for _, k := range keys {
			v := m["config.backup."+k]
			if v == "" {
				t.Errorf("%s misses config.backup.%s", loc, k)
			}
			if loc == "en" {
				en[k] = v
			} else if v == en[k] && k != "import_staged" {
				t.Errorf("%s: config.backup.%s is an English copy", loc, k)
			}
		}
		if !strings.Contains(m["config.backup.error_http"], "{status}") || !strings.Contains(m["config.backup.restore_pending_since"], "{time}") {
			t.Errorf("%s: placeholder missing", loc)
		}
	}
}
