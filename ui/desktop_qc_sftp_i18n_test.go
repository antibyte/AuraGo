package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopQuickConnectSftpI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, want := range []string{
		"t('desktop.bytes').replace('{{count}}'",
		"t('desktop.kib').replace('{{count}}'",
		"t('desktop.mib').replace('{{count}}'",
		"t('desktop.gib').replace('{{count}}'",
		"t('desktop.tib').replace('{{count}}'",
		"t('desktop.qc_sftp_items').replace('{{count}}', 0)",
		"t('desktop.qc_sftp_items').replace('{{count}}', sftpEntries.length)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("quick connect sftp i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"statusEl.textContent = '0 items'",
		"+ ' items'",
		"+ ' KiB'",
		"+ ' MiB'",
		"+ ' GiB'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("quick connect sftp still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.qc_sftp_items"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.qc_sftp_items", path)
		}
		if !strings.Contains(got, "{{count}}") {
			t.Fatalf("%s desktop.qc_sftp_items must keep {{count}}", path)
		}
		if lang == "de" && got == "{{count}} items" {
			t.Fatalf("%s must not copy the English sftp items string", path)
		}
	}
}
