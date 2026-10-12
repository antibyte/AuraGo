package ui

import (
	"fmt"
	"strings"
	"testing"

	"aurago/internal/cydcreator"
)

// TestCYDCreatorSerialFlasherMonitorModules checks the plan 5 serial, flasher and monitor modules:
// module shape (purpose comment, strict mode, namespace, line budget) and their contract markers,
// including the coordinator decisions D1 (install modes) and D2 (desktop serial policy).
func TestCYDCreatorSerialFlasherMonitorModules(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-serial.js",
		"P.serial = { support, classify, createManager, manager: createManager(), RESET_PULSE_MS };",
		"isSecureContext", "nav.serial", "allowsFeature('serial')", "requestToSend: true", "requestPort(", "revocable", "'disconnect'",
		"'aurago:desktop-policy'", "serial_browser_enabled", "reason: 'disabled'", "reason: 'readonly'")
	checkCYDCreatorModule(t, "cyd-creator-flasher.js",
		"const ESPTOOL_URL = '/js/vendor/esptool-js/esptool-js.min.mjs';", "import(ESPTOOL_URL)", "const BAUDS = [921600, 460800, 115200];",
		"'CYD_FLASH_WRONG_CHIP'", "calculateMD5Hash: md5Hex", "fileArray: files", "P.api.mintDevice(", "P.api.revokeDevice(",
		"verifyImage(bytes", "manager.request()", "'X-CYD-SHA256'", "o.monitor.connect({ reset: true })", "P.flasher = {",
		"const APP_OFFSET = 0x10000;", "const OTADATA_OFFSET = 0xE000;", "const OTADATA_SIZE = 0x2000;", "owner: inst.projectBag || inst.bag",
		"'cyd_creator.flash.error_device_url'", "'cyd_creator.flash.error_tls_pin'", "monitor: monitorBox.checked && !!inst.monitor",
		"loader.flashMd5sum(PART_TABLE_OFFSET, PART_TABLE_SIZE)", "configOffset: board.config_offset", "hasLink(images.app || images.merged)")
	checkCYDCreatorModule(t, "cyd-creator-monitor.js",
		"inst.dock.registerTab('monitor'", "labelKey: 'cyd_creator.dock.monitor'", "const MAX_LINES = 5000;", "const ATTACH_MAX_BYTES = 16 * 1024;",
		".attachLog(", "P.api.decodeBacktrace(", "revocable: true", "P.flasher.lastFlashed(", "P.monitor = {", "dispose() {")
}

// TestCYDCreatorFlasherOffsetsMatchPartitionTables pins the flasher's fixed offsets (keep settings,
// the erased config partition) to the Go partition tables, and checks that every board's cydcc offset
// (served as caps.boards[].config_offset) passes the flasher's offset rule.
func TestCYDCreatorFlasherOffsetsMatchPartitionTables(t *testing.T) {
	text := rawDesktopAssetText(t, "js/desktop/apps/cyd-creator-flasher.js")
	for _, mb := range []int{4, 8, 16} {
		table, err := cydcreator.PartitionTable(mb)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range table {
			var marker string
			switch p.Name {
			case "app0":
				marker = fmt.Sprintf("const APP_OFFSET = 0x%X;", p.Offset)
			case "otadata":
				marker = fmt.Sprintf("const OTADATA_OFFSET = 0x%X;", p.Offset)
				if !strings.Contains(text, fmt.Sprintf("const OTADATA_SIZE = 0x%X;", p.Size)) {
					t.Errorf("%d MB: otadata is 0x%X bytes, the flasher erases a different size", mb, p.Size)
				}
			case "cydcc":
				marker = fmt.Sprintf("const CONFIG_SIZE = 0x%X;", p.Size)
				if p.Offset < 0x10000 || p.Offset%p.Size != 0 {
					t.Errorf("%d MB: cydcc offset 0x%X fails the flasher's offset rule", mb, p.Offset)
				}
			case "nvs":
				// The partition table (0x8000, 3 KiB) the automatic mode compares ends before the NVS.
				marker = "const PART_TABLE_OFFSET = 0x8000;"
				if p.Offset < 0x8000+0xC00 {
					t.Errorf("%d MB: nvs at 0x%X overlaps the partition table area", mb, p.Offset)
				}
			default:
				continue
			}
			if !strings.Contains(text, marker) {
				t.Errorf("%d MB: the flasher misses %q (internal/cydcreator/partitions.go)", mb, marker)
			}
		}
	}
	boards, err := cydcreator.Boards()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range boards {
		if _, err := cydcreator.ConfigPartitionOffset(b.FlashMB); err != nil {
			t.Errorf("board %s (%d MB) has no config partition offset: %v", b.ID, b.FlashMB, err)
		}
	}
}
