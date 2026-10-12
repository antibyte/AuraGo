package ui

import "testing"

// TestCYDCreatorDevicesTransferModules checks the plan 5 OTA dialog and transfer modules.
func TestCYDCreatorDevicesTransferModules(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-devices.js",
		"P.devices = { open, errorText };", "inst.bus.on('device:changed'", "inst.confirm(", "listDevices:", "mintDevice:", "updateDevice:",
		"revokeDevice:", "'/devices'", "target_build_id", "ota.insecure")
	checkCYDCreatorModule(t, "cyd-creator-transfer.js",
		"P.transfer = { exportProject, importZip, pickFile, fileName };", "'/import'", "'/export'", "inst.flushAll()")
}
