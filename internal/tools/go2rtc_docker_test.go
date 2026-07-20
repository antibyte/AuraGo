package tools

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestRenderGo2RTCConfigContainsNoSecretOrStreamSource(t *testing.T) {
	cfg := config.Go2RTCConfig{
		WebRTC: config.Go2RTCWebRTCConfig{Enabled: true, BindAddress: "192.168.1.20", Port: 8555},
		Streams: []config.Go2RTCStreamConfig{{
			ID: "front-door", Source: "rtsp://user:password@camera.local/live",
		}},
		APIPassword: "internal-password",
	}
	rendered := string(renderGo2RTCConfig(cfg))
	for _, required := range []string{
		`base_path: "/api/go2rtc/proxy"`,
		`local_auth: true`,
		`password: "${AURAGO_GO2RTC_API_PASSWORD}"`,
		`- "/api/go2rtc/proxy/api/streams"`,
		`- "/api/go2rtc/proxy/api/frame.jpeg"`,
		`- "/api/go2rtc/proxy/api/hls/playlist.m3u8"`,
		`- "/api/go2rtc/proxy/api/hls/segment.m4s"`,
		`listen: "127.0.0.1:8554"`,
		`streams: {}`,
	} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("rendered config missing %q:\n%s", required, rendered)
		}
	}
	for _, forbidden := range []string{"camera.local", "user:password", "internal-password"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("rendered config leaked %q:\n%s", forbidden, rendered)
		}
	}
	for _, blocked := range []string{"/api/config", "/api/log", "/api/exit", "/api/restart"} {
		if strings.Contains(rendered, blocked) {
			t.Fatalf("rendered config allows sensitive upstream API %q:\n%s", blocked, rendered)
		}
	}
}

func TestGo2RTCContainerSpecIsHardenedAndLoopbackOnly(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled:       true,
		Image:         config.Go2RTCDefaultImage,
		ContainerName: "aurago_go2rtc",
		URL:           "http://127.0.0.1:1984",
		APIHostPort:   1984,
		APIPassword:   "internal-password",
	}
	manager := NewGo2RTCManager(cfg, nil, nil, nil)
	configPath := filepath.Join(dataDir, "go2rtc", "go2rtc.yaml")
	payload, err := manager.go2RTCContainerPayload(cfg.Go2RTC, configPath, cfg.Go2RTC.APIPassword, "fingerprint")
	if err != nil {
		t.Fatalf("go2RTCContainerPayload: %v", err)
	}
	encoded, _ := json.Marshal(payload)
	spec := string(encoded)
	for _, required := range []string{
		config.Go2RTCDefaultImage,
		`"User":"65532:65532"`,
		`"ReadonlyRootfs":true`,
		`"no-new-privileges:true"`,
		`"CapDrop":["ALL"]`,
		`"PidsLimit":100`,
		`"Memory":268435456`,
		`"NanoCpus":500000000`,
		`"HostIp":"127.0.0.1"`,
		`"1984/tcp"`,
	} {
		if !strings.Contains(spec, required) {
			t.Fatalf("container spec missing %q: %s", required, spec)
		}
	}
	if strings.Contains(spec, "8554/tcp") || strings.Contains(spec, `"Privileged":true`) {
		t.Fatalf("container spec exposes forbidden capability: %s", spec)
	}
}

