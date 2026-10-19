package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/persondetect/persondetecttest"
	"aurago/internal/tools"
)

func TestPersonDetectionSettingsFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join("data", "dir")
	cfg.Server.UILanguage = "de"
	cfg.Go2RTC.Enabled = true
	cfg.Go2RTC.Detection = config.Go2RTCDetectionConfig{
		Enabled: true, Model: "accurate", Backend: "gpu", RetentionDays: 7, MaxStorageMB: 500,
		Schedule: []config.Go2RTCDetectionWindow{{Days: []string{"mon", "sun"}, From: "22:30", To: "06:15"}},
	}
	front := config.Go2RTCStreamDetectionConfig{
		Enabled: true, IntervalSeconds: 3, MinScore: 0.6, CooldownSeconds: 120,
		Zones:   []config.Go2RTCDetectionZone{{Name: "Driveway", Points: [][2]float64{{0.1, 0.5}, {0.9, 0.5}, {0.9, 1}}}},
		Actions: config.Go2RTCDetectionActions{Message: true, Channel: "telegram", Agent: true, AgentInstruction: "Check", Desktop: false},
	}
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{
		{ID: "front-door", Name: "Front door", Enabled: true, Detection: front},
		{ID: "garage", Name: "Garage", Enabled: true},
		{ID: "porch", Name: "Porch", Enabled: false, Detection: front},
		{ID: "yard", Enabled: true, Detection: config.Go2RTCStreamDetectionConfig{Enabled: true}},
	}
	got := personDetectionSettingsFromConfig(cfg)
	want := persondetect.Settings{
		Enabled: true, DataDir: filepath.Join("data", "dir"),
		Model: persondetect.ModelAccurate, Backend: persondetect.BackendGPU,
		RetentionDays: 7, MaxStorageMB: 500,
		Schedule: []persondetect.Window{{Days: []time.Weekday{time.Monday, time.Sunday}, From: 22*60 + 30, To: 6*60 + 15}},
		Cameras: []persondetect.Camera{
			{
				ID: "front-door", Name: "Front door", Interval: 3 * time.Second, MinScore: 0.6, Cooldown: 2 * time.Minute,
				Zones:   []persondetect.Zone{{Name: "Driveway", Points: []persondetect.Point{{X: 0.1, Y: 0.5}, {X: 0.9, Y: 0.5}, {X: 0.9, Y: 1}}}},
				Actions: persondetect.Actions{Message: true, Channel: "telegram", Agent: true, AgentInstruction: "Check"},
			},
			{
				ID: "yard", Name: "yard", Interval: 2 * time.Second, MinScore: 0.5, Cooldown: 5 * time.Minute,
				Zones: []persondetect.Zone{}, Actions: persondetect.Actions{Channel: "all", Desktop: true},
			},
		},
		Location: time.Local, Language: "de",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("settings =\n%+v\nwant\n%+v", got, want)
	}
	cfg.Go2RTC.Enabled = false
	if personDetectionSettingsFromConfig(cfg).Enabled {
		t.Fatal("detection must be off while go2rtc is off")
	}
	if s := personDetectionSettingsFromConfig(nil); s.Enabled || s.Location != time.Local {
		t.Fatalf("nil config = %+v", s)
	}
}

// The schedule mapping must read days and clocks exactly like config validation
// (config.ParseGo2RTCDetectionDay/Clock) and produce windows that persondetect
// evaluates on the named weekday and minutes. persondetect does not import config,
// so this pins the conventions on both sides.
func TestPersonDetectionScheduleMappingUsesConfigParsers(t *testing.T) {
	sunday := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for i, spelling := range []string{"sun", "Mon", " tue ", "WED", "thu", "fri", "sat"} {
		day, ok := config.ParseGo2RTCDetectionDay(spelling)
		from, fromOK := config.ParseGo2RTCDetectionClock("11:30")
		to, toOK := config.ParseGo2RTCDetectionClock("12:30")
		if !ok || !fromOK || !toOK || int(day) != i {
			t.Fatalf("config parsers: %q = %v %v, clocks %d/%v %d/%v", spelling, day, ok, from, fromOK, to, toOK)
		}
		windows := personDetectionWindows([]config.Go2RTCDetectionWindow{{Days: []string{spelling}, From: "11:30", To: "12:30"}})
		want := []persondetect.Window{{Days: []time.Weekday{day}, From: from, To: to}}
		if !reflect.DeepEqual(windows, want) {
			t.Fatalf("%q mapped to %+v, want %+v", spelling, windows, want)
		}
		at := sunday.AddDate(0, 0, i)
		if !persondetect.InWindow(at, windows[0]) || persondetect.InWindow(at.AddDate(0, 0, 1), windows[0]) {
			t.Fatalf("%q window does not cover %s 12:00 only", spelling, at.Weekday())
		}
	}
	// Invalid entries never reach the service (config.Load rejects them anyway).
	if got := personDetectionWindows([]config.Go2RTCDetectionWindow{{Days: []string{"mon"}, From: "24:00", To: "06:00"}, {Days: []string{"xyz"}, From: "01:00", To: "02:00"}}); len(got) != 0 {
		t.Fatalf("invalid windows mapped to %+v", got)
	}
}

