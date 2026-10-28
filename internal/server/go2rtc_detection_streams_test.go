package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func newGo2RTCDetectionStreamServer(t *testing.T) (*Server, string) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if go2RTCFixtureCleanup(w, r) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/go2rtc/proxy/api":
			_, _ = w.Write([]byte(`{"version":"1.9.14"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/go2rtc/proxy/api/streams":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	server, configPath, _ := newGo2RTCMutationTestServer(t, upstream.URL, true)
	create := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/go2rtc/streams", strings.NewReader(`{"id":"front-door","name":"Front door","source":"rtsp://camera-user:pw@192.168.40.30/live"}`))
	create.Host = "aurago.local"
	recorder := httptest.NewRecorder()
	handleGo2RTCCreateStream(server).ServeHTTP(recorder, create)
	assertGo2RTCMutationResponse(t, recorder, http.StatusCreated, "ok", true)
	return server, configPath
}

func patchGo2RTCStream(server *Server, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPatch, "http://aurago.local/api/go2rtc/streams/front-door", strings.NewReader(body))
	request.Host = "aurago.local"
	recorder := httptest.NewRecorder()
	handleGo2RTCStreamMutation(server).ServeHTTP(recorder, request)
	return recorder
}

func TestGo2RTCStreamPatchSavesDetection(t *testing.T) {
	server, configPath := newGo2RTCDetectionStreamServer(t)
	recorder := patchGo2RTCStream(server, `{"detection":{"enabled":true,"interval_seconds":3,"min_score":0.6,"cooldown_seconds":120,"zones":[{"name":"Driveway","points":[[0.1,0.5],[0.9,0.5],[0.9,1]]}],"actions":{"message":true,"channel":"Telegram","agent":false,"agent_instruction":"","desktop":true}}}`)
	assertGo2RTCMutationResponse(t, recorder, http.StatusOK, "ok", true)
	var response struct {
		Stream struct {
			Detection config.Go2RTCStreamDetectionConfig `json:"detection"`
		} `json:"stream"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Stream.Detection.IntervalSeconds != 3 || response.Stream.Detection.Actions.Channel != "telegram" {
		t.Fatalf("response = %s", recorder.Body.String())
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := saved.Go2RTC.Streams[0]
	if got.Name != "Front door" || !got.Enabled || !got.Detection.Enabled || got.Detection.MinScore != 0.6 || got.Detection.CooldownSeconds != 120 ||
		len(got.Detection.Zones) != 1 || got.Detection.Zones[0].Name != "Driveway" || !got.Detection.Actions.Message {
		t.Fatalf("saved stream = %+v", got)
	}
}

// A PATCH merges the detection object into the camera's saved block: absent keys
// (also in actions) keep their values, a present zones list replaces the whole list,
// and the published snapshot's zones are never written through.
func TestGo2RTCStreamPatchMergesDetection(t *testing.T) {
	server, configPath := newGo2RTCDetectionStreamServer(t)
	full := `{"detection":{"enabled":true,"interval_seconds":3,"min_score":0.6,"cooldown_seconds":120,"zones":[{"name":"Driveway","points":[[0.1,0.5],[0.9,0.5],[0.9,1],[0.1,1]]}],"actions":{"message":true,"channel":"telegram","agent":false,"agent_instruction":"","desktop":false}}}`
	assertGo2RTCMutationResponse(t, patchGo2RTCStream(server, full), http.StatusOK, "ok", true)
	stored := func() config.Go2RTCStreamDetectionConfig {
		t.Helper()
		saved, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		return saved.Go2RTC.Streams[0].Detection
	}
	want := stored()

	// Switching detection off keeps the camera's zones and settings.
	assertGo2RTCMutationResponse(t, patchGo2RTCStream(server, `{"detection":{"enabled":false}}`), http.StatusOK, "ok", true)
	want.Enabled = false
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after detection off = %+v, want %+v", got, want)
	}
	// A nested key changes only itself.
	assertGo2RTCMutationResponse(t, patchGo2RTCStream(server, `{"detection":{"actions":{"agent":true}}}`), http.StatusOK, "ok", true)
	want.Actions.Agent = true
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after actions.agent = %+v, want %+v", got, want)
	}

	// zones replaces the whole list; a zone is never merged with the saved zone at
	// its index, and the snapshot that was published before is not modified.
	before := server.ConfigSnapshot().Go2RTC.Streams[0].Detection.Zones
	beforePoints := append([][2]float64(nil), before[0].Points...)
	recorder := patchGo2RTCStream(server, `{"detection":{"zones":[{"name":"Porch"}]}}`)
	var failure map[string]interface{}
	_ = json.Unmarshal(recorder.Body.Bytes(), &failure)
	if recorder.Code != http.StatusBadRequest || failure["field"] != "detection.zones[0].points" || failure["saved"] != false {
		t.Fatalf("zone without points = %d %s (must not inherit the saved zone's points)", recorder.Code, recorder.Body.String())
	}
	assertGo2RTCMutationResponse(t, patchGo2RTCStream(server, `{"detection":{"zones":[{"name":"Porch","points":[[0,0],[0.5,0],[0.5,0.5]]}]}}`), http.StatusOK, "ok", true)
	want.Zones = []config.Go2RTCDetectionZone{{Name: "Porch", Points: [][2]float64{{0, 0}, {0.5, 0}, {0.5, 0.5}}}}
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after replacing zones = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(before[0].Points, beforePoints) || before[0].Name != "Driveway" {
		t.Fatalf("the previous snapshot's zone changed to %+v", before[0])
	}

	// Switching the stream off leaves its detection block alone. Disabling a stream
	// recreates the sidecar, which needs Docker, so the runtime may be degraded.
	recorder = patchGo2RTCStream(server, `{"enabled":false}`)
	var saved struct {
		Saved bool `json:"saved"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &saved)
	if (recorder.Code != http.StatusOK && recorder.Code != http.StatusAccepted) || !saved.Saved {
		t.Fatalf("stream off = %d %s", recorder.Code, recorder.Body.String())
	}
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after the stream switch = %+v, want %+v", got, want)
	}
}

func TestGo2RTCStreamPatchRejectsInvalidDetection(t *testing.T) {
	server, configPath := newGo2RTCDetectionStreamServer(t)
	for body, field := range map[string]string{
		`{"detection":{"interval_seconds":3,"min_score":0.6,"zones":[{"name":"A","points":[[0,0],[1,0]]}],"actions":{"channel":"all"}}}`:         "detection.zones[0].points",
		`{"detection":{"interval_seconds":3,"min_score":0.6,"zones":[{"name":"A","points":[[0,0],[1,0],[1,1,0]]}],"actions":{"channel":"all"}}}`: "detection.zones[0].points[2]",
		`{"detection":{"interval_seconds":3,"min_score":2,"actions":{"channel":"all"}}}`:                                                         "detection.min_score",
		`{"detection":{"interval_seconds":"3"}}`: "detection.interval_seconds",
		`{"detection":{"Enabled":true}}`:         "detection.Enabled",
		`{"detection":{"enabled":null}}`:         "detection.enabled",
		`{"detection":null}`:                     "detection",
		`{"detection":5}`:                        "detection",
	} {
		recorder := patchGo2RTCStream(server, body)
		var response map[string]interface{}
		_ = json.Unmarshal(recorder.Body.Bytes(), &response)
		if recorder.Code != http.StatusBadRequest || response["code"] != "invalid_request" || response["field"] != field || response["saved"] != false {
			t.Fatalf("PATCH %s = %d %s, want 400 for %s", body, recorder.Code, recorder.Body.String(), field)
		}
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Go2RTC.Streams[0].Detection.Enabled {
		t.Fatal("a rejected detection object was saved")
	}
}

func TestGo2RTCAppStateShowsDetectionToManagersOnly(t *testing.T) {
	detection := config.DefaultGo2RTCStreamDetection(true)
	detection.MinScore = 0.7
	cfg := &config.Config{}
	cfg.Go2RTC = config.Go2RTCConfig{
		Enabled: true, URL: "http://127.0.0.1:1", APIHostPort: 1,
		Detection: config.Go2RTCDetectionConfig{Enabled: true},
		Streams: []config.Go2RTCStreamConfig{
			{ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://192.168.1.20/live", SourceConfigured: true, Detection: detection},
			{ID: "garage", Name: "Garage", Source: "rtsp://192.168.1.21/live", SourceConfigured: true},
		},
	}
	server := &Server{Cfg: cfg, Go2RTC: tools.NewGo2RTCManager(cfg, nil, nil, nil)}
	var state struct {
		DetectionEnabled bool `json:"detection_enabled"`
		Streams          []struct {
			ID        string                              `json:"id"`
			Detection *config.Go2RTCStreamDetectionConfig `json:"detection"`
		} `json:"streams"`
		Disabled []map[string]interface{} `json:"disabled_streams"`
	}
	admin := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(admin, httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil))
	if err := json.Unmarshal(admin.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode: %v (%s)", err, admin.Body.String())
	}
	if !state.DetectionEnabled || len(state.Streams) != 1 || state.Streams[0].Detection == nil || state.Streams[0].Detection.MinScore != 0.7 {
		t.Fatalf("manager state = %s", admin.Body.String())
	}
	if len(state.Disabled) != 1 || state.Disabled[0]["detection"] == nil {
		t.Fatalf("disabled streams = %v", state.Disabled)
	}
	viewerRequest := httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil)
	viewerRequest.Header.Set("Authorization", "Bearer viewer-scope-only")
	viewer := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(viewer, viewerRequest)
	if strings.Contains(viewer.Body.String(), `"detection":`) || !strings.Contains(viewer.Body.String(), `"detection_enabled":true`) {
		t.Fatalf("viewer state = %s", viewer.Body.String())
	}
	cfg.Go2RTC.Detection.Enabled = false
	server.Go2RTC.Configure(cfg)
	off := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(off, httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil))
	if !strings.Contains(off.Body.String(), `"detection_enabled":false`) {
		t.Fatalf("state with detection off = %s", off.Body.String())
	}
}

// The manager is reconfigured in a goroutine after a config-page save, so app/state
// takes each stream's detection block from the published config, not from the
// manager's possibly older copy.
func TestGo2RTCAppStateReadsDetectionFromThePublishedConfig(t *testing.T) {
	stale := config.DefaultGo2RTCStreamDetection(true)
	stale.MinScore = 0.7
	stale.Zones = []config.Go2RTCDetectionZone{{Name: "Old", Points: [][2]float64{{0, 0}, {1, 0}, {1, 1}}}}
	fresh := stale
	fresh.MinScore = 0.8
	fresh.Zones = []config.Go2RTCDetectionZone{{Name: "Driveway", Points: [][2]float64{{0, 0.5}, {1, 0.5}, {1, 1}}}}
	build := func(detection config.Go2RTCStreamDetectionConfig) *config.Config {
		cfg := &config.Config{}
		cfg.Go2RTC = config.Go2RTCConfig{
			Enabled: true, URL: "http://127.0.0.1:1", APIHostPort: 1,
			Detection: config.Go2RTCDetectionConfig{Enabled: true},
			Streams: []config.Go2RTCStreamConfig{
				{ID: "front-door", Name: "Front door", Enabled: true, Source: "rtsp://192.168.1.20/live", SourceConfigured: true, Detection: detection},
				{ID: "garage", Name: "Garage", Source: "rtsp://192.168.1.21/live", SourceConfigured: true, Detection: detection},
			},
		}
		return cfg
	}
	server := &Server{Cfg: build(fresh), Go2RTC: tools.NewGo2RTCManager(build(stale), nil, nil, nil)}
	var state struct {
		Streams []struct {
			Detection *config.Go2RTCStreamDetectionConfig `json:"detection"`
		} `json:"streams"`
		Disabled []struct {
			Detection config.Go2RTCStreamDetectionConfig `json:"detection"`
		} `json:"disabled_streams"`
	}
	recorder := httptest.NewRecorder()
	handleGo2RTCAppState(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/go2rtc/app/state", nil))
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode: %v (%s)", err, recorder.Body.String())
	}
	if len(state.Streams) != 1 || state.Streams[0].Detection == nil || state.Streams[0].Detection.MinScore != 0.8 ||
		len(state.Streams[0].Detection.Zones) != 1 || state.Streams[0].Detection.Zones[0].Name != "Driveway" {
		t.Fatalf("enabled stream detection = %s", recorder.Body.String())
	}
	if len(state.Disabled) != 1 || state.Disabled[0].Detection.MinScore != 0.8 || len(state.Disabled[0].Detection.Zones) != 1 || state.Disabled[0].Detection.Zones[0].Name != "Driveway" {
		t.Fatalf("disabled stream detection = %s", recorder.Body.String())
	}
}
