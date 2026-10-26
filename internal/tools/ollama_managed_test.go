package tools

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestOllamaDoesNotInferDaemonDevicesFromLocalFilesystem(t *testing.T) {
	desired, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{Backend: "intel", Devices: []string{"/not-mounted-in-aurago/renderD128"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if ollamaManagedSpecDrift(inspectFromCreatePayload(t, desired), desired) {
		t.Fatal("unchanged daemon device spec was treated as drift")
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
		"Config":     map[string]any{"Image": body.Image, "Labels": body.Labels},
		"HostConfig": body.HostConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ollamaManagedSpecDrift(changed, desired) {
		t.Fatal("different host port was not drift")
	}
}

func TestOllamaManagedSpecDriftNormalizesNativeNetwork(t *testing.T) {
	desired, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "default", "bridge", "host", "another_app"} {
		t.Run(mode, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(inspectFromCreatePayload(t, desired), &doc); err != nil {
				t.Fatal(err)
			}
			doc["HostConfig"].(map[string]any)["NetworkMode"] = mode
			inspect, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			wantDrift := mode == "host" || mode == "another_app"
			if drift := ollamaManagedSpecDrift(inspect, desired); drift != wantDrift {
				t.Fatalf("native network %q: drift=%v, want %v", mode, drift, wantDrift)
			}
		})
	}
	placed, err := ollamaPlacePayload(desired, "app_default")
	if err != nil {
		t.Fatal(err)
	}
	if !ollamaManagedSpecDrift(inspectFromCreatePayload(t, desired), placed) {
		t.Fatal("native bridge was treated as the named application bridge")
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

func TestOllamaManagedSpecDriftIgnoresRegistryPrefixAndBindMode(t *testing.T) {
	desired, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	var body ollamaCreateBody
	if err := json.Unmarshal(desired, &body); err != nil {
		t.Fatal(err)
	}
	body.Image = "docker.io/" + body.Image
	body.HostConfig.Binds[0] += ":rw"
	inspect, err := json.Marshal(map[string]any{
		"Config":     map[string]any{"Image": body.Image, "Labels": body.Labels},
		"HostConfig": body.HostConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ollamaManagedSpecDrift(inspect, desired) {
		t.Fatal("registry prefix or bind mode was treated as drift")
	}

	hostDesired, err := ollamaManagedCreatePayload(11434, `C:\models`, "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(hostDesired, &body); err != nil {
		t.Fatal(err)
	}
	body.Image = "index.docker.io/" + body.Image
	body.HostConfig.Binds[0] += ":rw,z"
	inspect, err = json.Marshal(map[string]any{
		"Config":     map[string]any{"Image": body.Image, "Labels": body.Labels},
		"HostConfig": body.HostConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ollamaManagedSpecDrift(inspect, hostDesired) {
		t.Fatal("windows bind mode was treated as drift")
	}
}

func TestOllamaReconcileLeavesContainerWhenReplacementImageIsMissing(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	desired, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	inspect := inspectFromCreatePayload(t, desired)
	var doc map[string]any
	if err := json.Unmarshal(inspect, &doc); err != nil {
		t.Fatal(err)
	}
	configDoc, _ := doc["Config"].(map[string]any)
	configDoc["Image"] = "ollama/ollama:latest"
	inspect, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	var deleted bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"+ollamaManagedContainerName+"/json"):
			_, _ = w.Write(inspect)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/images/json"):
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/images/create"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"registry unavailable"}`))
		case r.Method == http.MethodDelete || strings.Contains(r.URL.Path, "/stop"):
			deleted = true
			t.Errorf("container mutated before a replacement image existed: %s %s", r.Method, r.URL.Path)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	reconcileNamedOllamaContainer(ollamaReconcile{
		Label:         "Ollama Managed",
		ContainerName: ollamaManagedContainerName,
		Port:          11434,
		Image:         "ollama/ollama:" + ollamaImageTag,
		Payload:       desired,
		Docker:        DockerConfig{Host: host},
		Logger:        ollamaTestLogger{},
	})
	if deleted {
		t.Fatal("existing container was removed")
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
			"Image":  body.Image,
			"Labels": body.Labels,
			"Env":    append([]string{"PATH=/usr/bin"}, body.Env...),
		},
		"HostConfig": body.HostConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