// Zone points within 0.01 of a frame edge snap to the edge, so a zone drawn "to the
// bottom" by hand or by an older editor still contains persons cut off by the frame.
func TestPersonDetectionZonesSnapToFrameEdges(t *testing.T) {
	d := config.DefaultGo2RTCStreamDetection(true)
	d.Zones = []config.Go2RTCDetectionZone{{Name: "Lawn", Points: [][2]float64{{0.005, 0.5}, {0.995, 0.5}, {0.995, 0.995}, {0.005, 0.995}}}}
	cam := personDetectionCamera("yard", "Yard", d)
	want := []persondetect.Point{{X: 0, Y: 0.5}, {X: 1, Y: 0.5}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	if !reflect.DeepEqual(cam.Zones[0].Points, want) {
		t.Fatalf("snapped points = %+v, want %+v", cam.Zones[0].Points, want)
	}
	cutOff := persondetect.Box{X1: 0.4, Y1: 0.6, X2: 0.6, Y2: 1, Score: 0.9}
	if counts, zones := persondetect.BoxCounts(cutOff, cam.MinScore, cam.Zones); !counts || !slices.Equal(zones, []string{"Lawn"}) {
		t.Fatalf("a person cut off at the frame bottom = %v %v, want a hit in Lawn", counts, zones)
	}
	if d.Zones[0].Points[2] != [2]float64{0.995, 0.995} {
		t.Fatal("the mapping rewrote the stored config")
	}
	inner := config.DefaultGo2RTCStreamDetection(true)
	inner.Zones = []config.Go2RTCDetectionZone{{Name: "Path", Points: [][2]float64{{0.02, 0.5}, {0.98, 0.5}, {0.98, 0.98}}}}
	if got := personDetectionCamera("yard", "Yard", inner).Zones[0].Points[2]; got != (persondetect.Point{X: 0.98, Y: 0.98}) {
		t.Fatalf("a point 0.02 from the edge moved to %+v", got)
	}
}

// The service defaults and limits must not drift from config's (C4/C5).
func TestPersonDetectionConstantsMatchConfig(t *testing.T) {
	if persondetect.ModelTier(config.Go2RTCDetectionDefaultModel) != persondetect.ModelStandard ||
		persondetect.Backend(config.Go2RTCDetectionDefaultBackend) != persondetect.BackendAuto ||
		persondetect.DefaultRetentionDays != config.Go2RTCDetectionDefaultRetentionDays ||
		persondetect.DefaultMaxStorageMB != config.Go2RTCDetectionDefaultMaxStorageMB ||
		persondetect.DefaultCameraInterval != time.Duration(config.Go2RTCDetectionDefaultInterval)*time.Second ||
		persondetect.DefaultCameraMinScore != config.Go2RTCDetectionDefaultMinScore {
		t.Fatal("persondetect defaults drifted from config")
	}
	base := config.NormalizeGo2RTCDetectionSettings(config.Go2RTCDetectionConfig{})
	for _, tier := range []persondetect.ModelTier{persondetect.ModelStandard, persondetect.ModelAccurate} {
		d := base
		d.Model = string(tier)
		if err := config.ValidateGo2RTCDetectionSettings(d); err != nil {
			t.Errorf("config rejects model %q: %v", tier, err)
		}
		if _, ok := persondetect.Model(tier); !ok {
			t.Errorf("no model spec for %q", tier)
		}
	}
	for _, backend := range []persondetect.Backend{persondetect.BackendAuto, persondetect.BackendGPU, persondetect.BackendCPU} {
		d := base
		d.Backend = string(backend)
		if err := config.ValidateGo2RTCDetectionSettings(d); err != nil {
			t.Errorf("config rejects backend %q: %v", backend, err)
		}
	}
	// The on-demand test decodes at TestMinScore: it must equal the lowest camera
	// threshold config accepts, or a widened range would hide counting boxes.
	cam := config.DefaultGo2RTCStreamDetection(true)
	cam.MinScore = persondetect.TestMinScore
	if err := config.ValidateGo2RTCStreamDetection(cam); err != nil {
		t.Errorf("config rejects TestMinScore: %v", err)
	}
	cam.MinScore = persondetect.TestMinScore - 0.01
	if config.ValidateGo2RTCStreamDetection(cam) == nil {
		t.Error("config accepts thresholds below TestMinScore")
	}
	channels := config.Go2RTCDetectionChannels()
	for _, channel := range []string{"telegram", "discord", "all"} {
		if !slices.Contains(channels, channel) {
			t.Errorf("config does not offer the alert channel %q", channel)
		}
	}
}

func TestGo2RTCFrameSourceRequestsFullWidthFrames(t *testing.T) {
	jpeg := cameraTestJPEG()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/go2rtc/proxy/api/frame.jpeg" || r.URL.Query().Get("width") != "" || r.URL.Query().Get("src") != "aurago_front-door" {
			t.Errorf("unexpected frame request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpeg)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "internal-password",
		Streams: []config.Go2RTCStreamConfig{{ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://192.168.1.20/live"}},
	}
	s := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	data, err := (go2RTCFrameSource{s: s}).Frame(context.Background(), "front-door")
	if err != nil || !bytes.Equal(data, jpeg) {
		t.Fatalf("Frame = %x, %v", data, err)
	}
	if _, err := (go2RTCFrameSource{s: s}).Frame(context.Background(), "unknown"); err == nil {
		t.Fatal("an unknown stream returned a frame")
	}
	if _, err := (go2RTCFrameSource{s: &Server{}}).Frame(context.Background(), "front-door"); err == nil {
		t.Fatal("a server without go2rtc returned a frame")
	}
}

func personDetectionTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Go2RTC.Enabled = true
	cfg.Go2RTC.Detection = config.Go2RTCDetectionConfig{Enabled: true}
	cfg.Go2RTC.Streams = []config.Go2RTCStreamConfig{{ID: "front-door", Name: "Front door", Enabled: true, Detection: config.DefaultGo2RTCStreamDetection(true)}}
	return cfg
}

func TestSyncPersonDetectionSettingsWaitsForStartAndFollowsConfig(t *testing.T) {
	cfg := personDetectionTestConfig(t)
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	svc := persondetect.NewService(persondetect.Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Frames: persondetecttest.NewFrames(), NewDetector: factory.New,
	})
	persondetecttest.StopOnCleanup(t, svc)
	s := &Server{Cfg: cfg, PersonDetect: svc}
	s.initConfigSnapshot()
	s.syncPersonDetectionSettings()
	if factory.Calls() != 0 || svc.Status().Enabled {
		t.Fatal("person detection was configured before Start")
	}
	s.startPersonDetection()
	persondetecttest.WaitDetector(t, svc, persondetect.DetectorReady)
	if st := svc.Status(); !st.Enabled || len(st.Cameras) != 1 || st.Cameras[0].ID != "front-door" {
		t.Fatalf("status after start = %+v", st)
	}
	next := *cfg
	next.Go2RTC.Detection.Enabled = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(&next)
	s.CfgMu.Unlock()
	if st := svc.Status(); st.Enabled || st.Detector.State != persondetect.DetectorOff {
		t.Fatalf("status after a publication that disabled detection = %+v", st)
	}
}

func TestShutdownPersonDetectionResetsTheDefault(t *testing.T) {
	svc := persondetect.NewService(persondetect.Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	s := &Server{PersonDetect: svc}
	persondetect.SetDefault(svc)
	s.shutdownPersonDetection(context.Background())
	if persondetect.Default() != nil {
		t.Fatal("shutdown left the service published")
	}
	(&Server{}).shutdownPersonDetection(context.Background())
	var nilServer *Server
	nilServer.shutdownPersonDetection(context.Background())
}
