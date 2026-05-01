package tools

import "testing"

func TestOllamaContainerNeedsRecreateWhenConfiguredDeviceIsMissing(t *testing.T) {
	inspect := []byte(`{
		"HostConfig": {
			"Devices": [
				{"PathOnHost": "/definitely/missing/aurago-gpu-device", "PathInContainer": "/dev/dri/card1", "CgroupPermissions": "rwm"}
			]
		}
	}`)

	if !ollamaContainerNeedsRecreateForHostDevices(inspect) {
		t.Fatal("expected container with missing host GPU device to require recreation")
	}
}

func TestOllamaContainerNeedsRecreateIgnoresContainersWithoutHostDevices(t *testing.T) {
	inspect := []byte(`{"HostConfig": {}}`)

	if ollamaContainerNeedsRecreateForHostDevices(inspect) {
		t.Fatal("did not expect CPU-only container to require recreation")
	}
}
