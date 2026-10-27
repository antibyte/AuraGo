package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vishen/go-chromecast/cast"
)

func chromecastTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeCast struct {
	loadURL        string
	loadType       string
	player         string
	idle           string
	updateErr      error
	volume         float32
	volumeSet      bool
	muted          bool
	mutedSet       bool
	seek           float32
	paused         bool
	resumed        bool
	stopped        bool
	updates        int
	session        int
	content        string
	pendingSession int
	pendingContent string
	stalePolls     int
	nextPlayer     string
	nextIdle       string
}

func (f *fakeCast) Load(filenameOrUrl string, startTime int, contentType string, transcode, detach, forceDetach bool) error {
	f.loadURL = filenameOrUrl
	f.loadType = contentType
	f.pendingContent = filenameOrUrl
	next := f.session + 1
	if next <= 0 {
		next = 1
	}
	f.pendingSession = next
	if f.stalePolls == 0 {
		f.adoptPending()
	}
	return nil
}

func (f *fakeCast) adoptPending() {
	if f.pendingSession != 0 {
		f.session = f.pendingSession
	}
	if f.pendingContent != "" {
		f.content = f.pendingContent
	}
	if f.nextPlayer != "" {
		f.player = f.nextPlayer
	}
	if f.nextIdle != "" {
		f.idle = f.nextIdle
	}
}
func (f *fakeCast) Pause() error   { f.paused = true; return nil }
func (f *fakeCast) Unpause() error { f.resumed = true; return nil }
func (f *fakeCast) SeekToTime(value float32) error {
	f.seek = value
	return nil
}
func (f *fakeCast) SetVolume(value float32) error { f.volume = value; f.volumeSet = true; return nil }
func (f *fakeCast) SetMuted(value bool) error     { f.muted = value; f.mutedSet = true; return nil }
func (f *fakeCast) StopMedia() error              { f.stopped = true; return nil }
func (f *fakeCast) Status() (*cast.Application, *cast.Media, *cast.Volume) {
	return &cast.Application{DisplayName: "AuraGo", AppId: "CC1AD845"}, &cast.Media{
		MediaSessionId: f.session,
		PlayerState:    f.player,
		IdleReason:     f.idle,
		CurrentTime:    3,
		Media:          cast.MediaItem{ContentId: f.content, Duration: 12, Metadata: cast.MediaMetadata{Title: "Song"}},
	}, &cast.Volume{Level: 0.4, Muted: false}
}
func (f *fakeCast) Update() error {
	f.updates++
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.stalePolls > 0 {
		f.stalePolls--
		return nil
	}
	f.adoptPending()
	return nil
}
func (f *fakeCast) Close(stopMedia bool) error { return nil }

func knownLivingRoom() ChromecastConfig {
	return ChromecastConfig{
		ServerHost: "192.168.6.10",
		ServerPort: 8090,
		KnownDevices: []ChromecastDevice{{
			Name: "Living Room",
			Addr: "192.168.6.130",
			Port: 8009,
		}},
	}
}

