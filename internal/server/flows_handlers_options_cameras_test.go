package server

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestFlowOptionsListCameras(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	s.Cfg.Go2RTC.Enabled = true
	s.Cfg.Go2RTC.Detection.Enabled = true
	s.Cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{
		{ID: "garden", Name: "Garten", Enabled: true},
		{ID: "door", Enabled: true},
		{ID: "old", Name: "Alt", Enabled: false},
	}
	s.Cfg.Go2RTC.Streams[0].Detection.Enabled = true
	opts, err := s.flowOptions(context.Background(), "cameras", "en")
	if err != nil || len(opts) != 3 {
		t.Fatalf("camera options = %+v, %v", opts, err)
	}
	if opts[0].Value != "" || opts[0].Label == "" {
		t.Fatalf("first option must be any camera: %+v", opts[0])
	}
	if opts[1] != (flowOption{Value: "garden", Label: "Garten", Hint: "garden"}) {
		t.Fatalf("garden = %+v", opts[1])
	}
	if opts[2].Value != "door" || opts[2].Label != "door" || !strings.HasPrefix(opts[2].Hint, "door · ") {
		t.Fatalf("door (no name, detection off) = %+v", opts[2])
	}
}

// With person detection switched off as a whole (or go2rtc off), no camera's trigger fires,
// so every camera says so, its own detection switch notwithstanding.
func TestFlowOptionsCamerasShowDetectionOffGlobally(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	s.Cfg.Go2RTC.Enabled = true
	s.Cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{{ID: "garden", Name: "Garten", Enabled: true}}
	s.Cfg.Go2RTC.Streams[0].Detection.Enabled = true
	for _, tc := range []struct {
		name              string
		go2rtc, detection bool
	}{{"detection off", true, false}, {"go2rtc off", false, true}} {
		s.Cfg.Go2RTC.Enabled, s.Cfg.Go2RTC.Detection.Enabled = tc.go2rtc, tc.detection
		opts, err := s.flowOptions(context.Background(), "cameras", "en")
		if err != nil || len(opts) != 2 {
			t.Fatalf("%s: camera options = %+v, %v", tc.name, opts, err)
		}
		if want := "garden · " + flowsTranslator("en")("easydrag.option.camera_detection_off"); opts[1].Hint != want {
			t.Errorf("%s: garden hint = %q, want %q", tc.name, opts[1].Hint, want)
		}
	}
}
