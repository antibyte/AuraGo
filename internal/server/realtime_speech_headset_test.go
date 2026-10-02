package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"aurago/internal/bluetooth"
	"aurago/internal/realtimespeech"
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

type fakeRealtimeHeadsetLink struct {
	frames  chan []byte
	events  chan bluetooth.HeadsetEvent
	closed  chan struct{}
	once    sync.Once
	mu      sync.Mutex
	written map[int][]byte
	flushed []int
}

func newFakeRealtimeHeadsetLink() *fakeRealtimeHeadsetLink {
	return &fakeRealtimeHeadsetLink{frames: make(chan []byte, 4), events: make(chan bluetooth.HeadsetEvent, 4),
		closed: make(chan struct{}), written: map[int][]byte{}}
}

func (l *fakeRealtimeHeadsetLink) Frames() <-chan []byte                 { return l.frames }
func (l *fakeRealtimeHeadsetLink) Events() <-chan bluetooth.HeadsetEvent { return l.events }

func (l *fakeRealtimeHeadsetLink) Write(stream int, pcm []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.written[stream] = append(l.written[stream], pcm...)
	return nil
}

func (l *fakeRealtimeHeadsetLink) Flush(stream int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flushed = append(l.flushed, stream)
}

func (l *fakeRealtimeHeadsetLink) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

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

func acquireHeadsetTestSession(t *testing.T, registry *realtimespeech.Registry) realtimespeech.Session {
	t.Helper()
	session, _, err := registry.Acquire("browser", realtimespeech.Session{ProfileID: "primary", Provider: "openai", Surface: "webchat"}, false)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func dialRealtimeHeadset(server *httptest.Server, query string, header http.Header) (*websocket.Conn, *http.Response, error) {
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/realtime-speech/headset?" + query
	return websocket.DefaultDialer.Dial(url, header)
}

func readRealtimeHeadsetJSON(t *testing.T, conn *websocket.Conn) map[string]interface{} {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	kind, data, err := conn.ReadMessage()
	if err != nil || kind != websocket.TextMessage {
		t.Fatalf("read = %d %q %v", kind, data, err)
	}
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatal(err)
	}
	return message
}

func waitForRealtimeHeadset(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRealtimeSpeechHeadsetBridgesAudioAndState(t *testing.T) {
	registry := realtimespeech.NewRegistry(nil)
	session := acquireHeadsetTestSession(t, registry)
	link := newFakeRealtimeHeadsetLink()
	headsets := &fakeRealtimeHeadsets{link: link}
	server := httptest.NewServer(newRealtimeHeadsetHandler(registry, headsets, 20*time.Millisecond))
	defer server.Close()

	if _, response, err := dialRealtimeHeadset(server, "session=missing&client=browser&device=AA:BB:CC:DD:EE:FF", nil); err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("unknown session: err=%v response=%v", err, response)
	}

	conn, _, err := dialRealtimeHeadset(server, "session="+session.ID+"&client=browser&device=AA:BB:CC:DD:EE:FF", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if message := readRealtimeHeadsetJSON(t, conn); message["type"] != "ready" || message["input_rate"] != float64(16000) || message["output_rate"] != float64(24000) {
		t.Fatalf("ready = %v", message)
	}
	link.events <- bluetooth.HeadsetEvent{Type: bluetooth.HeadsetLost}
	if message := readRealtimeHeadsetJSON(t, conn); message["type"] != "device_lost" {
		t.Fatalf("lost = %v", message)
	}
	link.events <- bluetooth.HeadsetEvent{Type: bluetooth.HeadsetReady}
	if message := readRealtimeHeadsetJSON(t, conn); message["type"] != "device_ready" {
		t.Fatalf("ready event = %v", message)
	}
	link.events <- bluetooth.HeadsetEvent{Type: bluetooth.HeadsetError, Code: bluetooth.ErrorHeadsetProfileUnavailable}
	if message := readRealtimeHeadsetJSON(t, conn); message["type"] != "error" || message["code"] != "headset_profile_unavailable" {
		t.Fatalf("error event = %v", message)
	}

	link.frames <- []byte{1, 2, 3, 4}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if kind, data, err := conn.ReadMessage(); err != nil || kind != websocket.BinaryMessage || string(data) != "\x01\x02\x03\x04" {
		t.Fatalf("frame = %d %v %v", kind, data, err)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{1, 9, 8}); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"flush","stream":0}`)); err != nil {
		t.Fatal(err)
	}
	waitForRealtimeHeadset(t, "browser audio and flush", func() bool {
		link.mu.Lock()
		defer link.mu.Unlock()
		return string(link.written[1]) == "\x09\x08" && len(link.flushed) == 1 && link.flushed[0] == 0
	})

	registry.Release(session.ID, "browser")
	select {
	case <-link.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge kept running after the session ended")
	}
}

func TestRealtimeSpeechHeadsetReportsOpenErrors(t *testing.T) {
	registry := realtimespeech.NewRegistry(nil)
	session := acquireHeadsetTestSession(t, registry)
	headsets := &fakeRealtimeHeadsets{openErr: &bluetooth.CodedError{Code: bluetooth.ErrorHeadsetBusy, Message: "busy"}}
	server := httptest.NewServer(newRealtimeHeadsetHandler(registry, headsets, time.Second))
	defer server.Close()

	conn, _, err := dialRealtimeHeadset(server, "session="+session.ID+"&client=browser&device=AA:BB:CC:DD:EE:FF", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if message := readRealtimeHeadsetJSON(t, conn); message["type"] != "error" || message["code"] != "headset_busy" {
		t.Fatalf("error = %v", message)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("bridge stayed open after a failed open")
	}
}

func TestRealtimeSpeechHeadsetClosesTheLinkWhenTheBrowserLeaves(t *testing.T) {
	registry := realtimespeech.NewRegistry(nil)
	session := acquireHeadsetTestSession(t, registry)
	link := newFakeRealtimeHeadsetLink()
	server := httptest.NewServer(newRealtimeHeadsetHandler(registry, &fakeRealtimeHeadsets{link: link}, time.Second))
	defer server.Close()

	conn, _, err := dialRealtimeHeadset(server, "session="+session.ID+"&client=browser&device=AA:BB:CC:DD:EE:FF", nil)
	if err != nil {
		t.Fatal(err)
	}
	readRealtimeHeadsetJSON(t, conn)
	_ = conn.Close()
	select {
	case <-link.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("link still open after the browser left")
	}
}

func TestRealtimeSpeechHeadsetRejectsForeignOrigins(t *testing.T) {
	registry := realtimespeech.NewRegistry(nil)
	session := acquireHeadsetTestSession(t, registry)
	headsets := &fakeRealtimeHeadsets{link: newFakeRealtimeHeadsetLink()}
	server := httptest.NewServer(newRealtimeHeadsetHandler(registry, headsets, time.Second))
	defer server.Close()

	header := http.Header{"Origin": []string{"http://evil.example"}}
	if _, _, err := dialRealtimeHeadset(server, "session="+session.ID+"&client=browser&device=AA:BB:CC:DD:EE:FF", header); err == nil {
		t.Fatal("foreign origin was accepted")
	}
	if headsets.openCount() != 0 {
		t.Fatal("headset opened for a foreign origin")
	}
}