func TestAuthorizeChromecastDial(t *testing.T) {
	known := []ChromecastDevice{{Addr: "192.168.1.20", Port: 22}, {Addr: "10.0.0.8", Port: 8009}}
	tests := []struct {
		name    string
		addr    string
		port    int
		wantErr bool
	}{
		{name: "public", addr: "8.8.8.8", port: 8009, wantErr: true},
		{name: "loopback", addr: "127.0.0.1", port: 8009, wantErr: true},
		{name: "unknown private", addr: "192.168.1.9", port: 8009, wantErr: true},
		{name: "wrong port", addr: "192.168.1.20", port: 9999, wantErr: true},
		{name: "inventory default cast port", addr: "192.168.1.20", port: 8009, wantErr: false},
		{name: "inventory stored port", addr: "192.168.1.20", port: 22, wantErr: false},
		{name: "zero port means 8009", addr: "10.0.0.8", port: 0, wantErr: false},
		{name: "discovery hit", addr: "10.0.0.8", port: 8009, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthorizeChromecastDial(tt.addr, tt.port, known)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChromecastPlayRejectsUnknownAddressBeforeDial(t *testing.T) {
	probes := 0
	origFactory := chromecastHTTPClientFactory
	origConnect := connectChromecastController
	chromecastHTTPClientFactory = func(cfg ChromecastConfig) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			probes++
			header := make(http.Header)
			header.Set("Content-Type", "audio/mpeg")
			return &http.Response{StatusCode: 200, Header: header, Body: http.NoBody, Request: req}, nil
		})}
	}
	connectChromecastController = func(addr string, port int, ccCfg ChromecastConfig) (castController, error) {
		t.Fatal("dialed an unauthorized device")
		return nil, nil
	}
	defer func() {
		chromecastHTTPClientFactory = origFactory
		connectChromecastController = origConnect
	}()

	raw := ChromecastPlay("8.8.8.8", 8009, "https://media.example.test/song.mp3", "audio/mpeg", "", ChromecastConfig{}, chromecastTestLogger())
	if !strings.Contains(raw, "not a known private device") {
		t.Fatalf("result = %s", raw)
	}
	if probes != 0 {
		t.Fatalf("probes = %d, want 0", probes)
	}
}

func TestChromecastSpeakUsesFileMIMEAndHidesTicket(t *testing.T) {
	origSynth := synthesizeChromecastSpeech
	origConnect := connectChromecastController
	fake := &fakeCast{player: "PLAYING"}
	synthesizeChromecastSpeech = func(cfg TTSConfig, text string) (string, error) {
		return "voice.wav", nil
	}
	connectChromecastController = func(addr string, port int, ccCfg ChromecastConfig) (castController, error) {
		return fake, nil
	}
	defer func() {
		synthesizeChromecastSpeech = origSynth
		connectChromecastController = origConnect
	}()

	raw := ChromecastSpeak("192.168.6.130", 8009, "hello", TTSConfig{}, knownLivingRoom(), chromecastTestLogger())
	if fake.loadType != "audio/wav" {
		t.Fatalf("content type = %q", fake.loadType)
	}
	if strings.Contains(fake.loadURL, "cast_ticket=") == false {
		t.Fatal("device load lost the ticket")
	}
	if strings.Contains(raw, "cast_ticket") || strings.Contains(raw, "cast_expires") {
		t.Fatalf("model result leaked the ticket: %s", raw)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["status"] != "success" {
		t.Fatalf("status = %v", parsed["status"])
	}
}

func TestChromecastPlaybackConfirmation(t *testing.T) {
	origTimeout := chromecastPlaybackConfirmTimeout
	origPoll := chromecastPlaybackPollInterval
	chromecastPlaybackConfirmTimeout = 40 * time.Millisecond
	chromecastPlaybackPollInterval = 5 * time.Millisecond
	defer func() {
		chromecastPlaybackConfirmTimeout = origTimeout
		chromecastPlaybackPollInterval = origPoll
	}()

	const mediaURL = "http://media.example/a.mp3"
	for _, state := range []string{"PLAYING", "BUFFERING", "PAUSED"} {
		fake := &fakeCast{player: state, session: 2, content: mediaURL}
		if err := confirmChromecastPlayback(context.Background(), fake, mediaURL, 0, ""); err != nil {
			t.Fatalf("%s: %v", state, err)
		}
	}
	if err := confirmChromecastPlayback(context.Background(), &fakeCast{player: "IDLE", idle: "FINISHED", session: 2, content: mediaURL}, mediaURL, 0, ""); err != nil {
		t.Fatal(err)
	}
	for _, idle := range []string{"ERROR", "CANCELLED", "CANCELED"} {
		err := confirmChromecastPlayback(context.Background(), &fakeCast{player: "IDLE", idle: idle, session: 2, content: mediaURL}, mediaURL, 0, "")
		if err == nil || !strings.Contains(err.Error(), "playback rejected") {
			t.Fatalf("%s: %v", idle, err)
		}
	}
	err := confirmChromecastPlayback(context.Background(), &fakeCast{player: "IDLE", idle: "WAITING", session: 2, content: mediaURL}, mediaURL, 0, "")
	if err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatal(err)
	}
}

