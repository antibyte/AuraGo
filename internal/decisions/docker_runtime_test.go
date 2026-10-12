package decisions

import (
	"context"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDockerCreatePayloadKeepsReadOnlyMountsAndGPUAccessInHostConfig(t *testing.T) {
	mounts := []map[string]any{
		{"Type": "bind", "Source": filepath.Join(t.TempDir(), "models"), "Target": "/models", "ReadOnly": true},
		{"Type": "bind", "Source": filepath.Join(t.TempDir(), "token"), "Target": "/run/decisions", "ReadOnly": true},
	}
	baseHostConfig := dockerHostConfig("CUDA", "aurago-decisions-internal", map[string]any{
		"18765/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "45123"}},
	})
	body := dockerContainerCreateBody("sha256:pinned-image", []string{"--model", "/models/model.gguf"}, mounts, baseHostConfig)
	if body["Tty"] != false {
		t.Fatalf("discovery container Tty = %v, want disabled Docker stream framing", body["Tty"])
	}
	hostConfig, ok := body["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("HostConfig has unexpected type: %T", body["HostConfig"])
	}
	if _, misplaced := body["Mounts"]; misplaced {
		t.Fatal("Docker mounts were placed outside HostConfig")
	}
	gotMounts, ok := hostConfig["Mounts"].([]map[string]any)
	if !ok || len(gotMounts) != 2 {
		t.Fatalf("HostConfig.Mounts = %#v", hostConfig["Mounts"])
	}
	for _, mount := range gotMounts {
		if mount["ReadOnly"] != true {
			t.Fatalf("managed input mount is not read-only: %#v", mount)
		}
	}
	if hostConfig["NetworkMode"] != "aurago-decisions-internal" {
		t.Fatalf("NetworkMode = %v", hostConfig["NetworkMode"])
	}
	if _, ok := hostConfig["DeviceRequests"]; !ok {
		t.Fatal("CUDA container did not request accelerator access")
	}
	if hostConfig["Memory"] != int64(8<<30) || hostConfig["MemorySwap"] != int64(8<<30) {
		t.Fatalf("CUDA HostConfig memory cap = Memory %v MemorySwap %v, want %d", hostConfig["Memory"], hostConfig["MemorySwap"], int64(8<<30))
	}
	if _, ok := hostConfig["PortBindings"]; !ok {
		t.Fatal("loopback port binding is missing")
	}
	if _, mutated := baseHostConfig["Mounts"]; mutated {
		t.Fatal("create payload builder mutated its caller's HostConfig")
	}
}

func TestDockerDiscoveryLogsSeparateStdoutFromBackendStderr(t *testing.T) {
	stdoutOne := []byte(`{"protocol":"aurago.`)
	stdoutTwo := []byte(`decisions.v1","devices":[]}`)
	stderr := []byte("Vulkan: selected device GPU 0\n")
	frame := func(stream byte, payload []byte) []byte {
		result := make([]byte, 8+len(payload))
		result[0] = stream
		binary.BigEndian.PutUint32(result[4:8], uint32(len(payload)))
		copy(result[8:], payload)
		return result
	}
	data := append(frame(2, stderr), frame(1, stdoutOne)...)
	data = append(data, frame(1, stdoutTwo)...)
	got, err := dockerStdoutFromMultiplexedLogs(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(append(stdoutOne, stdoutTwo...)) {
		t.Fatalf("decoded discovery stdout = %q, want only protocol JSON %q", got, append(stdoutOne, stdoutTwo...))
	}
	if _, err := dockerStdoutFromMultiplexedLogs(data[:len(data)-1]); err == nil {
		t.Fatal("truncated Docker frame was accepted")
	}
}

func TestDockerCreatePayloadUsesVulkanDeviceMapping(t *testing.T) {
	body := dockerContainerCreateBody("sha256:image", nil, nil, dockerHostConfig("vulkan", "none", nil))
	hostConfig := body["HostConfig"].(map[string]any)
	devices, ok := hostConfig["Devices"].([]map[string]string)
	if !ok || len(devices) != 1 || devices[0]["PathOnHost"] != "/dev/dri" || devices[0]["PathInContainer"] != "/dev/dri" {
		t.Fatalf("Vulkan device access = %#v", hostConfig["Devices"])
	}
	if hostConfig["Memory"] != int64(8<<30) || hostConfig["MemorySwap"] != int64(8<<30) {
		t.Fatalf("Vulkan HostConfig memory cap = Memory %v MemorySwap %v, want %d", hostConfig["Memory"], hostConfig["MemorySwap"], int64(8<<30))
	}
}

func TestDockerContainerImageMustRemainPinnedToInspectedID(t *testing.T) {
	if !dockerContainerImageMatches("sha256:immutable", "sha256:immutable") {
		t.Fatal("unchanged inspected image was rejected")
	}
	for _, actual := range []string{"", "aurago-decisions-runtime:cuda", "sha256:replacement"} {
		if dockerContainerImageMatches("sha256:immutable", actual) {
			t.Fatalf("mutable or different image %q matched pinned ID", actual)
		}
	}
	if dockerContainerImageMatches("", "") {
		t.Fatal("missing inspected image ID was accepted")
	}
}

func TestDockerRuntimeCreatePayloadUsesImmutableImageReference(t *testing.T) {
	imageID := "sha256:" + strings.Repeat("a", 64)
	if !validDockerImageID(imageID) {
		t.Fatalf("valid inspected ID rejected: %s", imageID)
	}
	body := dockerContainerCreateBody(imageID, []string{"--discover"}, nil, dockerHostConfig("cuda", "none", nil))
	if body["Image"] != imageID {
		t.Fatalf("container creation would resolve mutable reference %v", body["Image"])
	}
	for _, invalid := range []string{"aurago-decisions-runtime:cuda", "sha256:short", "sha256:" + strings.Repeat("z", 64)} {
		if validDockerImageID(invalid) {
			t.Errorf("non-immutable Docker reference accepted: %q", invalid)
		}
	}
}

func TestDockerBindPathRequiresAnExplicitlyResolvableLocalMapping(t *testing.T) {
	docker := &decisionsDocker{host: "tcp://docker.example:2376"}
	_, _, _, _, err := dockerResolvedPathMount(t.Context(), docker, RuntimeAccess{}, t.TempDir(), "/models", nil)
	if ErrorCode(err) != CodeDeploymentDenied {
		t.Fatalf("remote daemon bind error = %v", err)
	}
}

func TestDockerContainerModeFailsClosedWithoutVerifiedSelfResolver(t *testing.T) {
	docker := &decisionsDocker{host: "unix:///var/run/docker.sock"}
	_, _, _, _, err := dockerResolvedPathMount(t.Context(), docker, RuntimeAccess{IsDocker: true}, t.TempDir(), "/models", nil)
	if ErrorCode(err) != CodeDeploymentDenied {
		t.Fatalf("missing verified container identity error = %v", err)
	}
}

func TestDockerContainerPathMappingUsesLongestRootedMount(t *testing.T) {
	base := struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	}{Type: "bind", Source: t.TempDir(), Destination: "/app"}
	child := struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	}{Type: "bind", Source: filepath.Join(t.TempDir(), "managed"), Destination: "/app/data"}
	mounts := []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	}{base, child}
	resolved, subpath, err := resolveContainerPath("/app/data/decisions/models", mounts)
	if err != nil || subpath != "" || resolved != filepath.ToSlash(filepath.Join(child.Source, "decisions", "models")) {
		t.Fatalf("resolved path = %q, subpath=%q, error=%v", resolved, subpath, err)
	}
	if _, _, err := resolveContainerPath("/app/data-evil/secrets", mounts); ErrorCode(err) != CodeDeploymentDenied {
		t.Fatalf("sibling path outside mapped mount was accepted: %v", err)
	}
}

func TestDockerContainerCleanupUsesBoundedContext(t *testing.T) {
	started := time.Now()
	deadlineObserved := false
	withDockerCleanupContext(25*time.Millisecond, func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		deadlineObserved = ok && time.Until(deadline) <= 25*time.Millisecond && time.Until(deadline) > 0
		<-ctx.Done()
	})
	if !deadlineObserved {
		t.Fatal("Docker cleanup callback did not receive a bounded deadline")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded Docker cleanup took %s", elapsed)
	}
}
