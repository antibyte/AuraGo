package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func detectingGo2RTCConfig(cameras int) config.Go2RTCConfig {
	cfg := config.Go2RTCConfig{
		Enabled: true, Image: config.Go2RTCDefaultImage, URL: "http://127.0.0.1:1984", APIHostPort: 1984,
		ContainerName: "aurago_go2rtc", APIPassword: "internal-password",
		Detection: config.Go2RTCDetectionConfig{Enabled: true},
	}
	for i := 0; i < cameras; i++ {
		cfg.Streams = append(cfg.Streams, config.Go2RTCStreamConfig{
			ID: fmt.Sprintf("cam%d", i), Enabled: true, Detection: config.Go2RTCStreamDetectionConfig{Enabled: true},
		})
	}
	return cfg
}

func TestGo2RTCResourcesForDetectingCameras(t *testing.T) {
	for _, tc := range []struct {
		cameras     int
		nano, bytes int64
	}{
		{0, 500_000_000, 256 << 20},
		{1, 750_000_000, 512 << 20},
		{2, 1_000_000_000, 512 << 20},
		{6, 2_000_000_000, 512 << 20},
		{9, 2_000_000_000, 512 << 20},
	} {
		got := Go2RTCResourcesFor(detectingGo2RTCConfig(tc.cameras))
		if got.NanoCPUs != tc.nano || got.MemoryBytes != tc.bytes || got.DetectingCameras != tc.cameras {
			t.Fatalf("%d cameras: resources = %+v", tc.cameras, got)
		}
	}
	cfg := detectingGo2RTCConfig(3)
	cfg.Streams[0].Enabled = false
	cfg.Streams[1].Detection.Enabled = false
	if got := Go2RTCDetectingCameras(cfg); got != 1 {
		t.Fatalf("disabled stream or stream detection counted: %d", got)
	}
	cfg.Detection.Enabled = false
	if got := Go2RTCDetectingCameras(cfg); got != 0 {
		t.Fatalf("global detection off still counts %d cameras", got)
	}
	cfg = detectingGo2RTCConfig(2)
	cfg.Enabled = false
	if got := Go2RTCDetectingCameras(cfg); got != 0 {
		t.Fatalf("go2rtc off still counts %d cameras", got)
	}
}

func TestGo2RTCContainerSpecRaisesLimitsWhileDetecting(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC = detectingGo2RTCConfig(2)
	manager := NewGo2RTCManager(cfg, nil, nil, nil)
	payload, err := manager.go2RTCContainerPayload(cfg.Go2RTC, filepath.Join(cfg.Directories.DataDir, "go2rtc", "go2rtc.yaml"), "internal-password", "fingerprint", 0)
	if err != nil {
		t.Fatalf("go2RTCContainerPayload: %v", err)
	}
	encoded, _ := json.Marshal(payload)
	for _, required := range []string{`"NanoCpus":1000000000`, `"Memory":536870912`, `"PidsLimit":100`, `"ReadonlyRootfs":true`} {
		if !strings.Contains(string(encoded), required) {
			t.Fatalf("container spec missing %q: %s", required, encoded)
		}
	}
}

func TestGo2RTCFingerprintKeepsLegacyValueWithoutDetection(t *testing.T) {
	cfg := detectingGo2RTCConfig(2)
	cfg.Detection.Enabled = false
	rendered := renderGo2RTCConfig(cfg)
	legacy := sha256.Sum256(append([]byte(cfg.Image+"\x00"+cfg.URL+"\x00"+"1984"+"\x00"+"owner"+"\x00"), rendered...))
	if got := go2RTCFingerprint(cfg, rendered, "owner"); got != hex.EncodeToString(legacy[:]) {
		t.Fatalf("fingerprint without active detection changed: %s", got)
	}
}

func TestGo2RTCFingerprintTracksDetectionLimits(t *testing.T) {
	fingerprint := func(cameras int) string {
		cfg := detectingGo2RTCConfig(cameras)
		return go2RTCFingerprint(cfg, renderGo2RTCConfig(cfg), "owner")
	}
	if fingerprint(0) == fingerprint(1) {
		t.Fatal("starting detection must change the fingerprint")
	}
	if fingerprint(1) == fingerprint(2) {
		t.Fatal("a second detecting camera raises the CPU limit and must change the fingerprint")
	}
	if fingerprint(6) != fingerprint(7) {
		t.Fatal("cameras beyond the 2-CPU cap must not recreate the sidecar")
	}
}