func TestChromecastPlaybackIgnoresStatusFromBeforeLoad(t *testing.T) {
	origTimeout := chromecastPlaybackConfirmTimeout
	origPoll := chromecastPlaybackPollInterval
	chromecastPlaybackConfirmTimeout = 40 * time.Millisecond
	chromecastPlaybackPollInterval = 5 * time.Millisecond
	defer func() {
		chromecastPlaybackConfirmTimeout = origTimeout
		chromecastPlaybackPollInterval = origPoll
	}()

	const nextURL = "http://media.example/new.mp3"
	stale := func(player, idle string, polls int) *fakeCast {
		return &fakeCast{
			player:     player,
			idle:       idle,
			session:    4,
			content:    "http://old.example/a.mp3",
			stalePolls: polls,
		}
	}
	for _, tc := range []struct {
		name   string
		player string
		idle   string
	}{
		{name: "playing", player: "PLAYING"},
		{name: "finished", player: "IDLE", idle: "FINISHED"},
		{name: "error", player: "IDLE", idle: "ERROR"},
	} {
		fake := stale(tc.player, tc.idle, 100)
		err := chromecastLoadAndConfirm(fake, context.Background(), nextURL, "audio/mpeg")
		if err == nil || !strings.Contains(err.Error(), "not confirmed") || strings.Contains(err.Error(), "rejected") {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if fake.stopped {
			t.Fatalf("%s stopped media on timeout", tc.name)
		}
	}

	same := stale("PLAYING", "", 100)
	same.content = "http://media.example/same.mp3"
	err := chromecastLoadAndConfirm(same, context.Background(), same.content, "audio/mpeg")
	if err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatalf("same url replay: %v", err)
	}

	delayed := stale("PLAYING", "", 2)
	if err := chromecastLoadAndConfirm(delayed, context.Background(), nextURL, "audio/mpeg"); err != nil {
		t.Fatal(err)
	}
	if delayed.updates < 3 || delayed.session == 4 || delayed.content != nextURL {
		t.Fatalf("updates=%d session=%d content=%s", delayed.updates, delayed.session, delayed.content)
	}

	replaced := stale("IDLE", "ERROR", 1)
	replaced.nextPlayer = "PLAYING"
	if err := chromecastLoadAndConfirm(replaced, context.Background(), nextURL, "audio/mpeg"); err != nil {
		t.Fatal(err)
	}
}

func TestChromecastVolumeAndMuteRequireExplicitValues(t *testing.T) {
	origConnect := connectChromecastController
	fake := &fakeCast{player: "PLAYING"}
	connectChromecastController = func(addr string, port int, ccCfg ChromecastConfig) (castController, error) {
		return fake, nil
	}
	defer func() { connectChromecastController = origConnect }()
	cfg := knownLivingRoom()
	logger := chromecastTestLogger()

	if raw := ChromecastVolume("192.168.6.130", 8009, 0, false, cfg, logger); !strings.Contains(raw, "'volume' is required") {
		t.Fatalf("missing volume: %s", raw)
	}
	if fake.volumeSet {
		t.Fatal("missing volume dialed a level")
	}
	if raw := ChromecastVolume("192.168.6.130", 8009, 0, true, cfg, logger); !strings.Contains(raw, `"status":"success"`) || fake.volume != 0 {
		t.Fatalf("volume 0 result=%s level=%v", raw, fake.volume)
	}
	if raw := ChromecastMute("192.168.6.130", 8009, false, false, cfg, logger); !strings.Contains(raw, "'muted' is required") {
		t.Fatalf("missing mute: %s", raw)
	}
	if raw := ChromecastMute("192.168.6.130", 8009, true, true, cfg, logger); !strings.Contains(raw, `"status":"success"`) || !fake.mutedSet || !fake.muted {
		t.Fatalf("mute result=%s", raw)
	}
}

