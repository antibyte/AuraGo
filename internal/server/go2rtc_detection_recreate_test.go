package server

import (
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
	stopped := base
	stopped.Enabled = false
	if go2RTCRequiresRecreate(stopped, detecting) {
		t.Fatal("enabling go2rtc starts a fresh container; the fingerprint already carries the limits")
	}
}
