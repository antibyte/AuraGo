package server

import (
	"fmt"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCRecreateWhenDetectionLimitsChange(t *testing.T) {
	base := config.Go2RTCConfig{
		Enabled: true, Image: config.Go2RTCDefaultImage, ContainerName: "aurago_go2rtc",
		URL: "http://127.0.0.1:1984", APIHostPort: 1984,
		Detection: config.Go2RTCDetectionConfig{Enabled: true},
		Streams:   []config.Go2RTCStreamConfig{{ID: "front-door", Enabled: true}},
	}
	detecting := base
	detecting.Streams = []config.Go2RTCStreamConfig{{ID: "front-door", Enabled: true, Detection: config.Go2RTCStreamDetectionConfig{Enabled: true}}}
	if !go2RTCRequiresRecreate(base, detecting) {
		t.Fatal("enabling detection on a camera must recreate the sidecar with raised limits")
	}
	if !go2RTCRequiresRecreate(detecting, base) {
		t.Fatal("disabling the last detecting camera must restore the base limits")
	}
	tuned := detecting
	tuned.Streams = []config.Go2RTCStreamConfig{{ID: "front-door", Enabled: true, Detection: config.Go2RTCStreamDetectionConfig{Enabled: true, MinScore: 0.7}}}
	if go2RTCRequiresRecreate(detecting, tuned) {
		t.Fatal("tuning a detecting camera does not change the limits")
	}
	disabled := base
	disabled.Enabled = false
	if go2RTCRequiresRecreate(disabled, detecting) {
		t.Fatal("enabling go2rtc starts a fresh container; the fingerprint already carries the limits")
	}
}

// detectionLimitsConfig is an enabled sidecar with the given number of cameras,
// the first `detecting` of them with person detection on.
func detectionLimitsConfig(cameras, detecting int, streamsEnabled bool) config.Go2RTCConfig {
	cfg := config.Go2RTCConfig{
		Enabled: true, Image: config.Go2RTCDefaultImage, ContainerName: "aurago_go2rtc",
		URL: "http://127.0.0.1:1984", APIHostPort: 1984,
		Detection: config.Go2RTCDetectionConfig{Enabled: true},
	}
	for i := 0; i < cameras; i++ {
		cfg.Streams = append(cfg.Streams, config.Go2RTCStreamConfig{
			ID: fmt.Sprintf("cam%d", i), Enabled: streamsEnabled,
			Detection: config.Go2RTCStreamDetectionConfig{Enabled: i < detecting},
		})
	}
	return cfg
}

// The sidecar is only recreated when the CPU or memory limits it would get
// change, never for detection edits that leave them alone.
func TestGo2RTCRecreateMatrixForDetectionLimits(t *testing.T) {
	globalOff := detectionLimitsConfig(1, 1, true)
	globalOff.Detection.Enabled = false
	globalOnNoCameras := detectionLimitsConfig(1, 0, true)
	globalOffNoCameras := globalOnNoCameras
	globalOffNoCameras.Detection.Enabled = false
	globalOnWithCamera := detectionLimitsConfig(1, 1, true)

	for _, tc := range []struct {
		name     string
		old, new config.Go2RTCConfig
		want     bool
	}{
		{"global detection switched off with a detecting camera", detectionLimitsConfig(1, 1, true), globalOff, true},
		{"global detection switched on with a detecting camera", globalOff, globalOnWithCamera, true},
		{"global detection switched on without detecting cameras", globalOffNoCameras, globalOnNoCameras, false},
		{"enabling a stream whose detection is on", detectionLimitsConfig(1, 1, false), detectionLimitsConfig(1, 1, true), true},
		{"enabling a stream whose detection is off", detectionLimitsConfig(1, 0, false), detectionLimitsConfig(1, 0, true), false},
		{"second detecting camera", detectionLimitsConfig(2, 1, true), detectionLimitsConfig(2, 2, true), true},
		{"adding a detecting camera", detectionLimitsConfig(1, 1, true), detectionLimitsConfig(2, 2, true), true},
		{"adding a camera without detection", detectionLimitsConfig(1, 1, true), detectionLimitsConfig(2, 1, true), false},
		{"sixth to seventh detecting camera stays at the 2 CPU cap", detectionLimitsConfig(7, 6, true), detectionLimitsConfig(7, 7, true), false},
		{"seventh to sixth detecting camera stays at the 2 CPU cap", detectionLimitsConfig(7, 7, true), detectionLimitsConfig(7, 6, true), false},
		{"fifth to sixth detecting camera reaches the cap", detectionLimitsConfig(6, 5, true), detectionLimitsConfig(6, 6, true), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldCfg, newCfg := config.Config{Go2RTC: tc.old}, config.Config{Go2RTC: tc.new}
			if _, recreate := go2RTCRuntimeTransition(oldCfg, newCfg); recreate != tc.want {
				t.Fatalf("recreate = %v, want %v", recreate, tc.want)
			}
		})
	}
}
