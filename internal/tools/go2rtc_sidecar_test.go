package tools

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCManagedSidecarBuildIsPinned(t *testing.T) {
	var built bool
	server := newDockerAPITestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/"+dockerAPIVersion+"/build" {
			t.Error("unexpected Docker operation", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var args map[string]string
		_ = json.Unmarshal([]byte(r.URL.Query().Get("buildargs")), &args)
		if args["BASE_IMAGE"] != config.Go2RTCHardwareImage {
			t.Error("hardware base pin lost", args)
		}
		files := map[string]string{}
		reader := tar.NewReader(r.Body)
		for {
			h, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Error(err)
				return
			}
			data, _ := io.ReadAll(reader)
			files[h.Name] = string(data)
		}
		if !strings.Contains(files["Dockerfile"], "78aa79bcedec8f155e4060a379613979b0b3ee48ff62ee5164bafc0ac6532386") || !strings.Contains(files["lifecycle.patch"], "tr.OnClose") || !strings.Contains(files["lifecycle_test.go.txt"], "TestAuraGoSocketCloseTerminatesPeer") {
			t.Error("build omitted source pin, patch or actual upstream regression")
		}
		if !strings.Contains(files["Dockerfile"], "FROM golang:1.27.3-alpine@sha256:") {
			t.Error("managed sidecar lost the fixed, digest-pinned Go compiler")
		}
		if !strings.Contains(files["tuya.patch"], "DialContext") || !strings.Contains(files["tuya_api.go.txt"], "post-v1") || !strings.Contains(files["tuya_runtime.go.txt"], "BoundHTTPClient") || !strings.Contains(files["tuya_api_test.go.txt"], "TestAuraGoTuyaProbeJPEGAndCancellation") {
			t.Error("Tuya runtime or pinned regressions missing from image")
		}
		built = true
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"stream\":\"built\"}\n"))
	}))
	defer server.Close()
	m := NewGo2RTCManager(&config.Config{}, nil, nil, nil)
	defer m.Close()
	docker := DockerConfig{Host: "tcp://" + strings.TrimPrefix(server.URL, "http://")}
	if err := m.ensureManagedSidecarImage(context.Background(), docker, go2RTCManagedImage(config.Go2RTCHardwareImage)); err != nil {
		t.Fatal(err)
	}
	if !built {
		t.Fatal("managed image was pulled or used without applying the patch")
	}
	if Go2RTCImageFor(config.Go2RTCConfig{Image: "example/custom:reviewed"}) != "example/custom:reviewed" {
		t.Fatal("custom image changed")
	}
}

func TestGo2RTCTranscodingResourcesAndTmpfs(t *testing.T) {
	cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, Streams: []config.Go2RTCStreamConfig{{ID: "one", Enabled: true, CodecProfile: "h264_opus"}}}}
	cfg.Directories.DataDir = t.TempDir()
	m := NewGo2RTCManager(cfg, nil, nil, nil)
	defer m.Close()
	resources := Go2RTCResourcesFor(cfg.Go2RTC)
	if resources.NanoCPUs < 2_000_000_000 || resources.MemoryBytes < 1<<30 {
		t.Fatal(resources)
	}
	payload, err := m.go2RTCContainerPayload(m.Config(), cfg.Directories.DataDir+"/go2rtc.yaml", "fixture", "fixture", 1)
	if err != nil {
		t.Fatal(err)
	}
	host := payload["HostConfig"].(map[string]interface{})
	if host["NanoCpus"] != int64(1_000_000_000) {
		t.Fatal("host CPU cap lost", host["NanoCpus"])
	}
	command := strings.Join(payload["Cmd"].([]string), " ")
	if !strings.Contains(command, "cp /config/go2rtc.yaml /tmp/aurago-go2rtc.yaml") || !strings.Contains(command, "exec /usr/local/bin/aurago-go2rtc") {
		t.Fatal(command)
	}
	if host["ReadonlyRootfs"] != true || host["Tmpfs"].(map[string]string)["/tmp"] == "" {
		t.Fatal("writable runtime copy escaped tmpfs")
	}
}
