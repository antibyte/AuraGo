package tools

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

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

func TestOllamaManagedCreatePayloadUsesNamedVolume(t *testing.T) {
	raw, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	var body ollamaCreateBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Image != "ollama/ollama:"+ollamaImageTag {
		t.Fatalf("image = %s, want ollama/ollama:%s", body.Image, ollamaImageTag)
	}
	if ollamaImageTag == "latest" || strings.Contains(ollamaImageTag, "sha256") {
		t.Fatalf("recorded image tag %q is floating or a digest", ollamaImageTag)
	}
	if len(body.HostConfig.Binds) != 1 || body.HostConfig.Binds[0] != ollamaManagedVolumeName+":/root/.ollama" {
		t.Fatalf("binds = %#v", body.HostConfig.Binds)
	}
	bind := body.HostConfig.PortBindings["11434/tcp"]
	if len(bind) != 1 || bind[0].HostIp != "127.0.0.1" || bind[0].HostPort != "11434" {
		t.Fatalf("port binding = %#v", bind)
	}
}

func TestOllamaManagedCreatePayloadUsesHostPath(t *testing.T) {
	raw, err := ollamaManagedCreatePayload(11434, "/data/ollama", "8g", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	var body ollamaCreateBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.HostConfig.Binds) != 1 || body.HostConfig.Binds[0] != "/data/ollama:/root/.ollama" {
		t.Fatalf("binds = %#v", body.HostConfig.Binds)
	}
	if body.HostConfig.Memory != 8*1024*1024*1024 {
		t.Fatalf("memory = %d", body.HostConfig.Memory)
	}
}

func TestOllamaManagedSpecDrift(t *testing.T) {
	desired, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if ollamaManagedSpecDrift(inspectFromCreatePayload(t, desired), desired) {
		t.Fatal("matching inspect was treated as drift")
	}
	var body ollamaCreateBody
	if err := json.Unmarshal(desired, &body); err != nil {
		t.Fatal(err)
	}
	body.HostConfig.PortBindings["11434/tcp"][0].HostPort = "11499"
	changed, err := json.Marshal(map[string]any{
		"Config":     map[string]any{"Image": body.Image},
		"HostConfig": body.HostConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ollamaManagedSpecDrift(changed, desired) {
		t.Fatal("different host port was not drift")
	}
}

func TestOllamaImageForGPUUsesRecordedTag(t *testing.T) {
	if got := ollamaImageForGPU(GPUInfo{}); got != "ollama/ollama:0.40.3" {
		t.Fatalf("cpu image = %s", got)
	}
	if got := ollamaImageForGPU(GPUInfo{Backend: "nvidia"}); got != "ollama/ollama:0.40.3" {
		t.Fatalf("nvidia image = %s", got)
	}
	// Docker Hub published 0.40.3-rocm next to 0.40.3 on 11 Oct 2026.
	if got := ollamaImageForGPU(GPUInfo{Backend: "amd"}); got != "ollama/ollama:0.40.3-rocm" {
		t.Fatalf("amd image = %s", got)
	}
}

func TestOllamaManagedEnsureDoesNotAdoptByAncestor(t *testing.T) {
	for _, name := range []string{"ollama_managed.go", "ollama_embeddings.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "ancestor") {
			t.Fatalf("%s still adopts a foreign container by ancestor image", name)
		}
	}
}

func inspectFromCreatePayload(t *testing.T, desired []byte) []byte {
	t.Helper()
	var body ollamaCreateBody
	if err := json.Unmarshal(desired, &body); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"Config": map[string]any{
			"Image": body.Image,
			"Env":   append([]string{"PATH=/usr/bin"}, body.Env...),
		},
		"HostConfig": body.HostConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
