package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// cydCreatorFlashModules are the plan 5 modules in loader order (below the "plan 5" marker).
var cydCreatorFlashModules = []string{"cyd-creator-serial.js", "cyd-creator-flasher.js", "cyd-creator-monitor.js", "cyd-creator-devices.js", "cyd-creator-transfer.js"}

// TestCYDCreatorFlashKeysExistInAllLocales scans the plan 5 modules that exist for full "cyd_creator.…"
// key literals (dynamic families need a member list here). Every used key exists in all sixteen locales,
// every locale has the English flash/monitor/devices/transfer key set, and German uses "du".
func TestCYDCreatorFlashKeysExistInAllLocales(t *testing.T) {
	families := map[string][]string{
		// disabled/readonly: the desktop serial policy (virtual_desktop.serial_browser_enabled, read-only desktop).
		"cyd_creator.flash.unsupported_": {"insecure", "unsupported", "framed", "policy", "disabled", "readonly"},
		"cyd_creator.flash.phase_":       {"port", "cancelled", "device", "download", "connect", "write", "reset", "done", "failed"},
		"cyd_creator.monitor.status_":    {"disconnected", "connected", "lost", "flashing"},
		"cyd_creator.devices.state_":     {"pending", "online", "offline"},
		// CYD_FILE_LIMIT by details.limit (app_files covers file_bytes, files and total_bytes) and
		// CYD_BUSY by details.reason on export.
		"cyd_creator.transfer.limit_": {"asset_bytes", "export_bytes", "app_files"},
		"cyd_creator.transfer.busy_":  {"export", "publication_review"},
		// CYD_IMPORT_INVALID by details.reason (internal/cydcreator/transfer_import.go).
		"cyd_creator.transfer.import_": {"not_zip", "too_large", "too_many_entries", "unsafe_path", "layout_missing", "file_type", "layout", "board", "app_files", "asset"},
	}
	// error.<CODE> is plan 4a's family (every overview code); 4a's key test covers its members.
	ignored := map[string]bool{"cyd_creator.error.": true}
	used := map[string]bool{}
	for _, name := range cydCreatorFlashModules {
		data, err := os.ReadFile(filepath.Join("js", "desktop", "apps", name))
		if errors.Is(err, fs.ErrNotExist) {
			continue // the module task adds it later; its keys are checked from then on
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cydCreatorKeyLiteral.FindAllStringSubmatch(string(data), -1) {
			key := m[1]
			if m[2] != "" || strings.HasSuffix(key, "_") || strings.HasSuffix(key, ".") {
				if ignored[key] {
					continue
				}
				if _, ok := families[key]; !ok {
					t.Errorf("dynamic key family %s (in %s) has no member list in this test", key, name)
				}
				continue
			}
			used[key] = true
		}
	}
	for prefix, members := range families {
		for _, member := range members {
			used[prefix+member] = true
		}
	}
	plan5 := regexp.MustCompile(`^cyd_creator\.(flash|monitor|devices|transfer)\.`)
	en := readCYDCreatorShellLang(t, "en")
	for _, lang := range cydCreatorLocales {
		words := readCYDCreatorShellLang(t, lang)
		for key := range used {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("lang/cyd_creator/%s.json misses %s", lang, key)
			}
		}
		for key := range en {
			if _, ok := words[key]; plan5.MatchString(key) && !ok {
				t.Errorf("lang/cyd_creator/%s.json misses %s (present in en)", lang, key)
			}
		}
	}
	de := readCYDCreatorShellLang(t, "de")
	formal := regexp.MustCompile(`\b(Sie|Ihnen|Ihre[nmrs]?)\b`)
	for key := range used {
		if formal.MatchString(de[key]) {
			t.Errorf("de %s addresses the user formally: %q", key, de[key])
		}
	}
	if !strings.Contains(de["cyd_creator.flash.ready"], "„") || !strings.Contains(de["cyd_creator.devices.title"], "ä") {
		t.Error("German plan 5 strings must use German quotes and real umlauts")
	}
}
