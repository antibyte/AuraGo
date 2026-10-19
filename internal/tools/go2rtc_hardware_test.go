package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCHardwareDevicesAndProbe(t *testing.T) {
	host := map[string]interface{}{}
	payload := map[string]interface{}{"Env": []string{}}
	cfg := config.Go2RTCConfig{Hardware: config.Go2RTCHardwareConfig{Backend: "vaapi", RenderNode: "/dev/dri/renderD129", RenderGroup: "993"}}
	addGo2RTCGPUDevices(host, payload, cfg)
	encoded, _ := json.Marshal(host)
	if string(encoded) == "{}" || host["Privileged"] != nil {
		t.Fatal("invalid GPU permissions")
	}
	devices := host["Devices"].([]map[string]string)
	if len(devices) != 1 || devices[0]["PathOnHost"] != "/dev/dri/renderD129" || devices[0]["CgroupPermissions"] != "rw" {
		t.Fatal(devices)
	}
	if Go2RTCBaseImageFor(cfg) != config.Go2RTCHardwareImage || Go2RTCImageFor(cfg) != go2RTCManagedImage(config.Go2RTCHardwareImage) {
		t.Fatal("hardware image not selected")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"sources":[{"name":"OK","url":"ffmpeg:...#video=h264#hardware=vaapi"}]}`))
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg.Enabled = true
	cfg.URL = upstream.URL
	cfg.APIHostPort = port
	cfg.APIPassword = "fixture"
	full := &config.Config{Go2RTC: cfg}
	full.Directories.DataDir = t.TempDir()
	m := NewGo2RTCManager(full, nil, nil, nil)
	defer m.Close()
	result, err := m.CheckHardware(context.Background())
	if err != nil || !result.Verified {
		t.Fatalf("probe: %+v %v", result, err)
	}
	if !m.HardwareStatus().Verified {
		t.Fatal("verification not retained")
	}
	cfg.Hardware.RenderNode = "/dev/dri/renderD130"
	full.Go2RTC = cfg
	m.Configure(full)
	if m.HardwareStatus().Verified {
		t.Fatal("old verification survived hardware change")
	}
}

func TestGo2RTCCUDAExposesOnlySelectedGPU(t *testing.T) {
	host := map[string]interface{}{}
	payload := map[string]interface{}{"Env": []string{}}
	cfg := config.Go2RTCConfig{Hardware: config.Go2RTCHardwareConfig{Backend: "cuda", DeviceID: "1"}}
	addGo2RTCGPUDevices(host, payload, cfg)
	requests := host["DeviceRequests"].([]map[string]interface{})
	if len(requests) != 1 || requests[0]["DeviceIDs"].([]string)[0] != "1" || host["Privileged"] != nil || host["Devices"] != nil {
		t.Fatal(host)
	}
	env := payload["Env"].([]string)
	if len(env) != 2 || env[0] != "NVIDIA_VISIBLE_DEVICES=1" {
		t.Fatal(env)
	}
	host = map[string]interface{}{}
	payload = map[string]interface{}{"Env": []string{}}
	addGo2RTCGPUDevices(host, payload, config.Go2RTCConfig{})
	if payload["Env"].([]string)[0] != "NVIDIA_VISIBLE_DEVICES=void" {
		t.Fatal("base hardware image can expose implicit NVIDIA devices")
	}
}
