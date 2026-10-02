package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/bluetooth"
)

type fakeRealtimeHeadsets struct {
	devices []bluetooth.HeadsetDevice
	reason  string
	link    *fakeRealtimeHeadsetLink
	openErr error
	mu      sync.Mutex
	opened  []string
}

func (f *fakeRealtimeHeadsets) List(context.Context) ([]bluetooth.HeadsetDevice, string) {
	return f.devices, f.reason
}

func (f *fakeRealtimeHeadsets) Open(_ context.Context, address string) (realtimeHeadsetLink, error) {
	f.mu.Lock()
	f.opened = append(f.opened, address)
	f.mu.Unlock()
	if f.openErr != nil {
		return nil, f.openErr
	}
	return f.link, nil
}

func (f *fakeRealtimeHeadsets) openCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.opened)
}

// fakeRealtimeHeadsetLink is filled in by the bridge tests.
type fakeRealtimeHeadsetLink struct{}

func (l *fakeRealtimeHeadsetLink) Frames() <-chan []byte                 { return nil }
func (l *fakeRealtimeHeadsetLink) Events() <-chan bluetooth.HeadsetEvent { return nil }
func (l *fakeRealtimeHeadsetLink) Write(int, []byte) error               { return nil }
func (l *fakeRealtimeHeadsetLink) Flush(int)                             {}
func (l *fakeRealtimeHeadsetLink) Close() error                          { return nil }

func TestRealtimeSpeechAudioDevicesListsServerHeadsets(t *testing.T) {
	headsets := &fakeRealtimeHeadsets{devices: []bluetooth.HeadsetDevice{{Address: "AA:BB:CC:DD:EE:FF", Name: "Earbuds", Connected: true}}}
	handler := handleRealtimeSpeechAudioDevices(headsets)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/realtime-speech/audio-devices", nil))
	var body struct {
		Devices []map[string]interface{} `json:"devices"`
		Reason  string                   `json:"reason"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || len(body.Devices) != 1 {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
	device := body.Devices[0]
	if device["id"] != "AA:BB:CC:DD:EE:FF" || device["name"] != "Earbuds" || device["connected"] != true || device["busy"] != false {
		t.Fatalf("device = %v", device)
	}

	headsets.devices, headsets.reason = nil, "Server headsets need PipeWire; PulseAudio is not supported."
	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/realtime-speech/audio-devices", nil))
	if !strings.Contains(rec.Body.String(), `"devices":[]`) || !strings.Contains(rec.Body.String(), "PulseAudio is not supported") {
		t.Fatalf("empty response = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/realtime-speech/audio-devices", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", rec.Code)
	}
}
