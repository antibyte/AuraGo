package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopQuickConnectUsesProtocolNeutralCopyAndFilters(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.EqualFold(strings.TrimSpace(values["desktop.qc_title"]), "SSH") {
			t.Fatalf("%s keeps SSH-only Quick Connect title", path)
		}
		if strings.Contains(strings.ToLower(values["desktop.qc_no_devices"]), "ssh server") ||
			strings.Contains(strings.ToLower(values["desktop.qc_no_devices"]), "ssh-server") {
			t.Fatalf("%s keeps SSH-only Quick Connect empty state: %q", path, values["desktop.qc_no_devices"])
		}
	}

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id)")
	for _, marker := range []string{
		`data-qc-filter="all"`,
		`data-qc-filter="ssh"`,
		`data-qc-filter="vnc"`,
		"let activeProtocolFilter = 'all'",
		"function protocolMatchesFilter(device)",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing protocol filter marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectVNCSessionToolbarMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id)")
	for _, marker := range []string{
		"function renderVNCToolbar(",
		"function setVNCStatus(",
		`class="vd-qc-vnc-toolbar"`,
		`data-vnc-scale="fit"`,
		`data-vnc-scale="native"`,
		`data-vnc-action="view-only"`,
		`data-vnc-action="ctrl-alt-del"`,
		"rfb.sendCtrlAltDel",
		"desktop.qc_vnc_status_connected",
		"desktop.qc_vnc_status_error",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing VNC toolbar marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".vd-qc-vnc-toolbar",
		".vd-qc-vnc-status",
		".vd-qc-vnc-toolbar .vd-qc-btn",
		"min-height: 44px",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("quick connect css missing VNC toolbar marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectVNCErrorCodeCopyMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id)")
	for _, marker := range []string{
		"function vncErrorText(",
		"desktop.qc_vnc_error_device_not_found",
		"desktop.qc_vnc_error_protocol_mismatch",
		"desktop.qc_vnc_error_credential_unavailable",
		"desktop.qc_vnc_error_dial_failed",
		"desktop.qc_vnc_error_auth_failed",
		"desktop.qc_vnc_error_init_failed",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing VNC error mapping marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectVNCPreservesErrorOnDisconnectMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id)")
	for _, marker := range []string{
		"let lastVNCError = null",
		"JSON.parse(reason)",
		"parsed.code",
		"parsed.message",
		"lastVNCError = message",
		"if (lastVNCError)",
		"setVNCStatus(sessionEl, 'error', lastVNCError)",
		"disconnectPlaceholderHTML('desktop.qc_vnc_connection_error', deviceId, true, lastVNCError)",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing VNC persistent error marker %q", marker)
		}
	}
}