func TestChromecastGroupRefusesMutationAndAllowsStatus(t *testing.T) {
	origConnect := connectChromecastController
	fake := &fakeCast{player: "PLAYING"}
	calls := 0
	connectChromecastController = func(addr string, port int, ccCfg ChromecastConfig) (castController, error) {
		calls++
		return fake, nil
	}
	defer func() { connectChromecastController = origConnect }()
	cfg := knownLivingRoom()
	cfg.KnownDevices[0].Group = true
	logger := chromecastTestLogger()
	if raw := ChromecastPlay("192.168.6.130", 8009, "https://media.example.test/a.mp3", "", "", cfg, logger); !strings.Contains(raw, "not controllable") || calls != 0 {
		t.Fatalf("play group calls=%d result=%s", calls, raw)
	}
	raw := ChromecastStatus("192.168.6.130", 8009, cfg, logger)
	if !strings.Contains(raw, `"title":"Song"`) || !strings.Contains(raw, `"player_state":"PLAYING"`) {
		t.Fatalf("status = %s", raw)
	}
}

func TestChromecastReadOnlyBlocksMutation(t *testing.T) {
	cfg := knownLivingRoom()
	cfg.ReadOnly = true
	raw := ChromecastStop("192.168.6.130", 8009, cfg, chromecastTestLogger())
	if !strings.Contains(raw, "read-only") {
		t.Fatalf("result = %s", raw)
	}
}

func TestValidateChromecastMediaURLFallsBackFromHeadAndRejectsHTML(t *testing.T) {
	origFactory := chromecastHTTPClientFactory
	defer func() { chromecastHTTPClientFactory = origFactory }()

	t.Run("head fallback", func(t *testing.T) {
		chromecastHTTPClientFactory = func(cfg ChromecastConfig) *http.Client {
			return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodHead {
					return &http.Response{StatusCode: http.StatusMethodNotAllowed, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
				}
				if req.Header.Get("Range") != "bytes=0-0" {
					t.Errorf("range = %q", req.Header.Get("Range"))
				}
				header := make(http.Header)
				header.Set("Content-Type", "audio/mpeg")
				return &http.Response{StatusCode: http.StatusPartialContent, Header: header, Body: http.NoBody, Request: req}, nil
			})}
		}
		if err := validateChromecastMediaURL("https://media.example.test/song.mp3", ChromecastConfig{}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("youtube html", func(t *testing.T) {
		chromecastHTTPClientFactory = func(cfg ChromecastConfig) *http.Client {
			return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				header := make(http.Header)
				header.Set("Content-Type", "text/html; charset=utf-8")
				return &http.Response{StatusCode: 200, Header: header, Body: http.NoBody, Request: req}, nil
			})}
		}
		err := validateChromecastMediaURL("https://www.youtube.com/watch?v=abc", ChromecastConfig{})
		if err == nil || !strings.Contains(err.Error(), "YouTube pages are unsupported") || !strings.Contains(err.Error(), "direct media file") {
			t.Fatal(err)
		}
	})
}

