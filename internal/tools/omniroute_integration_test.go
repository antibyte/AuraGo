package tools

import (
	"encoding/json"
	"slices"
	"testing"

	"aurago/internal/config"
)

func TestResolveOmniRouteSidecarConfigRequiresInitialPassword(t *testing.T) {
	cfg := &config.Config{}
	cfg.OmniRoute.Enabled = true
	cfg.OmniRoute.Mode = "managed"
	cfg.OmniRoute.JWTSecret = "jwt-secret"
	cfg.OmniRoute.APIKeySecret = "api-key-secret"
	cfg.OmniRoute.WSBridgeSecret = "ws-bridge-secret"

	if _, err := ResolveOmniRouteSidecarConfig(cfg, false); err == nil {
		t.Fatal("ResolveOmniRouteSidecarConfig() error = nil, want missing initial password error")
	}
}

func TestResolveOmniRouteSidecarConfigDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.OmniRoute.Enabled = true
	cfg.OmniRoute.Mode = "managed"
	cfg.OmniRoute.URL = "http://omniroute:20128"
	cfg.OmniRoute.InitialPassword = "initial-admin-password"
	cfg.OmniRoute.JWTSecret = "jwt-secret"
	cfg.OmniRoute.APIKeySecret = "api-key-secret"
	cfg.OmniRoute.WSBridgeSecret = "ws-bridge-secret"

	sidecar, err := ResolveOmniRouteSidecarConfig(cfg, true)
	if err != nil {
		t.Fatalf("ResolveOmniRouteSidecarConfig() error = %v", err)
	}
	if sidecar.ContainerName != "aurago_omniroute" {
		t.Fatalf("ContainerName = %q, want aurago_omniroute", sidecar.ContainerName)
	}
	if sidecar.Image != "diegosouzapw/omniroute:3.8.39" {
		t.Fatalf("Image = %q, want pinned OmniRoute image", sidecar.Image)
	}
	if sidecar.ProviderBaseURL != "http://omniroute:20128/v1" {
		t.Fatalf("ProviderBaseURL = %q, want managed /v1 URL", sidecar.ProviderBaseURL)
	}
	if sidecar.DataVolume != "aurago_omniroute_data" {
		t.Fatalf("DataVolume = %q, want aurago_omniroute_data", sidecar.DataVolume)
	}
	if sidecar.HealthPath != "/api/monitoring/health" {
		t.Fatalf("HealthPath = %q, want /api/monitoring/health", sidecar.HealthPath)
	}
	if sidecar.MemoryMB != 512 {
		t.Fatalf("MemoryMB = %d, want 512", sidecar.MemoryMB)
	}
}

func TestBuildOmniRouteCreatePayload(t *testing.T) {
	sidecar := OmniRouteSidecarConfig{
		InternalBaseURL: "http://omniroute:20128",
		BrowserBaseURL:  "http://127.0.0.1:20128",
		ContainerName:   "aurago_omniroute",
		Image:           "diegosouzapw/omniroute:3.8.39",
		Host:            "127.0.0.1",
		Port:            20128,
		HostPort:        20128,
		NetworkName:     "aurago_omniroute",
		DataVolume:      "aurago_omniroute_data",
		InitialPassword: "initial-admin-password",
		JWTSecret:       "jwt-secret",
		APIKeySecret:    "api-key-secret",
		WSBridgeSecret:  "ws-bridge-secret",
		HealthPath:      "/api/monitoring/health",
		MemoryMB:        512,
		RunningInDocker: false,
	}

	payload, err := buildOmniRouteCreatePayload(sidecar, "aurago_omniroute")
	if err != nil {
		t.Fatalf("buildOmniRouteCreatePayload() error = %v", err)
	}

	var body struct {
		Image      string   `json:"Image"`
		Env        []string `json:"Env"`
		HostConfig struct {
			Binds        []string `json:"Binds"`
			NetworkMode  string   `json:"NetworkMode"`
			Memory       int64    `json:"Memory"`
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Image != "diegosouzapw/omniroute:3.8.39" {
		t.Fatalf("Image = %q, want pinned OmniRoute image", body.Image)
	}
	for _, want := range []string{
		"DATA_DIR=/app/data",
		"PORT=20128",
		"DASHBOARD_PORT=20128",
		"HOSTNAME=0.0.0.0",
		"NODE_ENV=production",
		"REQUIRE_API_KEY=true",
		"ALLOW_API_KEY_REVEAL=false",
		"JWT_SECRET=jwt-secret",
		"API_KEY_SECRET=api-key-secret",
		"INITIAL_PASSWORD=initial-admin-password",
		"OMNIROUTE_WS_BRIDGE_SECRET=ws-bridge-secret",
		"NEXT_PUBLIC_BASE_URL=http://127.0.0.1:20128",
	} {
		if !slices.Contains(body.Env, want) {
			t.Fatalf("Env missing %q in %#v", want, body.Env)
		}
	}
	if !slices.Contains(body.HostConfig.Binds, "aurago_omniroute_data:/app/data") {
		t.Fatalf("Binds = %#v, want OmniRoute data volume", body.HostConfig.Binds)
	}
	if body.HostConfig.NetworkMode != "aurago_omniroute" {
		t.Fatalf("NetworkMode = %q, want aurago_omniroute", body.HostConfig.NetworkMode)
	}
	if body.HostConfig.Memory != 512*1024*1024 {
		t.Fatalf("Memory = %d, want 512 MiB", body.HostConfig.Memory)
	}
	bindings := body.HostConfig.PortBindings["20128/tcp"]
	if len(bindings) != 1 || bindings[0].HostIP != "0.0.0.0" || bindings[0].HostPort != "20128" {
		t.Fatalf("PortBindings = %#v, want 0.0.0.0:20128", body.HostConfig.PortBindings)
	}
}

func TestOmniRouteContainerNeedsRecreateWhenDataVolumeChanges(t *testing.T) {
	inspect := []byte(`{
		"Config":{"Env":["NEXT_PUBLIC_BASE_URL=http://127.0.0.1:20128"]},
		"HostConfig":{"Binds":["old_volume:/app/data"],"PortBindings":{"20128/tcp":[{"HostIp":"0.0.0.0","HostPort":"20128"}]}},
		"NetworkSettings":{"Networks":{"aurago_omniroute":{}}}
	}`)
	sidecar := OmniRouteSidecarConfig{
		BrowserBaseURL: "http://127.0.0.1:20128",
		Port:           20128,
		HostPort:       20128,
		Host:           "127.0.0.1",
		DataVolume:     "aurago_omniroute_data",
	}
	if !omniRouteContainerNeedsRecreate(inspect, sidecar, "aurago_omniroute") {
		t.Fatal("omniRouteContainerNeedsRecreate() = false, want true when data volume changes")
	}
}
