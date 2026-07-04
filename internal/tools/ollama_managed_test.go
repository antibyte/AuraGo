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

func TestOllamaContainerListHasHostPortMatchesPublishedPort(t *testing.T) {
	list := []byte(`[
		{
			"Id": "abc",
			"Image": "ollama/ollama:latest",
			"Ports": [
				{"PrivatePort": 11434, "PublicPort": 11435, "IP": "127.0.0.1", "Type": "tcp"}
			]
		}
	]`)

	if !ollamaContainerListHasHostPort(list, 11435) {
		t.Fatal("expected container list to match published host port 11435")
	}
	if ollamaContainerListHasHostPort(list, 11434) {
		t.Fatal("did not expect private container port 11434 to match host port")
	}
}

func TestOllamaContainerListHasHostPortIgnoresUnpublishedContainers(t *testing.T) {
	list := []byte(`[
		{
			"Id": "abc",
			"Image": "ollama/ollama:latest",
			"Ports": [
				{"PrivatePort": 11434, "Type": "tcp"}
			]
		}
	]`)

	if ollamaContainerListHasHostPort(list, 11434) {
		t.Fatal("did not expect unpublished container port to match host port")
	}
}