func TestGo2RTCContainerSpecPublishesExplicitWebRTCOnTCPAndUDP(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled:       true,
		Image:         config.Go2RTCDefaultImage,
		ContainerName: "aurago_go2rtc",
		URL:           "http://127.0.0.1:1984",
		APIHostPort:   1984,
		APIPassword:   "internal-password",
		WebRTC: config.Go2RTCWebRTCConfig{
			Enabled: true, BindAddress: "192.168.1.20", Port: 8555,
		},
	}
	manager := NewGo2RTCManager(cfg, nil, nil, nil)
	payload, err := manager.go2RTCContainerPayload(cfg.Go2RTC, filepath.Join(t.TempDir(), "go2rtc.yaml"), cfg.Go2RTC.APIPassword, "fingerprint")
	if err != nil {
		t.Fatalf("go2RTCContainerPayload: %v", err)
	}
	encoded, _ := json.Marshal(payload)
	spec := string(encoded)
	for _, required := range []string{`"8555/tcp"`, `"8555/udp"`, `"HostIp":"192.168.1.20"`, `"HostPort":"8555"`} {
		if !strings.Contains(spec, required) {
			t.Fatalf("WebRTC container spec missing %q: %s", required, spec)
		}
	}
}

func TestValidateGo2RTCWebRTCRequiresConcretePrivateAddress(t *testing.T) {
	for _, invalid := range []string{"", "0.0.0.0", "127.0.0.1", "8.8.8.8", "not-an-ip"} {
		err := validateGo2RTCWebRTC(config.Go2RTCWebRTCConfig{Enabled: true, BindAddress: invalid, Port: 8555})
		if err == nil {
			t.Fatalf("validateGo2RTCWebRTC(%q) unexpectedly succeeded", invalid)
		}
	}
	if err := validateGo2RTCWebRTC(config.Go2RTCWebRTCConfig{Enabled: true, BindAddress: "192.168.1.20", Port: 8555}); err != nil {
		t.Fatalf("validateGo2RTCWebRTC(private IP): %v", err)
	}
}

func TestGo2RTCDockerPlacementUsesDataVolumeSubpathAndAvoidsControlNetwork(t *testing.T) {
	inspection := []byte(`{
		"NetworkSettings":{"Networks":{"aurago_docker-control":{},"aurago_default":{}}},
		"Mounts":[
			{"Type":"volume","Name":"aurago_data","Source":"/var/lib/docker/volumes/aurago_data/_data","Destination":"/app/data"}
		]
	}`)
	network, mount, err := go2RTCPlacementFromInspection(inspection, "/app/data/go2rtc")
	if err != nil {
		t.Fatalf("go2RTCPlacementFromInspection: %v", err)
	}
	if network != "aurago_default" {
		t.Fatalf("network = %q, want non-control application network", network)
	}
	encoded, _ := json.Marshal(mount)
	spec := string(encoded)
	for _, required := range []string{
		`"Type":"volume"`,
		`"Source":"aurago_data"`,
		`"Target":"/config"`,
		`"ReadOnly":true`,
		`"Subpath":"go2rtc"`,
	} {
		if !strings.Contains(spec, required) {
			t.Fatalf("volume subpath mount missing %q: %s", required, spec)
		}
	}
}

func TestGo2RTCFingerprintIsOwnerSpecific(t *testing.T) {
	cfg := config.Go2RTCConfig{Image: config.Go2RTCDefaultImage, URL: "http://go2rtc:1984", APIHostPort: 1984}
	rendered := renderGo2RTCConfig(cfg)
	first := go2RTCFingerprint(cfg, rendered, "container:first")
	second := go2RTCFingerprint(cfg, rendered, "container:second")
	if first == second {
		t.Fatal("go2rtc fingerprints must differ between AuraGo owners")
	}
}

func TestDockerInspectRedactsGo2RTCInternalPassword(t *testing.T) {
	redacted := redactDockerInspectEnv([]interface{}{
		"NORMAL=value",
		"AURAGO_GO2RTC_API_PASSWORD=super-secret",
	})
	encoded, _ := json.Marshal(redacted)
	output := string(encoded)
	if !strings.Contains(output, "NORMAL=value") || !strings.Contains(output, "AURAGO_GO2RTC_API_PASSWORD=••••••••") {
		t.Fatalf("unexpected redacted Docker environment: %s", output)
	}
	if strings.Contains(output, "super-secret") {
		t.Fatalf("Docker inspect environment leaked go2rtc credential: %s", output)
	}
}
