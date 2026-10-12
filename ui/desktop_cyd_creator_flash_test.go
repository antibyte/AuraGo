package ui

import "testing"

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
		"verifyImage(bytes", "manager.request()", "'X-CYD-SHA256'", "inst.monitor.connect({ reset: true })", "P.flasher = {",
		"const APP_OFFSET = 0x10000;", "const OTADATA_OFFSET = 0xE000;", "const OTADATA_SIZE = 0x2000;", "owner: inst.projectBag || inst.bag",
		"'cyd_creator.flash.error_device_url'", "'cyd_creator.flash.error_tls_pin'")
	checkCYDCreatorModule(t, "cyd-creator-monitor.js",
		"inst.dock.registerTab('monitor'", "labelKey: 'cyd_creator.dock.monitor'", "const MAX_LINES = 5000;", "const ATTACH_MAX_BYTES = 16 * 1024;",
		".attachLog(", "P.api.decodeBacktrace(", "revocable: true", "P.flasher.lastFlashed(", "P.monitor = {", "dispose() {")
}