func TestDiscoverChromecastDevicesLabelsAndPrefersPrivateIPv4(t *testing.T) {
	orig := chromecastMDNSQuery
	chromecastMDNSQuery = func(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
		return []*mdnsEntry{
			{
				Name: "Group._googlecast._tcp.local.",
				IPs:  []string{"fe80::1", "2001:db8::5", "8.8.8.8", "192.168.1.9"},
				Port: 8009,
				TXTs: []string{"id=cast-id-1", "fn=Wohnzimmer", "md=Google Cast Group", "ca=36"},
			},
			{
				Name: "Audio._googlecast._tcp.local.",
				IPs:  []string{"192.168.1.10"},
				Port: 8009,
				TXTs: []string{"id=cast-id-2", "fn=Küche", "md=Chromecast Audio", "ca=4"},
			},
		}, nil
	}
	defer func() { chromecastMDNSQuery = orig }()

	devices, err := DiscoverChromecastDevices(chromecastTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 {
		t.Fatalf("devices = %d", len(devices))
	}
	if devices[0].Addr != "192.168.1.9" || devices[0].UUID != "cast-id-1" || !devices[0].Group || devices[0].Model != "Google Cast Group" {
		t.Fatalf("group device = %+v", devices[0])
	}
	if !devices[1].AudioOnly || devices[1].Group || devices[1].Capabilities != "4" {
		t.Fatalf("audio device = %+v", devices[1])
	}
}

func TestCachedDiscoverChromecastDevices(t *testing.T) {
	ResetChromecastDiscoveryCache()
	defer ResetChromecastDiscoveryCache()
	orig := chromecastMDNSQuery
	calls := 0
	chromecastMDNSQuery = func(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
		calls++
		return []*mdnsEntry{{
			Name: "One._googlecast._tcp.local.",
			IPs:  []string{"192.168.1.9"},
			Port: 8009,
			TXTs: []string{"id=one", "fn=One"},
		}}, nil
	}
	defer func() { chromecastMDNSQuery = orig }()
	logger := chromecastTestLogger()
	first, err := CachedDiscoverChromecastDevices(logger)
	if err != nil || len(first) != 1 {
		t.Fatalf("first = %+v %v", first, err)
	}
	first[0].Addr = "changed"
	second, err := CachedDiscoverChromecastDevices(logger)
	if err != nil || second[0].Addr != "192.168.1.9" || calls != 1 {
		t.Fatalf("second = %+v calls=%d err=%v", second, calls, err)
	}
	ResetChromecastDiscoveryCache()
	if _, err := CachedDiscoverChromecastDevices(logger); err != nil || calls != 2 {
		t.Fatalf("calls after reset = %d err=%v", calls, err)
	}
}

func TestPruneExpiredCastMedia(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.mp3")
	newPath := filepath.Join(dir, "new.mp3")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-CastMediaRetention - time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := PruneExpiredCastMedia(dir, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old file stat = %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal(err)
	}
}

func TestChromecastPlayEchoesLiveHintWithoutTicket(t *testing.T) {
	origFactory := chromecastHTTPClientFactory
	origConnect := connectChromecastController
	fake := &fakeCast{player: "PLAYING"}
	chromecastHTTPClientFactory = func(cfg ChromecastConfig) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			header := make(http.Header)
			header.Set("Content-Type", "audio/mpeg")
			return &http.Response{StatusCode: 200, Header: header, Body: http.NoBody, Request: req}, nil
		})}
	}
	connectChromecastController = func(addr string, port int, ccCfg ChromecastConfig) (castController, error) {
		return fake, nil
	}
	defer func() {
		chromecastHTTPClientFactory = origFactory
		connectChromecastController = origConnect
	}()
	raw := ChromecastPlay("192.168.6.130", 8009, "https://media.example.test/live.mp3?cast_ticket=secret", "audio/mpeg", "live", knownLivingRoom(), chromecastTestLogger())
	if strings.Contains(raw, "secret") || strings.Contains(raw, "cast_ticket") || !strings.Contains(raw, `"stream_type":"live"`) {
		t.Fatalf("result = %s", raw)
	}
	if !strings.Contains(fake.loadURL, "cast_ticket=secret") {
		t.Fatalf("device URL = %s", fake.loadURL)
	}
}
